package bilevel

import (
	"sync"

	"alphaprobe/orchestrator/internal/pipeline"
)

// --- Public API ---

type RunnerConfig struct {
	ProposeConcurrency int
	ObserveConcurrency int
}
type RunnerFunc[PReq any] func(initialTasks []PReq)
type UpdateFunc[E, C, PReq any] func(evidence E, ctx C) (newTasks []PReq, done bool)
type ProposeFunc[PReq any, POut any, C any] func(proposeReq PReq) (pout POut, ctx C)
type ObserveFunc[Q any, E any] func(query Q) (evidence E)
type AdapterFunc[POut any, Q any, C any] func(in <-chan proposeRes[POut, C], out chan<- *observeReq[Q, C])
type FanOutFunc[POut any, Q any, C any] func(pout POut, ctx C) []Q

// --- Internal Data Structures ---

type proposeRes[POut any, C any] struct {
	POut POut
	Ctx  C
}

type observeReq[Q any, C any] struct {
	Query Q
	Ctx   C
}

type observeRes[E any, C any] struct {
	Evidence E
	Ctx      C
}

// --- Factories ---

func NewFanOutAdapter[POut any, Q any, C any](
	logic FanOutFunc[POut, Q, C],
) AdapterFunc[POut, Q, C] {
	return func(in <-chan proposeRes[POut, C], out chan<- *observeReq[Q, C]) {
		defer close(out)
		for pRes := range in {
			queries := logic(pRes.POut, pRes.Ctx)
			for _, q := range queries {
				out <- &observeReq[Q, C]{
					Query: q,
					Ctx:   pRes.Ctx,
				}
			}
		}
	}
}

func New[PReq, Q, C, E any](
	updateFn UpdateFunc[E, C, PReq],
	proposeFn ProposeFunc[PReq, Q, C],
	observeFn ObserveFunc[Q, E],
	proposeConcurrency int,
	observeConcurrency int,
	maxQueueSize int,
) RunnerFunc[PReq] {
	r := &simpleRunner[PReq, Q, C, E]{
		updateFn:           updateFn,
		proposeFn:          proposeFn,
		observeFn:          observeFn,
		proposeConcurrency: proposeConcurrency,
		observeConcurrency: observeConcurrency,
		maxQueueSize:       maxQueueSize,
	}
	return r.Run
}

func NewWithAdapter[PReq, POut, Q, C, E any](
	updateFn UpdateFunc[E, C, PReq],
	proposeFn ProposeFunc[PReq, POut, C],
	adapterFn AdapterFunc[POut, Q, C],
	observeFn ObserveFunc[Q, E],
	proposeConcurrency int,
	observeConcurrency int,
	maxQueueSize int,
) RunnerFunc[PReq] {
	r := &adaptedRunner[PReq, POut, Q, C, E]{
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

type simpleRunner[PReq, Q, C, E any] struct {
	updateFn           UpdateFunc[E, C, PReq]
	proposeFn          ProposeFunc[PReq, Q, C]
	observeFn          ObserveFunc[Q, E]
	proposeConcurrency int
	observeConcurrency int
	maxQueueSize       int
}

func (r *simpleRunner[PReq, Q, C, E]) Run(initialTasks []PReq) {
	proposeReqCh := make(chan PReq, r.proposeConcurrency)
	observeReqCh := make(chan *observeReq[Q, C], r.observeConcurrency)
	observeResCh := make(chan *observeRes[E, C], r.observeConcurrency)

	var wgPropose, wgObserve sync.WaitGroup

	proposeTask := func(req PReq) *observeReq[Q, C] {
		q, ctx := r.proposeFn(req)
		return &observeReq[Q, C]{Query: q, Ctx: ctx}
	}

	observeTask := func(obsIn *observeReq[Q, C]) *observeRes[E, C] {
		evidence := r.observeFn(obsIn.Query)
		return &observeRes[E, C]{Evidence: evidence, Ctx: obsIn.Ctx}
	}

	pipeline.WorkerPool(r.proposeConcurrency, proposeTask, proposeReqCh, observeReqCh, &wgPropose)
	pipeline.WorkerPool(r.observeConcurrency, observeTask, observeReqCh, observeResCh, &wgObserve)

	go func() { wgPropose.Wait(); close(observeReqCh) }()
	go func() { wgObserve.Wait(); close(observeResCh) }()

	update := func(res *observeRes[E, C]) ([]PReq, bool) {
		return r.updateFn(res.Evidence, res.Ctx)
	}

	pipeline.ControlLoop(update, initialTasks, proposeReqCh, observeResCh, r.maxQueueSize)
}

type adaptedRunner[PReq, POut, Q, C, E any] struct {
	updateFn           UpdateFunc[E, C, PReq]
	proposeFn          ProposeFunc[PReq, POut, C]
	adapterFn          AdapterFunc[POut, Q, C]
	observeFn          ObserveFunc[Q, E]
	proposeConcurrency int
	observeConcurrency int
	maxQueueSize       int
}

func (r *adaptedRunner[PReq, POut, Q, C, E]) Run(initialTasks []PReq) {
	proposeReqCh := make(chan PReq, r.proposeConcurrency)
	proposeResCh := make(chan proposeRes[POut, C], r.proposeConcurrency)
	observeReqCh := make(chan *observeReq[Q, C], r.observeConcurrency)
	observeResCh := make(chan *observeRes[E, C], r.observeConcurrency)

	var wgPropose, wgObserve sync.WaitGroup

	proposeTask := func(req PReq) proposeRes[POut, C] {
		pout, ctx := r.proposeFn(req)
		return proposeRes[POut, C]{POut: pout, Ctx: ctx}
	}

	observeTask := func(obsReq *observeReq[Q, C]) *observeRes[E, C] {
		evidence := r.observeFn(obsReq.Query)
		return &observeRes[E, C]{Evidence: evidence, Ctx: obsReq.Ctx}
	}

	pipeline.WorkerPool(r.proposeConcurrency, proposeTask, proposeReqCh, proposeResCh, &wgPropose)
	pipeline.WorkerPool(r.observeConcurrency, observeTask, observeReqCh, observeResCh, &wgObserve)

	go func() { wgPropose.Wait(); close(proposeResCh) }()
	go func() { wgObserve.Wait(); close(observeResCh) }()

	go r.adapterFn(proposeResCh, observeReqCh)

	update := func(res *observeRes[E, C]) ([]PReq, bool) {
		return r.updateFn(res.Evidence, res.Ctx)
	}

	pipeline.ControlLoop(update, initialTasks, proposeReqCh, observeResCh, r.maxQueueSize)
}
