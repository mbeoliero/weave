package graphviz

import (
	"fmt"
	"strings"
	"time"

	"github.com/awalterschulze/gographviz"
	"github.com/mbeoliero/weave"
)

// renderGraphviz renders the graphviz graph.
func (g *Graphviz) renderGraphviz(groups []*graphGroup, edges []*DependencyEdge) *gographviz.Graph {
	graph := gographviz.NewGraph()
	totalCostMs := time.Since(g.start).Milliseconds()
	graphAst, _ := gographviz.Parse([]byte(fmt.Sprintf(`digraph G{rankdir=LR; label="%v %vms";}`, g.name, totalCostMs)))
	_ = gographviz.Analyse(graphAst, graph)

	// Generate node name mapping
	nodeNames := g.generateNodeNames(groups)

	// Store mapping from group path to graphviz cluster name
	groupPathToGraphName := make(map[string]string)

	// Render subgraphs
	g.renderSubGraphs(graph, groups, groupPathToGraphName, nodeNames)

	// Render dependency edges
	g.renderDependencyEdges(graph, edges, nodeNames, groups)

	return graph
}

// generateNodeNames generates node name mapping, handling duplicate names.
func (g *Graphviz) generateNodeNames(groups []*graphGroup) map[weave.DependencyNode]string {
	nodeNames := map[weave.DependencyNode]string{}
	nameCount := map[string]int{}

	for _, group := range groups {
		for _, node := range group.Nodes {
			var name string
			if n, ok := nodeNames[node.Node]; ok {
				name = n
			} else if count := nameCount[node.Node.Name()]; count > 0 {
				name = fmt.Sprintf("%v_%v", node.Node.Name(), count)
				nodeNames[node.Node] = name
				nameCount[node.Node.Name()]++
			} else {
				name = node.Node.Name()
				nodeNames[node.Node] = name
				nameCount[node.Node.Name()]++
			}
		}
	}

	return nodeNames
}

// renderSubGraphs renders subgraphs.
func (g *Graphviz) renderSubGraphs(graph *gographviz.Graph, groups []*graphGroup, groupPathToGraphName map[string]string, nodeNames map[weave.DependencyNode]string) {
	for idx, group := range groups {
		graphName := fmt.Sprintf("cluster_%v", idx)
		groupPathToGraphName[group.GroupPath] = graphName

		// Determine parent graph and label
		parentGraphName := "G"
		groupLabel := group.GroupPath
		if lastSepIndex := strings.LastIndex(group.GroupPath, HierarchyPathJoinChar); lastSepIndex != -1 {
			parentGroupPath := group.GroupPath[:lastSepIndex]
			if name, ok := groupPathToGraphName[parentGroupPath]; ok {
				parentGraphName = name
			}
			groupLabel = group.GroupPath[lastSepIndex+1:]
		}

		if groupLabel != "" {
			_ = graph.AddSubGraph(parentGraphName, graphName, map[string]string{
				"label": fmt.Sprintf(`"%s\n%vms"`, groupLabel, group.MaxFinish.Sub(group.MinStart).Milliseconds()),
				"style": "solid",
			})
		}

		// Create longest path mapping
		longestMap := map[weave.DependencyNode]bool{}
		for _, node := range group.LongestPath {
			longestMap[node.Node] = true
		}

		// Render nodes
		g.renderNodes(graph, graphName, group.Nodes, nodeNames, longestMap)
	}
}

// renderNodes renders nodes.
func (g *Graphviz) renderNodes(graph *gographviz.Graph, graphName string, nodes []*graphNode, nodeNames map[weave.DependencyNode]string, longestMap map[weave.DependencyNode]bool) {
	for _, node := range nodes {
		name := nodeNames[node.Node]
		var attr map[string]string

		if node.Error != nil {
			attr = map[string]string{
				"label":     fmt.Sprintf(`"%v\nError"`, name),
				"style":     "filled",
				"fillcolor": "red",
			}
		} else {
			labelBuilder := func(withLongest bool) string {
				str := fmt.Sprintf(`"%v\n`, name)
				if withLongest {
					str += "Longest"
				}
				str += fmt.Sprintf(`%vms`, node.Finish.Sub(node.Start).Milliseconds())
				if g.costDetail {
					str += fmt.Sprintf("\n[%v,%v]",
						node.Start.Sub(g.start).Milliseconds(),
						node.Finish.Sub(g.start).Milliseconds())
				}
				str += `"`
				return str
			}

			attr = map[string]string{
				"label": labelBuilder(false),
				"color": "green",
			}

			if longestMap[node.Node] {
				attr["color"] = "red"
				attr["style"] = "filled"
				attr["fillcolor"] = "lightcoral"
				if len(longestMap) == 1 {
					attr["label"] = labelBuilder(true)
				}
			}
		}

		_ = graph.AddNode(graphName, name, attr)
	}
}

// renderDependencyEdges renders dependency edges.
func (g *Graphviz) renderDependencyEdges(graph *gographviz.Graph, edges []*DependencyEdge, nodeNames map[weave.DependencyNode]string, groups []*graphGroup) {
	// Create node name to node mapping
	nodeByName := make(map[string]*graphNode)
	for _, node := range g.nodes {
		nodeByName[node.Node.Name()] = node
	}

	// Create group name to group mapping, supporting both full path and group name
	groupNameToGroup := make(map[string]*graphGroup)
	for _, group := range groups {
		// Use full GroupPath as key
		groupNameToGroup[group.GroupPath] = group

		// Also use group name (last part of path) as key for matching node dependencies
		if lastSepIndex := strings.LastIndex(group.GroupPath, HierarchyPathJoinChar); lastSepIndex != -1 {
			groupName := group.GroupPath[lastSepIndex+1:]
			groupNameToGroup[groupName] = group
		}
	}

	// Mark longest path edges
	g.markLongestEdges(edges, groups)

	// Track rendered edges to avoid duplicates
	renderedEdges := make(map[string]bool)

	for _, edge := range edges {
		edgeKey := fmt.Sprintf("%s -> %s", edge.FromNode, edge.ToNode)
		if !renderedEdges[edgeKey] {
			g.renderSingleEdge(graph, edge, nodeNames, nodeByName, groupNameToGroup)
			renderedEdges[edgeKey] = true
		}
	}
}

// markLongestEdges marks edges on the longest path.
func (g *Graphviz) markLongestEdges(edges []*DependencyEdge, groups []*graphGroup) {
	for _, group := range groups {
		if len(group.LongestPath) <= 1 {
			continue
		}

		// Mark edges on the longest path within the group
		for i := 0; i < len(group.LongestPath)-1; i++ {
			fromNode := group.LongestPath[i].Node.Name()
			toNode := group.LongestPath[i+1].Node.Name()

			for _, edge := range edges {
				if edge.FromNode == fromNode && edge.ToNode == toNode {
					edge.IsLongest = true
					break
				}
			}
		}
	}
}

// renderSingleEdge renders a single edge.
func (g *Graphviz) renderSingleEdge(graph *gographviz.Graph, edge *DependencyEdge, nodeNames map[weave.DependencyNode]string, nodeByName map[string]*graphNode, groupNameToGroup map[string]*graphGroup) {
	attr := map[string]string{}

	fromNode := nodeByName[edge.FromNode]
	toNode := nodeByName[edge.ToNode]

	// Set style based on edge type
	switch edge.EdgeType {
	case NodeToNode:
		// Normal node-to-node dependency - only render when both nodes are not container nodes and exist in nodeNames
		if fromNode != nil && toNode != nil && !fromNode.IsContainer && !toNode.IsContainer {
			if fromNodeName, ok := nodeNames[fromNode.Node]; ok {
				if toNodeName, ok := nodeNames[toNode.Node]; ok {
					if edge.IsLongest {
						attr["color"] = "red"
						attr["style"] = "bold"
					}
					_ = graph.AddEdge(fromNodeName, toNodeName, true, attr)
				}
			}
		}

	case NodeToGroup:
		// Node to group: connect to the first node of the group
		if depGroup, exists := groupNameToGroup[edge.ToNode]; exists && depGroup.First != nil {
			attr["style"] = "dashed"
			attr["color"] = "blue"
			attr["label"] = fmt.Sprintf(`"%s"`, edge.ToNode)
			// Only render edge when source node is not a container and exists in nodeNames
			if fromNode != nil && !fromNode.IsContainer {
				if fromNodeName, ok := nodeNames[fromNode.Node]; ok {
					if firstNodeName, ok := nodeNames[depGroup.First.Node]; ok {
						_ = graph.AddEdge(fromNodeName, firstNodeName, true, attr)
					}
				}
			}
		}

	case GroupToNode:
		// Group to node: connect from the last node of the group
		if depGroup, exists := groupNameToGroup[edge.FromNode]; exists && depGroup.Last != nil {
			attr["style"] = "dashed"
			attr["color"] = "purple"
			attr["label"] = fmt.Sprintf(`"%s -> %s"`, edge.FromNode, edge.ToNode)
			// Only render edge when target node is not a container and exists in nodeNames
			if toNode != nil && !toNode.IsContainer {
				if toNodeName, ok := nodeNames[toNode.Node]; ok {
					if lastNodeName, ok := nodeNames[depGroup.Last.Node]; ok {
						_ = graph.AddEdge(lastNodeName, toNodeName, true, attr)
					}
				}
			}
		}

	case GroupToGroup:
		// Group to group: from last node of source group to first node of target group
		fromGroup, fromExists := groupNameToGroup[edge.FromNode]
		toGroup, toExists := groupNameToGroup[edge.ToNode]

		if fromExists && toExists && fromGroup.Last != nil && toGroup.First != nil {
			attr["style"] = "dashed"
			attr["color"] = "orange"
			attr["label"] = fmt.Sprintf(`"%s -> %s"`, edge.FromNode, edge.ToNode)
			// Ensure both boundary nodes exist in nodeNames
			if lastNodeName, ok := nodeNames[fromGroup.Last.Node]; ok {
				if firstNodeName, ok := nodeNames[toGroup.First.Node]; ok {
					_ = graph.AddEdge(lastNodeName, firstNodeName, true, attr)
				}
			}
		}
	}
}
