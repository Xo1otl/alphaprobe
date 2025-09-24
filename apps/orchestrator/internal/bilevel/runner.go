package bilevel

import (
	"sync"

	"alphaprobe/orchestrator/internal/pipeline"
)

// --- Public API ---

// RunnerConfig holds the configuration for the runner.
type RunnerConfig struct {
	ProposeConcurrency int
	ObserveConcurrency int
}

// ObserveRes is the final output of the observe stage, containing both the
// evaluation evidence and the context from the propose stage.
type ObserveRes[E any, C any] struct {
	Evidence E
	Ctx      C
}

// RunnerFunc is the executable function returned by the factory. It encapsulates
// the entire pipeline logic, abstracting away the internal implementation details.
// S:   Type of the overall state object.
// PReq: Type of the input for the propose function.
// C:   Type of the context object passed through the pipeline.
// E:   Type of the evidence produced by the observe function.
type RunnerFunc[S any, PReq any, C any, E any] func(
	state S,
	dispatch func(state S, proposeCh chan<- PReq),
	propagate func(state S, observeRes ObserveRes[E, C]),
	shouldTerminate func(state S) bool,
)

// New creates a RunnerFunc for a simple, high-performance, two-stage pipeline.
// This version assumes the proposal output type is the same as the query type for the observe stage.
// It does not create any intermediate goroutines between the propose and observe stages.
// S:   Type of the overall state object.
// PReq: Type of the input for the propose function.
// Q:   Type of the query (output of propose, input of observe).
// C:   Type of the context object passed through the pipeline.
// E:   Type of the evidence produced by the observe function.
func New[S any, PReq any, Q any, C any, E any](
	config RunnerConfig,
	proposeFn ProposeFunc[PReq, Q, C],
	observeFn ObserveFunc[Q, E],
) RunnerFunc[S, PReq, C, E] {
	runner := &simpleRunner[S, PReq, Q, C, E]{
		config:    config,
		proposeFn: proposeFn,
		observeFn: observeFn,
	}
	return runner.Run
}

// NewWithAdapter creates a RunnerFunc for a pipeline that includes a custom adapter stage.
// This allows for fan-in/fan-out transformations between the propose and observe stages.
// S:    Type of the overall state object.
// PReq:  Type of the input for the propose function.
// PRes: Type of the observeRes from the propose function.
// Q:    Type of the query for the observe function.
// C:    Type of the context object passed through the pipeline.
// E:    Type of the evidence produced by the observe function.
func NewWithAdapter[S any, PReq any, PRes any, Q any, C any, E any](
	config RunnerConfig,
	proposeFn ProposeFunc[PReq, PRes, C],
	adapterFn AdapterFunc[PRes, Q, C],
	observeFn ObserveFunc[Q, E],
) RunnerFunc[S, PReq, C, E] {
	runner := &adaptedRunner[S, PReq, PRes, Q, C, E]{
		config:    config,
		proposeFn: proposeFn,
		adapterFn: adapterFn,
		observeFn: observeFn,
	}
	return runner.Run
}

// NewFanOutAdapter is a factory that takes a simple fan-out logic and returns a complete
// AdapterFunc. It handles the channel iteration and closing, allowing the user to
// focus only on the transformation logic.
func NewFanOutAdapter[PRes any, Q any, C any](
	logic FanOutLogicFunc[PRes, Q, C],
) AdapterFunc[PRes, Q, C] {
	return func(in <-chan ProposeRes[PRes, C], out chan<- ObserveReq[Q, C]) {
		defer close(out)
		for proposeRes := range in {
			logic(proposeRes, out)
		}
	}
}

// --- DI Function Types (used by Factories) ---

// ProposeFunc generates a proposal observeRes and a context object from a given input.
type ProposeFunc[PReq any, PRes any, C any] func(proposeReq PReq) (pres PRes, ctx C)

// AdapterFunc transforms observeRess from the propose stage into queries for the observe stage.
// The implementation is responsible for the entire loop and closing the output channel.
type AdapterFunc[PRes any, Q any, C any] func(in <-chan ProposeRes[PRes, C], out chan<- ObserveReq[Q, C])

// FanOutLogicFunc defines the core logic for a fan-out adapter. It processes a single
// observeRes from the propose stage and can generate multiple queries for the observe stage.
// The runner handles the channel loopreqg and closing.
type FanOutLogicFunc[PRes any, Q any, C any] func(proposeRes ProposeRes[PRes, C], out chan<- ObserveReq[Q, C])

// ObserveFunc evaluates a query and returns the evidence.
type ObserveFunc[Q any, E any] func(query Q) (evidence E)

// --- Internal Data Structures ---

// ProposeRes is an internal struct to pass data from the propose worker to the adapter.
type ProposeRes[POut any, C any] struct {
	POut POut
	Ctx  C
}

// ObserveReq is an internal struct to pass data from the adapter to the observe worker.
type ObserveReq[Q any, C any] struct {
	Query Q
	Ctx   C
}

// --- Internal Runner Implementations ---

// simpleRunner handles the direct propose-observe pipeline with no adapter.
type simpleRunner[S any, PReq any, Q any, C any, E any] struct {
	config    RunnerConfig
	proposeFn ProposeFunc[PReq, Q, C]
	observeFn ObserveFunc[Q, E]
}

// Run executes the simple pipeline process.
func (r *simpleRunner[S, PReq, Q, C, E]) Run(
	state S,
	dispatch func(state S, proposeCh chan<- PReq),
	propagate func(state S, observeRes ObserveRes[E, C]),
	shouldTerminate func(state S) bool,
) {
	proposeReqCh := make(chan PReq, r.config.ProposeConcurrency)
	observeReqCh := make(chan ObserveReq[Q, C], r.config.ObserveConcurrency)
	observeResCh := make(chan ObserveRes[E, C], r.config.ObserveConcurrency)

	var wgPropose, wgObserve sync.WaitGroup

	proposeTask := func(proposeReq PReq) ObserveReq[Q, C] {
		query, ctx := r.proposeFn(proposeReq)
		return ObserveReq[Q, C]{Query: query, Ctx: ctx}
	}

	observeTask := func(obsIn ObserveReq[Q, C]) ObserveRes[E, C] {
		evidence := r.observeFn(obsIn.Query)
		return ObserveRes[E, C]{Evidence: evidence, Ctx: obsIn.Ctx}
	}

	pipeline.WorkerPool(r.config.ProposeConcurrency, proposeTask, proposeReqCh, observeReqCh, &wgPropose)
	pipeline.WorkerPool(r.config.ObserveConcurrency, observeTask, observeReqCh, observeResCh, &wgObserve)

	go func() { wgPropose.Wait(); close(observeReqCh) }()
	go func() { wgObserve.Wait(); close(observeResCh) }()

	pipeline.ControlLoop(dispatch, propagate, shouldTerminate, proposeReqCh, observeResCh, state)
}

// adaptedRunner handles the pipeline with a fan-in/fan-out adapter stage.
type adaptedRunner[S any, PReq any, PRes any, Q any, C any, E any] struct {
	config    RunnerConfig
	proposeFn ProposeFunc[PReq, PRes, C]
	adapterFn AdapterFunc[PRes, Q, C]
	observeFn ObserveFunc[Q, E]
}

// Run executes the adapted pipeline process.
func (r *adaptedRunner[S, PReq, POut, Q, C, E]) Run(
	state S,
	dispatch func(state S, proposeCh chan<- PReq),
	propagate func(state S, observeRes ObserveRes[E, C]),
	shouldTerminate func(state S) bool,
) {
	proposeReqCh := make(chan PReq, r.config.ProposeConcurrency)
	proposeOutCh := make(chan ProposeRes[POut, C], r.config.ProposeConcurrency)
	proposeResCh := make(chan ObserveReq[Q, C], r.config.ObserveConcurrency)
	observeResCh := make(chan ObserveRes[E, C], r.config.ObserveConcurrency)

	var wgPropose, wgObserve sync.WaitGroup

	proposeTask := func(proposeReq PReq) ProposeRes[POut, C] {
		pres, ctx := r.proposeFn(proposeReq)
		return ProposeRes[POut, C]{POut: pres, Ctx: ctx}
	}

	observeTask := func(obsIn ObserveReq[Q, C]) ObserveRes[E, C] {
		evidence := r.observeFn(obsIn.Query)
		return ObserveRes[E, C]{Evidence: evidence, Ctx: obsIn.Ctx}
	}

	pipeline.WorkerPool(r.config.ProposeConcurrency, proposeTask, proposeReqCh, proposeOutCh, &wgPropose)
	pipeline.WorkerPool(r.config.ObserveConcurrency, observeTask, proposeResCh, observeResCh, &wgObserve)

	go func() { wgPropose.Wait(); close(proposeOutCh) }()
	go func() { wgObserve.Wait(); close(observeResCh) }()

	go r.adapterFn(proposeOutCh, proposeResCh)

	pipeline.ControlLoop(dispatch, propagate, shouldTerminate, proposeReqCh, observeResCh, state)
}
