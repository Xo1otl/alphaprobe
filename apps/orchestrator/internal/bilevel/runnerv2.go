package bilevel

import (
	"sync"

	"alphaprobe/orchestrator/internal/pipeline"
)

// RunnerV2Func is the executable function returned by the V2 factories.
// It accepts initial tasks to start the execution.
type RunnerV2Func[PIn any] func(initialTasks []PIn)

// --- Factories ---

// NewV2 creates a runner for a simple, adapter-less pipeline.
func NewV2[PIn, Q, C, E any](
	updateLogic func(observeRes *ObserveRes[E, C]) ([]PIn, bool),
	proposeFn ProposeFunc[PIn, Q, C],
	observeFn ObserveFunc[Q, E],
	proposeConcurrency int,
	observeConcurrency int,
	maxQueueSize int,
) RunnerV2Func[PIn] {
	r := &simpleRunnerV2[PIn, Q, C, E]{
		updateLogic:        updateLogic,
		proposeFn:          proposeFn,
		observeFn:          observeFn,
		proposeConcurrency: proposeConcurrency,
		observeConcurrency: observeConcurrency,
		maxQueueSize:       maxQueueSize,
	}
	return r.Run
}

// NewWithAdapterV2 creates a runner for a pipeline that includes a custom adapter stage.
func NewWithAdapterV2[PIn, POut, Q, C, E any](
	updateLogic func(observeRes *ObserveRes[E, C]) ([]PIn, bool),
	proposeFn ProposeFunc[PIn, POut, C],
	adapterFn AdapterFunc[POut, Q, C],
	observeFn ObserveFunc[Q, E],
	proposeConcurrency int,
	observeConcurrency int,
	maxQueueSize int,
) RunnerV2Func[PIn] {
	r := &adaptedRunnerV2[PIn, POut, Q, C, E]{
		updateLogic:        updateLogic,
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

// simpleRunnerV2 manages a pipeline without an adapter.
type simpleRunnerV2[PReq, Q, C, E any] struct {
	updateLogic        func(observeRes *ObserveRes[E, C]) ([]PReq, bool)
	proposeFn          ProposeFunc[PReq, Q, C]
	observeFn          ObserveFunc[Q, E]
	proposeConcurrency int
	observeConcurrency int
	maxQueueSize       int
}

func (r *simpleRunnerV2[PReq, Q, C, E]) Run(initialTasks []PReq) {
	proposeReqCh := make(chan PReq, r.proposeConcurrency)
	observeReqCh := make(chan ObserveReq[Q, C], r.observeConcurrency)
	observeResCh := make(chan ObserveRes[E, C], r.observeConcurrency)

	var wgPropose, wgObserve sync.WaitGroup

	proposeTask := func(req PReq) ObserveReq[Q, C] {
		q, ctx := r.proposeFn(req)
		return ObserveReq[Q, C]{Query: q, Ctx: ctx}
	}
	pipeline.WorkerPool(r.proposeConcurrency, proposeTask, proposeReqCh, observeReqCh, &wgPropose)

	observeTask := func(obsIn ObserveReq[Q, C]) ObserveRes[E, C] {
		evidence := r.observeFn(obsIn.Query)
		return ObserveRes[E, C]{Evidence: evidence, Ctx: obsIn.Ctx}
	}
	pipeline.WorkerPool(r.observeConcurrency, observeTask, observeReqCh, observeResCh, &wgObserve)

	go func() { wgPropose.Wait(); close(observeReqCh) }()
	go func() { wgObserve.Wait(); close(observeResCh) }()

	updateFn := func(observeRes ObserveRes[E, C]) ([]PReq, bool) {
		return r.updateLogic(&observeRes)
	}

	pipeline.ControlLoopV2(updateFn, initialTasks, proposeReqCh, observeResCh, r.maxQueueSize)
}

// adaptedRunnerV2 manages a pipeline with a custom adapter stage.
type adaptedRunnerV2[PReq, POut, Q, C, E any] struct {
	updateLogic        func(observeRes *ObserveRes[E, C]) ([]PReq, bool)
	proposeFn          ProposeFunc[PReq, POut, C]
	adapterFn          AdapterFunc[POut, Q, C]
	observeFn          ObserveFunc[Q, E]
	proposeConcurrency int
	observeConcurrency int
	maxQueueSize       int
}

func (r *adaptedRunnerV2[PReq, POut, Q, C, E]) Run(initialTasks []PReq) {
	proposeReqCh := make(chan PReq, r.proposeConcurrency)
	proposeResCh := make(chan ProposeRes[POut, C], r.proposeConcurrency)
	observeReqCh := make(chan ObserveReq[Q, C], r.observeConcurrency)
	observeResCh := make(chan ObserveRes[E, C], r.observeConcurrency)

	var wgPropose, wgObserve sync.WaitGroup

	proposeTask := func(req PReq) ProposeRes[POut, C] {
		pout, ctx := r.proposeFn(req)
		return ProposeRes[POut, C]{POut: pout, Ctx: ctx}
	}
	pipeline.WorkerPool(r.proposeConcurrency, proposeTask, proposeReqCh, proposeResCh, &wgPropose)

	go r.adapterFn(proposeResCh, observeReqCh)

	observeTask := func(obsReq ObserveReq[Q, C]) ObserveRes[E, C] {
		evidence := r.observeFn(obsReq.Query)
		return ObserveRes[E, C]{Evidence: evidence, Ctx: obsReq.Ctx}
	}
	pipeline.WorkerPool(r.observeConcurrency, observeTask, observeReqCh, observeResCh, &wgObserve)

	go func() { wgPropose.Wait(); close(proposeResCh) }()
	go func() { wgObserve.Wait(); close(observeResCh) }()

	updateFunc := func(observeRes ObserveRes[E, C]) ([]PReq, bool) {
		return r.updateLogic(&observeRes)
	}

	pipeline.ControlLoopV2(updateFunc, initialTasks, proposeReqCh, observeResCh, r.maxQueueSize)
}
