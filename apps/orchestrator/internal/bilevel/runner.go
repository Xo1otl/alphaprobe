package bilevel

import (
	"context"
	"sync"

	"alphaprobe/orchestrator/internal/pipeline"
)

// --- Public API ---

type RunFunc[PReq any] func(ctx context.Context, initialTasks []PReq)
type UpdateFunc[Q, E, D, PReq any] func(ctx context.Context, query Q, evidence E, data D) (newTasks []PReq, done bool)
type ProposeFunc[PReq any, POut any, D any] func(ctx context.Context, preq PReq) (pout POut, data D)
type ObserveFunc[Q any, E any] func(ctx context.Context, query Q) (evidence E)
type AdapterFunc[POut any, Q any, D any] func(in <-chan proposeRes[POut, D], out chan<- *observeReq[Q, D])
type FanOutFunc[POut any, Q any, D any] func(pout POut, data D) []Q

// --- Internal Data Structures ---

type proposeRes[POut any, D any] struct {
	POut POut
	Data D
}

type observeReq[Q any, D any] struct {
	Query Q
	Data  D
}

type observeRes[Q, E, D any] struct {
	Query    Q
	Evidence E
	Data     D
}

// --- Factories ---

func NewFanOutAdapter[POut any, Q any, D any](
	fanOut FanOutFunc[POut, Q, D],
) AdapterFunc[POut, Q, D] {
	return func(in <-chan proposeRes[POut, D], out chan<- *observeReq[Q, D]) {
		defer close(out)
		for pRes := range in {
			queries := fanOut(pRes.POut, pRes.Data)
			for _, q := range queries {
				out <- &observeReq[Q, D]{
					Query: q,
					Data:  pRes.Data,
				}
			}
		}
	}
}

func Run[PReq, Q, D, E any](
	updateFn UpdateFunc[Q, E, D, PReq],
	proposeFn ProposeFunc[PReq, Q, D],
	observeFn ObserveFunc[Q, E],
	proposeConcurrency int,
	observeConcurrency int,
	maxQueueSize int,
) RunFunc[PReq] {
	r := &simpleRunner[PReq, Q, D, E]{
		updateFn:           updateFn,
		proposeFn:          proposeFn,
		observeFn:          observeFn,
		proposeConcurrency: proposeConcurrency,
		observeConcurrency: observeConcurrency,
		maxQueueSize:       maxQueueSize,
	}
	return r.Run
}

func RunWithAdapter[PReq, POut, Q, D, E any](
	updateFn UpdateFunc[Q, E, D, PReq],
	proposeFn ProposeFunc[PReq, POut, D],
	adapterFn AdapterFunc[POut, Q, D],
	observeFn ObserveFunc[Q, E],
	proposeConcurrency int,
	observeConcurrency int,
	maxQueueSize int,
) RunFunc[PReq] {
	r := &adaptedRunner[PReq, POut, Q, D, E]{
		updateFn:           updateFn,
		proposeFn:          proposeFn,
		adapterFn:          adapterFn,
		observeFn:          observeFn,
		proposeConcurrency: proposeConcurrency,
		observeConcurrency: observeConcurrency,
		maxQueueSize:       maxQueueSize,
	}
	return r.Run
}

// --- Private Runner Implementations ---

type simpleRunner[PReq, Q, D, E any] struct {
	updateFn           UpdateFunc[Q, E, D, PReq]
	proposeFn          ProposeFunc[PReq, Q, D]
	observeFn          ObserveFunc[Q, E]
	proposeConcurrency int
	observeConcurrency int
	maxQueueSize       int
}

func (r *simpleRunner[PReq, Q, D, E]) Run(ctx context.Context, initialTasks []PReq) {
	proposeReqCh := make(chan PReq, r.proposeConcurrency)
	observeReqCh := make(chan *observeReq[Q, D], r.observeConcurrency)
	observeResCh := make(chan *observeRes[Q, E, D], r.observeConcurrency)

	var proposeWg, observeWg sync.WaitGroup

	proposeTask := func(ctx context.Context, req PReq) *observeReq[Q, D] {
		q, data := r.proposeFn(ctx, req)
		return &observeReq[Q, D]{Query: q, Data: data}
	}

	observeTask := func(ctx context.Context, obsIn *observeReq[Q, D]) *observeRes[Q, E, D] {
		evidence := r.observeFn(ctx, obsIn.Query)
		return &observeRes[Q, E, D]{Query: obsIn.Query, Evidence: evidence, Data: obsIn.Data}
	}

	pipeline.LaunchWorkers(ctx, &proposeWg, r.proposeConcurrency, proposeTask, proposeReqCh, observeReqCh)
	pipeline.LaunchWorkers(ctx, &observeWg, r.observeConcurrency, observeTask, observeReqCh, observeResCh)
	go func() { proposeWg.Wait(); close(observeReqCh) }()

	update := func(ctx context.Context, res *observeRes[Q, E, D]) ([]PReq, bool) {
		return r.updateFn(ctx, res.Query, res.Evidence, res.Data)
	}
	pipeline.Loop(ctx, update, initialTasks, proposeReqCh, observeResCh, r.maxQueueSize)

	observeWg.Wait()
}

type adaptedRunner[PReq, POut, Q, D, E any] struct {
	updateFn           UpdateFunc[Q, E, D, PReq]
	proposeFn          ProposeFunc[PReq, POut, D]
	adapterFn          AdapterFunc[POut, Q, D]
	observeFn          ObserveFunc[Q, E]
	proposeConcurrency int
	observeConcurrency int
	maxQueueSize       int
}

func (r *adaptedRunner[PReq, POut, Q, D, E]) Run(ctx context.Context, initialTasks []PReq) {
	proposeReqCh := make(chan PReq, r.proposeConcurrency)
	proposeResCh := make(chan proposeRes[POut, D], r.proposeConcurrency)
	observeReqCh := make(chan *observeReq[Q, D], r.observeConcurrency)
	observeResCh := make(chan *observeRes[Q, E, D], r.observeConcurrency)

	var proposeWg, observeWg sync.WaitGroup

	proposeTask := func(ctx context.Context, req PReq) proposeRes[POut, D] {
		pout, data := r.proposeFn(ctx, req)
		return proposeRes[POut, D]{POut: pout, Data: data}
	}

	observeTask := func(ctx context.Context, obsReq *observeReq[Q, D]) *observeRes[Q, E, D] {
		evidence := r.observeFn(ctx, obsReq.Query)
		return &observeRes[Q, E, D]{Query: obsReq.Query, Evidence: evidence, Data: obsReq.Data}
	}

	pipeline.LaunchWorkers(ctx, &proposeWg, r.proposeConcurrency, proposeTask, proposeReqCh, proposeResCh)
	pipeline.LaunchWorkers(ctx, &observeWg, r.observeConcurrency, observeTask, observeReqCh, observeResCh)
	go func() { proposeWg.Wait(); close(proposeResCh) }()
	go r.adapterFn(proposeResCh, observeReqCh)

	update := func(ctx context.Context, res *observeRes[Q, E, D]) ([]PReq, bool) {
		return r.updateFn(ctx, res.Query, res.Evidence, res.Data)
	}
	pipeline.Loop(ctx, update, initialTasks, proposeReqCh, observeResCh, r.maxQueueSize)

	observeWg.Wait()
}
