// Package graphviz provides DAG execution visualization capabilities.
// It records node execution information and generates Graphviz DOT format
// visualizations with critical path analysis.
package graphviz

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/awalterschulze/gographviz"
	"github.com/mbeoliero/weave"
)

const (
	maxGraphCount           = 50
	maxGraphNodeCount       = 500
	maxOriginGraphUrlLength = 4096
	HierarchyPathJoinChar   = "/"
)

type graphvizKey struct{}

// Builder is used to construct Graphviz instances with configuration options.
type Builder struct {
	name        string
	minCostMs   int
	recordLimit int
	costDetail  bool
}

// NewBuilder creates a new graphviz builder with the given name.
func NewBuilder(name string) *Builder {
	return &Builder{
		name:        name,
		minCostMs:   0,
		recordLimit: maxGraphNodeCount,
		costDetail:  false,
	}
}

// WithLimit sets the record limit for nodes.
func (b *Builder) WithLimit(limit int) *Builder {
	b.recordLimit = limit
	return b
}

// WithMinCost sets the minimum cost threshold in milliseconds.
// Graphs with total cost below this threshold will not be generated.
func (b *Builder) WithMinCost(minCostMs int) *Builder {
	b.minCostMs = minCostMs
	return b
}

// WithTimeDetail enables detailed timing information in node labels.
func (b *Builder) WithTimeDetail() *Builder {
	b.costDetail = true
	return b
}

// Build constructs a Graphviz instance and returns a context with it embedded.
func (b *Builder) Build(ctx context.Context) (context.Context, *Graphviz) {
	g := newGraphviz(b.name, b.minCostMs, b.costDetail, true)
	return context.WithValue(ctx, graphvizKey{}, g), g
}

// EdgeType represents the type of dependency edge.
type EdgeType int

const (
	NodeToNode EdgeType = iota
	NodeToGroup
	GroupToNode
	GroupToGroup
)

// DependencyEdge represents a dependency relationship edge.
type DependencyEdge struct {
	FromNode      string
	ToNode        string
	EdgeType      EdgeType
	IsLongest     bool
	GroupCross    bool
	FromGroupPath string
	ToGroupPath   string
}

// graphNode represents a node in the graph.
type graphNode struct {
	Node         weave.DependencyNode
	GroupPath    string
	Dependencies []string
	Start        time.Time
	Finish       time.Time
	Error        error
	IsContainer  bool // true for container nodes, false for actual execution nodes
}

// newGraphNode creates a new graph node.
func newGraphNode(node weave.DependencyNode, groupPath string) *graphNode {
	return &graphNode{
		Node:         node,
		GroupPath:    groupPath,
		Dependencies: node.Dependencies(),
		IsContainer:  false,
	}
}

// record records execution information for the node.
func (n *graphNode) record(start, finish time.Time, err error) *graphNode {
	n.Start = start
	n.Finish = finish
	n.Error = err
	return n
}

// graphGroup represents a group's information.
type graphGroup struct {
	GroupPath   string
	Nodes       []*graphNode
	NodeMap     map[*graphNode]bool
	First       *graphNode
	Last        *graphNode
	MinStart    time.Time
	MaxFinish   time.Time
	LongestPath []*graphNode
	LongestCost time.Duration
}

// Graphviz is the main graphviz management structure.
type Graphviz struct {
	name       string
	minCostMs  int
	costDetail bool
	start      time.Time
	valid      atomic.Bool
	mu         sync.Mutex
	nodes      []*graphNode
	nodeMap    map[weave.DependencyNode]bool
	groups     map[string]bool
}

// newGraphviz creates a new Graphviz instance.
func newGraphviz(name string, minCostMs int, costDetail, valid bool) *Graphviz {
	g := &Graphviz{
		name:       name,
		minCostMs:  minCostMs,
		costDetail: costDetail,
		start:      time.Now(),
		valid:      atomic.Bool{},
		nodes:      []*graphNode{},
		nodeMap:    map[weave.DependencyNode]bool{},
		groups:     map[string]bool{},
	}
	g.valid.Store(valid)
	return g
}

// Record records node execution information.
// This is the public API for recording node execution.
func (g *Graphviz) Record(node weave.DependencyNode, groupPath string, start, finish time.Time, err error) {
	if !g.valid.Load() {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(g.nodes) >= maxGraphNodeCount {
		g.valid.Store(false)
		return
	}

	g.nodes = append(g.nodes, newGraphNode(node, groupPath).record(start, finish, err))
	g.nodeMap[node] = true
}

// Log outputs graph information to stdout.
func (g *Graphviz) Log(ctx context.Context) {
	info := g.GetInfo()
	if info != "" {
		fmt.Println(info)
	}
}

// GetInfo generates graphviz visualization information.
func (g *Graphviz) GetInfo() string {
	if !g.valid.Load() {
		return ""
	}

	// Filter by cost
	totalCostMs := time.Since(g.start).Milliseconds()
	if totalCostMs < int64(g.minCostMs) {
		return ""
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// Data preprocessing
	g.preprocessNodes()

	// Build dependency graph
	edges := g.buildDependencyGraph()

	// Divide into groups by path
	groups := g.divideIntoGroupsByPath()

	// Calculate critical paths
	g.calculateCriticalPaths(groups)

	// Render graphviz
	graph := g.renderGraphviz(groups, edges)

	// Generate result
	return g.generateResult(graph, groups, totalCostMs)
}

// preprocessNodes performs data preprocessing.
func (g *Graphviz) preprocessNodes() {
	g.identifyContainerNodes()
}

// identifyContainerNodes analyzes all node group paths to identify container nodes.
func (g *Graphviz) identifyContainerNodes() {
	// Collect all group path components
	for _, node := range g.nodes {
		if node.GroupPath != "" {
			parts := strings.Split(node.GroupPath, HierarchyPathJoinChar)
			for _, part := range parts {
				if part != "" {
					g.groups[part] = true
				}
			}
		}
	}

	// Mark container nodes
	for _, node := range g.nodes {
		if g.groups[node.Node.Name()] {
			node.IsContainer = true
		}
	}
}

// buildDependencyGraph builds the unified dependency graph.
func (g *Graphviz) buildDependencyGraph() []*DependencyEdge {
	var edges []*DependencyEdge

	// Create node name to node mapping
	nodeByName := make(map[string]*graphNode)
	for _, node := range g.nodes {
		nodeByName[node.Node.Name()] = node
	}

	// Map to avoid duplicate edges
	edgeMap := make(map[string]bool)

	// Process all node dependencies
	for _, node := range g.nodes {
		for _, depName := range node.Dependencies {
			edgeKey := fmt.Sprintf("%s -> %s", depName, node.Node.Name())
			if edgeMap[edgeKey] {
				continue
			}

			if depNode, exists := nodeByName[depName]; exists {
				edge := &DependencyEdge{
					FromNode:      depName,
					ToNode:        node.Node.Name(),
					EdgeType:      g.classifyEdgeType(depNode, node),
					FromGroupPath: depNode.GroupPath,
					ToGroupPath:   node.GroupPath,
				}

				edge.GroupCross = g.isGroupCrossEdge(edge)
				edges = append(edges, edge)
				edgeMap[edgeKey] = true
			}
		}
	}

	return edges
}

// classifyEdgeType classifies the type of an edge.
func (g *Graphviz) classifyEdgeType(fromNode, toNode *graphNode) EdgeType {
	switch {
	case !fromNode.IsContainer && !toNode.IsContainer:
		return NodeToNode
	case !fromNode.IsContainer && toNode.IsContainer:
		return NodeToGroup
	case fromNode.IsContainer && !toNode.IsContainer:
		return GroupToNode
	case fromNode.IsContainer && toNode.IsContainer:
		return GroupToGroup
	}
	return NodeToNode
}

// isGroupCrossEdge determines if an edge crosses groups.
func (g *Graphviz) isGroupCrossEdge(edge *DependencyEdge) bool {
	return edge.FromGroupPath != edge.ToGroupPath
}

// generateResult generates the final result string.
func (g *Graphviz) generateResult(graph *gographviz.Graph, groups []*graphGroup, totalCostMs int64) string {
	// Generate path description
	var pathParts []string
	for _, group := range groups {
		if len(group.LongestPath) > 0 {
			nodeNames := g.generateNodeNames([]*graphGroup{group})
			var nodeParts []string
			for _, node := range group.LongestPath {
				nodeParts = append(nodeParts, nodeNames[node.Node])
			}
			nodePath := strings.Join(nodeParts, " -> ")
			pathParts = append(pathParts, fmt.Sprintf("[%v(%vms)]", nodePath, group.LongestCost.Milliseconds()))
		}
	}
	path := strings.Join(pathParts, "")

	// Generate graph URL
	graphUrl := g.compressGraphUrl(fmt.Sprintf("https://dreampuf.github.io/GraphvizOnline/?presentation#%v",
		url.PathEscape(graph.String())))

	return fmt.Sprintf("total cost: %vms, longest path: %v, graph: %v", totalCostMs, path, graphUrl)
}

// compressGraphUrl compresses the graph URL if it exceeds the maximum length.
func (g *Graphviz) compressGraphUrl(originUrl string) string {
	if len(originUrl) <= maxOriginGraphUrlLength {
		return originUrl
	}

	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, _ = writer.Write([]byte(originUrl))
	_ = writer.Close()

	compressed := base64.StdEncoding.EncodeToString(buf.Bytes())
	return fmt.Sprintf("compressed: %s", compressed)
}

// FromContext retrieves the Graphviz instance from context.
func FromContext(ctx context.Context) (*Graphviz, bool) {
	g, ok := ctx.Value(graphvizKey{}).(*Graphviz)
	return g, ok
}
