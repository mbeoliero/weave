package graphviz

import (
	"context"
	"time"

	"github.com/mbeoliero/weave"
)

// Middleware returns a graphviz middleware that records node execution information.
// This middleware should be added to the global middleware chain to enable graphviz visualization.
func Middleware() weave.Middleware {
	return func(node weave.DependencyNode, next weave.Endpoint) weave.Endpoint {
		return func(ctx context.Context, req any) (any, error) {
			// Get graphviz instance from context
			gv, ok := FromContext(ctx)
			if !ok || gv == nil {
				// If no graphviz instance, execute next middleware directly
				return next(ctx, req)
			}

			// Get current group path
			groupPath := GetCurrentGroupPath(ctx)

			start := time.Now()
			resp, err := next(ctx, req)
			finish := time.Now()

			// Record node execution information with group path
			gv.Record(node, groupPath, start, finish, err)

			return resp, err
		}
	}
}

// hierarchyPathKey is the context key for hierarchy path.
type hierarchyPathKey struct{}

// HierarchyPath represents the hierarchical path of execution.
type HierarchyPath struct {
	path []string
}

// NewHierarchyPath creates a new hierarchy path.
func NewHierarchyPath(root string) *HierarchyPath {
	return &HierarchyPath{
		path: []string{root},
	}
}

// Push adds a new level to the hierarchy path.
func (h *HierarchyPath) Push(name string) *HierarchyPath {
	newPath := make([]string, len(h.path)+1)
	copy(newPath, h.path)
	newPath[len(h.path)] = name
	return &HierarchyPath{path: newPath}
}

// String returns the full path as a string.
func (h *HierarchyPath) String() string {
	result := ""
	for i, p := range h.path {
		if i > 0 {
			result += HierarchyPathJoinChar
		}
		result += p
	}
	return result
}

// GetPath returns the path components.
func (h *HierarchyPath) GetPath() []string {
	result := make([]string, len(h.path))
	copy(result, h.path)
	return result
}

// WithHierarchyPath stores the hierarchy path in context.
func WithHierarchyPath(ctx context.Context, path *HierarchyPath) context.Context {
	return context.WithValue(ctx, hierarchyPathKey{}, path)
}

// GetHierarchyPath retrieves the hierarchy path from context.
func GetHierarchyPath(ctx context.Context) (*HierarchyPath, bool) {
	path, ok := ctx.Value(hierarchyPathKey{}).(*HierarchyPath)
	return path, ok
}

// GetCurrentGroupPath returns the current group path from context.
func GetCurrentGroupPath(ctx context.Context) string {
	if path, ok := GetHierarchyPath(ctx); ok {
		return path.String()
	}
	return ""
}

// WithPushedHierarchyPath pushes a new level to the hierarchy path in context.
func WithPushedHierarchyPath(ctx context.Context, name string) context.Context {
	var hierarchyPath *HierarchyPath
	if existingPath, ok := GetHierarchyPath(ctx); ok {
		hierarchyPath = existingPath.Push(name)
	} else {
		hierarchyPath = NewHierarchyPath(name)
	}
	return WithHierarchyPath(ctx, hierarchyPath)
}
