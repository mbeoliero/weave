package graphviz

import (
	"time"
)

// calculateCriticalPaths calculates critical paths for all groups and marks longest edges.
func (g *Graphviz) calculateCriticalPaths(groups []*graphGroup) {
	for _, group := range groups {
		group.LongestPath, group.LongestCost = g.calculateLongestPath(group)
	}
}

// calculateLongestPath calculates the longest path within a group.
func (g *Graphviz) calculateLongestPath(group *graphGroup) ([]*graphNode, time.Duration) {
	if len(group.Nodes) == 0 {
		return nil, 0
	}

	// Check if it's serial execution
	isSerialExecution := g.detectSerialExecution(group)

	if isSerialExecution {
		return g.calculateSerialExecutionPath(group)
	}

	// Parallel execution mode: calculate critical path based on dependencies
	return g.calculateParallelExecutionPath(group)
}

// detectSerialExecution detects if the group is executed serially.
func (g *Graphviz) detectSerialExecution(group *graphGroup) bool {
	if len(group.Nodes) <= 1 {
		return false
	}

	// Sort nodes by start time
	sortedNodes := make([]*graphNode, len(group.Nodes))
	copy(sortedNodes, group.Nodes)
	g.sortNodesByStartTime(sortedNodes)

	// Check overlap between adjacent nodes
	overlapCount := 0
	totalPairs := len(sortedNodes) - 1

	for i := 0; i < len(sortedNodes)-1; i++ {
		current := sortedNodes[i]
		next := sortedNodes[i+1]

		if next.Start.Before(current.Finish) {
			overlapCount++
		}
	}

	// If overlap ratio is less than 30%, consider it serial execution
	overlapRatio := float64(overlapCount) / float64(totalPairs)
	return overlapRatio < 0.3
}

// calculateSerialExecutionPath calculates the path for serial execution.
func (g *Graphviz) calculateSerialExecutionPath(group *graphGroup) ([]*graphNode, time.Duration) {
	if len(group.Nodes) == 0 {
		return nil, 0
	}

	// Sort by time order
	sortedNodes := make([]*graphNode, len(group.Nodes))
	copy(sortedNodes, group.Nodes)
	g.sortNodesByStartTime(sortedNodes)

	// For serial execution, the longest path is the sum of all execution times
	var totalDuration time.Duration
	for _, node := range sortedNodes {
		totalDuration += node.Finish.Sub(node.Start)
	}

	return sortedNodes, totalDuration
}

// calculateParallelExecutionPath calculates the path for parallel execution.
func (g *Graphviz) calculateParallelExecutionPath(group *graphGroup) ([]*graphNode, time.Duration) {
	if len(group.Nodes) == 0 {
		return nil, 0
	}

	// Build node to index mapping
	nodeToIndex := make(map[*graphNode]int)
	for i, node := range group.Nodes {
		nodeToIndex[node] = i
	}

	// Build adjacency list based on dependencies
	adj := make([][]int, len(group.Nodes))
	inDegree := make([]int, len(group.Nodes))

	for i, node := range group.Nodes {
		for _, depName := range node.Dependencies {
			// Find dependency node within the same group
			for j, depNode := range group.Nodes {
				if depNode.Node.Name() == depName {
					adj[j] = append(adj[j], i)
					inDegree[i]++
					break
				}
			}
		}
	}

	// Calculate longest path based on actual execution time
	type pathInfo struct {
		path          []int
		totalDuration time.Duration
	}

	dp := make([]pathInfo, len(group.Nodes))
	for i := range dp {
		dp[i] = pathInfo{
			path:          []int{i},
			totalDuration: group.Nodes[i].Finish.Sub(group.Nodes[i].Start),
		}
	}

	// Topological sort + dynamic programming
	queue := []int{}
	currentInDegree := make([]int, len(inDegree))
	copy(currentInDegree, inDegree)

	for i, degree := range currentInDegree {
		if degree == 0 {
			queue = append(queue, i)
		}
	}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		for _, next := range adj[curr] {
			newDuration := dp[curr].totalDuration + group.Nodes[next].Finish.Sub(group.Nodes[next].Start)
			if newDuration > dp[next].totalDuration {
				dp[next].totalDuration = newDuration
				dp[next].path = append(append([]int{}, dp[curr].path...), next)
			}

			currentInDegree[next]--
			if currentInDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	// Find the longest path
	maxDuration := time.Duration(0)
	maxPath := []int{}
	for _, info := range dp {
		if info.totalDuration > maxDuration {
			maxDuration = info.totalDuration
			maxPath = info.path
		}
	}

	// Convert to node path
	var longestPath []*graphNode
	for _, idx := range maxPath {
		longestPath = append(longestPath, group.Nodes[idx])
	}

	return longestPath, maxDuration
}

// divideIntoGroupsByPath divides nodes into groups by group path.
func (g *Graphviz) divideIntoGroupsByPath() []*graphGroup {
	// Group nodes by group path, only include non-container nodes for visualization
	groupMap := make(map[string]*graphGroup)

	for _, node := range g.nodes {
		// Skip container nodes, only process actual execution nodes
		if node.IsContainer {
			continue
		}

		groupPath := node.GroupPath
		if _, exists := groupMap[groupPath]; !exists {
			groupMap[groupPath] = &graphGroup{
				GroupPath: groupPath,
				Nodes:     []*graphNode{},
				NodeMap:   map[*graphNode]bool{},
			}
		}

		group := groupMap[groupPath]
		group.Nodes = append(group.Nodes, node)
		group.NodeMap[node] = true
	}

	// Convert to slice and sort
	var groups []*graphGroup
	for _, group := range groupMap {
		if len(group.Nodes) > 0 {
			groups = append(groups, group)
		}
	}

	// Sort by group path, ensuring parent groups come before child groups
	g.sortGroupsByPath(groups)

	// Calculate statistics for each group
	g.calculateGroupStatistics(groups)

	return groups
}

// sortGroupsByPath sorts groups by path depth.
func (g *Graphviz) sortGroupsByPath(groups []*graphGroup) {
	for i := 0; i < len(groups)-1; i++ {
		for j := i + 1; j < len(groups); j++ {
			depthI := countPathSeparators(groups[i].GroupPath)
			depthJ := countPathSeparators(groups[j].GroupPath)

			if depthI > depthJ {
				groups[i], groups[j] = groups[j], groups[i]
			} else if depthI == depthJ {
				if groups[i].GroupPath > groups[j].GroupPath {
					groups[i], groups[j] = groups[j], groups[i]
				}
			}
		}
	}
}

// countPathSeparators counts the number of path separators in a path.
func countPathSeparators(path string) int {
	count := 0
	for _, c := range path {
		if string(c) == HierarchyPathJoinChar {
			count++
		}
	}
	return count
}

// calculateGroupStatistics calculates statistics for groups.
func (g *Graphviz) calculateGroupStatistics(groups []*graphGroup) {
	for _, group := range groups {
		if len(group.Nodes) == 0 {
			continue
		}

		// Sort nodes by start time
		g.sortNodesByStartTime(group.Nodes)

		group.First = group.Nodes[0]
		group.MinStart = group.First.Start
		group.MaxFinish = group.Nodes[0].Finish

		// Calculate MinStart, MaxFinish and Last node
		lastByTime := group.Nodes[0]
		for _, node := range group.Nodes {
			if node.Start.Before(group.MinStart) {
				group.MinStart = node.Start
			}
			if node.Finish.After(group.MaxFinish) {
				group.MaxFinish = node.Finish
			}
			if node.Finish.After(lastByTime.Finish) {
				lastByTime = node
			}
		}

		// Calculate longest path
		group.LongestPath, group.LongestCost = g.calculateLongestPath(group)

		// Determine Last node
		if len(group.LongestPath) > 0 {
			lastInLongestPath := group.LongestPath[len(group.LongestPath)-1]
			if lastInLongestPath.Finish.Sub(lastByTime.Finish) > -2*time.Millisecond {
				group.Last = lastInLongestPath
			} else {
				group.Last = lastByTime
			}
		} else {
			group.Last = lastByTime
		}
	}
}

// sortNodesByStartTime sorts nodes by start time.
func (g *Graphviz) sortNodesByStartTime(nodes []*graphNode) {
	for i := 0; i < len(nodes)-1; i++ {
		for j := i + 1; j < len(nodes); j++ {
			if nodes[i].Start.After(nodes[j].Start) {
				nodes[i], nodes[j] = nodes[j], nodes[i]
			}
		}
	}
}
