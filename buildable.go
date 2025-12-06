package weave

// Buildable is an interface for nodes that require a build step before execution.
// This allows Chain and other executors to detect and build sub-components
// without knowing their concrete types.
type Buildable interface {
	Build() error
}

// SubExecutor is an interface for nodes that manage their own execution context.
// These nodes typically represent a sub-graph or group of nodes that should be
// executed as a unit with their own hierarchy path.
type SubExecutor interface {
	Buildable
	// InternalName returns the name used for hierarchy path.
	// This is typically the name of the sub-group or sub-chain.
	InternalName() string
}
