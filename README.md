# Weave

Weave is a powerful, type-safe, and flexible DAG (Directed Acyclic Graph) execution engine for Go. It allows you to define complex execution workflows with dependencies, groups, and chains, while providing robust concurrency control and middleware support.

## Features

- **Type-Safe Context**: strictly typed execution context to pass data between nodes safely.
- **DAG Execution**: Automatic dependency resolution and parallel execution where possible.
- **Hierarchical Structure**: Organize nodes into **Groups** for better modularity and management.
- **Sequential Chains**: Easily define linear workflows with **Chains**.
- **Concurrency Control**: Fine-grained control over parallelism with global and local semaphores (`SetMaxGoNum`).
- **Middleware Support**: Intercept node execution for logging, metrics, tracing, or error handling.
- **Cycle Detection**: Automatically detects circular dependencies during the build phase.

## Installation

```bash
go get github.com/mbeoliero/weave
```

## Quick Start

Here is a simple example showing how to create a graph with dependencies and execute it.

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/mbeoliero/weave"
)

// Define your execution context
type OrderContext struct {
	OrderID   string
	Amount    float64
	Paid      bool
	Shipped   bool
	Notified  bool
}

func main() {
	// Create a new graph with our typed context
	graph := weave.NewGraph[*OrderContext]()

	// 1. Validate Order
	validate := weave.NewNode("validate", func(ctx context.Context, c *OrderContext) error {
		fmt.Printf("Validating order %s...\n", c.OrderID)
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	// 2. Process Payment (depends on validate)
	payment := weave.NewNode("payment", func(ctx context.Context, c *OrderContext) error {
		fmt.Println("Processing payment...")
		c.Paid = true
		return nil
	}, "validate")

	// 3. Ship Items (depends on payment)
	ship := weave.NewNode("ship", func(ctx context.Context, c *OrderContext) error {
		fmt.Println("Shipping items...")
		c.Shipped = true
		return nil
	}, "payment")

	// 4. Send Notification (depends on payment, can run in parallel with ship)
	notify := weave.NewNode("notify", func(ctx context.Context, c *OrderContext) error {
		fmt.Println("Sending notification...")
		c.Notified = true
		return nil
	}, "payment")

	// Add nodes to the graph
	graph.AddNode(validate)
	graph.AddNode(payment)
	graph.AddNode(ship)
	graph.AddNode(notify)

	// Execute
	ctx := context.Background()
	order := &OrderContext{OrderID: "ORD-123", Amount: 99.9}

	if err := graph.Exec(ctx, order); err != nil {
		panic(err)
	}

	fmt.Printf("Order finished: %+v\n", order)
}
```

### Graph
The `Graph` is the top-level container that manages the execution of nodes. It handles cycle detection, concurrency control settings, and providing the execution context.

```go
graph := weave.NewGraph[*MyContext]()
graph.AddNode(node1)
graph.AddNode(node2)

// Set global concurrency limit
graph.SetMaxGoNum(10)
```

### Node
The fundamental unit of execution. A node has a unique name, an execution function, and a list of dependencies.

```go
node := weave.NewNode("my-node", func(ctx context.Context, c *MyContext) error {
    // do work
    return nil
}, "dependency-1", "dependency-2")
```

### Group
Groups allow you to encapsulate a set of nodes. A `Group` can be treated as a single `Node` within a larger graph, enabling powerful hierarchical compositions.

```go
authGroup := weave.NewGroup[*MyContext]("auth-module")
authGroup.AddNode(loginNode)
authGroup.AddNode(sessionNode)

// Add the entire group to the main graph
graph.AddNode(authGroup.AsNode())
```

### Chain
A `Chain` is a specialized container for strictly sequential execution. It's syntactic sugar when you have a linear list of steps.

```go
chain := weave.NewChain[*MyContext]()
chain.AddNode(step1).AddNode(step2).AddNode(step3)

graph.AddNode(weave.NewNode("process-chain", func(ctx context.Context, c *MyContext) error {
    return chain.Exec(ctx, c)
}))
```

## Predefined Components

### Flow
`Flow` is a predefined architectural pattern widely used in API or RPC read endpoints. It implements a **Loader -> Processor -> Assembler** pipeline:

1.  **Loader**: Fetches data (e.g., RPC calls, DB queries). Loaders run in parallel and can have dependencies on other loaders.
2.  **Processor**: Processes the fetched data. Processors run sequentially.
3.  **Assembler**: Assembles the final response. Assemblers run sequentially.

```go
package main

import (
	"context"
	"fmt"
	"github.com/mbeoliero/weave/flow"
)

type Request struct {
	UserID int
}

type Response struct {
	User    string
	Orders  []string
}

// 1. Define a Loader
type UserLoader struct{}
func (l *UserLoader) Name() string { return "user_loader" }
func (l *UserLoader) Depends() []flow.IDepend { return nil }
func (l *UserLoader) Load(ctx context.Context, req *Request) error {
    fmt.Printf("Loading user %d\n", req.UserID)
    return nil
}

// 2. Define a Processor
type UserProcessor struct{}
func (p *UserProcessor) Name() string { return "user_processor" }
func (p *UserProcessor) Process(ctx context.Context, req *Request) error {
    fmt.Println("Processing user functionality...")
    return nil
}

// 3. Define an Assembler
type UserAssembler struct{}
func (a *UserAssembler) Name() string { return "user_assembler" }
func (a *UserAssembler) Assemble(ctx context.Context, req *Request, resp *Response) error {
    resp.User = fmt.Sprintf("User_%d", req.UserID)
    return nil
}

func main() {
    f := flow.NewFlow[*Request, *Response]()
    
    f.AddLoader(&UserLoader{})
    f.AddProcessor(&UserProcessor{})
    f.AddAssembler(&UserAssembler{})
    
    req := &Request{UserID: 123}
    resp := &Response{}
    
    if err := f.Exec(context.Background(), req, resp); err != nil {
        panic(err)
    }
    
    fmt.Printf("Result: %+v\n", resp)
}
```

## Advanced Features

### Middleware
You can add middleware to specific nodes, groups, or the entire graph to wrap execution logic.

```go
// Add logging to all nodes in the graph
graph.AddGlobalMW(weave.LoggerMW())

// Custom middleware
myMiddleware := func(node weave.DependencyNode, next weave.Endpoint) weave.Endpoint {
    return func(ctx context.Context, req any) (any, error) {
        // before execution
        res, err := next(ctx, req)
        // after execution
        return res, err
    }
}
```

### Concurrency Control
Weave allows you to limit the maximum number of concurrent goroutines per Graph, Group, or Chain.

```go
// Limit global concurrency to 10
graph.SetMaxGoNum(10)

// You can also limit concurrency for a specific group
group.SetMaxGoNum(2)
```
