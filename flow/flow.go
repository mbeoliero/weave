//
// Loader-Processor-Assembler: a load-process-assemble pipeline commonly used in API or RPC read endpoints.
// Loader: fetches data. Multiple loaders run in parallel. If dependencies exist, execute the depended-on loaders first. Typically time-consuming.
// Processor: processes data. Executes sequentially in the order added. Typically not time-consuming.
// Assembler: assembles and returns the result. Executes sequentially in the order added. Typically not time-consuming.
//

package flow

import (
	"context"

	"github.com/mbeoliero/weave"
)

type Loader[C any] interface {
	Name() string
	Depends() []IDepend // depends on other
	Load(ctx context.Context, c C) error
}

type Processor[C any] interface {
	Name() string
	Process(ctx context.Context, c C) error
}

type Assembler[P, R any] interface {
	Name() string
	Assemble(ctx context.Context, payload P, result R) error
}

type ExecCtx[P, R any] struct {
	Payload P
	Result  R
}

type Flow[P, R any] struct {
	flow      *weave.Graph[ExecCtx[P, R]]
	loader    *weave.Group[ExecCtx[P, R]]
	processor *weave.Group[ExecCtx[P, R]]
	assembler *weave.Group[ExecCtx[P, R]]
}

func NewFlow[P, R any]() *Flow[P, R] {
	f := &Flow[P, R]{
		flow:      weave.NewGraph[ExecCtx[P, R]](),
		loader:    weave.NewGroup[ExecCtx[P, R]]("loader"),
		processor: weave.NewGroup[ExecCtx[P, R]]("processor", "loader").SetMaxGoNum(1),
		assembler: weave.NewGroup[ExecCtx[P, R]]("assembler", "processor").SetMaxGoNum(1),
	}
	f.flow.AddNode(f.loader.AsNode())
	f.flow.AddNode(f.processor.AsNode())
	f.flow.AddNode(f.assembler.AsNode())
	return f
}

func (e *Flow[P, R]) SetMaxGoNum(goNum int) *Flow[P, R] {
	e.flow.SetMaxGoNum(goNum)
	return e
}

func (e *Flow[P, R]) AddGlobalMW(mws ...weave.Middleware) *Flow[P, R] {
	e.flow.AddGlobalMW(mws...)
	return e
}

func (e *Flow[P, R]) AddLoader(loader Loader[P], opts ...weave.Option) {
	node := weave.NewNode(loader.Name(), func(ctx context.Context, execCtx ExecCtx[P, R]) error {
		return loader.Load(ctx, execCtx.Payload)
	}, weave.Map(loader.Depends(), func(dep IDepend) string {
		return dep.Name()
	})...)

	e.loader.AddNode(node, opts...)
}

func (e *Flow[P, R]) AddProcessor(processor Processor[P], opts ...weave.Option) {
	node := weave.NewNode(processor.Name(), func(ctx context.Context, execCtx ExecCtx[P, R]) error {
		return processor.Process(ctx, execCtx.Payload)
	})

	e.processor.AddNode(node, opts...)
}

func (e *Flow[P, R]) AddAssembler(assembler Assembler[P, R], opts ...weave.Option) {
	node := weave.NewNode(assembler.Name(), func(ctx context.Context, execCtx ExecCtx[P, R]) error {
		return assembler.Assemble(ctx, execCtx.Payload, execCtx.Result)
	})

	e.assembler.AddNode(node, opts...)
}

func (e *Flow[P, R]) Exec(ctx context.Context, payload P, result R) error {
	return e.flow.Exec(ctx, ExecCtx[P, R]{
		Payload: payload,
		Result:  result,
	})
}
