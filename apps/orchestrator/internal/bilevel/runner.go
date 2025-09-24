package bilevel

import (
	"sync"

	"alphaprobe/orchestrator/internal/pipeline"
)

// --- Public API ---

// RunnerConfig holds the configuration for the runner.
type RunnerConfig struct {
	ProposeConcurrency int
	ObserveConcurrency  int
}

// Result is the final output of the observe stage, containing both the
// evaluation evidence and the context from the propose stage.
type Result[E any, C any] struct {
	Evidence E
	Ctx      C
}

// RunnerFunc is the executable function returned by the factory. It encapsulates
// the entire pipeline logic, abstracting away the internal implementation details.
// S:   Type of the overall state object.
// PIn: Type of the input for the propose function.
// C:   Type of the context object passed through the pipeline.
// E:   Type of the evidence produced by the observe function.
type RunnerFunc[S any, PIn any, C any, E any] func(
	state S,
	dispatch func(state S, proposeCh chan<- PIn),
	propagate func(state S, result Result[E, C]),
	shouldTerminate func(state S) bool,
)

// New creates a RunnerFunc for a simple, high-performance, two-stage pipeline.
// This version assumes the proposal output type is the same as the query type for the observe stage.
// It does not create any intermediate goroutines between the propose and observe stages.
// S:   Type of the overall state object.
// PIn: Type of the input for the propose function.
// Q:   Type of the query (output of propose, input of observe).
// C:   Type of the context object passed through the pipeline.
// E:   Type of the evidence produced by the observe function.
func New[S any, PIn any, Q any, C any, E any](
	config RunnerConfig,
	proposeFn ProposeFunc[PIn, Q, C],
	observeFn ObserveFunc[Q, E],
) RunnerFunc[S, PIn, C, E] {
	runner := &simpleRunner[S, PIn, Q, C, E]{
		config:    config,
		proposeFn: proposeFn,
		observeFn: observeFn,
	}
	return runner.Run
}

// NewWithAdapter creates a RunnerFunc for a pipeline that includes a custom adapter stage.
// This allows for fan-in/fan-out transformations between the propose and observe stages.
// S:    Type of the overall state object.
// PIn:  Type of the input for the propose function.
// PRes: Type of the result from the propose function.
// Q:    Type of the query for the observe function.
// C:    Type of the context object passed through the pipeline.
// E:    Type of the evidence produced by the observe function.
func NewWithAdapter[S any, PIn any, PRes any, Q any, C any, E any](
	config RunnerConfig,
	proposeFn ProposeFunc[PIn, PRes, C],
	adapterFn AdapterFunc[PRes, Q, C],
	observeFn ObserveFunc[Q, E],
) RunnerFunc[S, PIn, C, E] {
	runner := &adaptedRunner[S, PIn, PRes, Q, C, E]{
		config:    config,
		proposeFn: proposeFn,
		adapterFn: adapterFn,
		observeFn: observeFn,
	}
	return runner.Run
}

// --- DI Function Types (used by Factories) ---

// ProposeFunc generates a proposal result and a context object from a given input.
type ProposeFunc[PIn any, PRes any, C any] func(proposeIn PIn) (pres PRes, ctx C)

// AdapterFunc transforms results from the propose stage into queries for the observe stage.
type AdapterFunc[PRes any, Q any, C any] func(in <-chan ProposeOut[PRes, C], out chan<- ObserveIn[Q, C])

// ObserveFunc evaluates a query and returns the evidence.
type ObserveFunc[Q any, E any] func(query Q) (evidence E)

// --- Internal Data Structures ---

// ProposeOut is an internal struct to pass data from the propose worker to the adapter.
type ProposeOut[PRes any, C any] struct {
	PRes PRes
	Ctx  C
}

// ObserveIn is an internal struct to pass data from the adapter to the observe worker.
type ObserveIn[Q any, C any] struct {
	Query Q
	Ctx   C
}

// --- Internal Runner Implementations ---

// simpleRunner handles the direct propose-observe pipeline with no adapter.
type simpleRunner[S any, PIn any, Q any, C any, E any] struct {
	config    RunnerConfig
	proposeFn ProposeFunc[PIn, Q, C]
	observeFn ObserveFunc[Q, E]
}

// Run executes the simple pipeline process.
func (r *simpleRunner[S, PIn, Q, C, E]) Run(
	state S,
	dispatch func(state S, proposeCh chan<- PIn),
	propagate func(state S, result Result[E, C]),
	shouldTerminate func(state S) bool,
) {
	proposeCh := make(chan PIn, r.config.ProposeConcurrency)
	observeCh := make(chan ObserveIn[Q, C], r.config.ObserveConcurrency)
	resultCh := make(chan Result[E, C], r.config.ObserveConcurrency)

	var wgPropose, wgObserve sync.WaitGroup

	proposeTask := func(proposeIn PIn) ObserveIn[Q, C] {
		query, ctx := r.proposeFn(proposeIn)
		return ObserveIn[Q, C]{Query: query, Ctx: ctx}
	}

	observeTask := func(obsIn ObserveIn[Q, C]) Result[E, C] {
		evidence := r.observeFn(obsIn.Query)
		return Result[E, C]{Evidence: evidence, Ctx: obsIn.Ctx}
	}

	pipeline.WorkerPool(r.config.ProposeConcurrency, proposeTask, proposeCh, observeCh, &wgPropose)
	pipeline.WorkerPool(r.config.ObserveConcurrency, observeTask, observeCh, resultCh, &wgObserve)

	go func() { wgPropose.Wait(); close(observeCh) }()
	go func() { wgObserve.Wait(); close(resultCh) }()

	pipeline.ControlLoop(dispatch, propagate, shouldTerminate, proposeCh, resultCh, state)
}

// adaptedRunner handles the pipeline with a fan-in/fan-out adapter stage.
type adaptedRunner[S any, PIn any, PRes any, Q any, C any, E any] struct {
	config    RunnerConfig
	proposeFn ProposeFunc[PIn, PRes, C]
	adapterFn AdapterFunc[PRes, Q, C]
	observeFn ObserveFunc[Q, E]
}

// Run executes the adapted pipeline process.
func (r *adaptedRunner[S, PIn, PRes, Q, C, E]) Run(
	state S,
	dispatch func(state S, proposeCh chan<- PIn),
	propagate func(state S, result Result[E, C]),
	shouldTerminate func(state S) bool,
) {
	proposeInCh := make(chan PIn, r.config.ProposeConcurrency)
	proposeOutCh := make(chan ProposeOut[PRes, C], r.config.ProposeConcurrency)
	observeInCh := make(chan ObserveIn[Q, C], r.config.ObserveConcurrency)
	resultCh := make(chan Result[E, C], r.config.ObserveConcurrency)

	var wgPropose, wgAdapter, wgObserve sync.WaitGroup

	proposeTask := func(proposeIn PIn) ProposeOut[PRes, C] {
		pres, ctx := r.proposeFn(proposeIn)
		return ProposeOut[PRes, C]{PRes: pres, Ctx: ctx}
	}

	observeTask := func(obsIn ObserveIn[Q, C]) Result[E, C] {
		evidence := r.observeFn(obsIn.Query)
		return Result[E, C]{Evidence: evidence, Ctx: obsIn.Ctx}
	}

	pipeline.WorkerPool(r.config.ProposeConcurrency, proposeTask, proposeInCh, proposeOutCh, &wgPropose)
	pipeline.WorkerPool(r.config.ObserveConcurrency, observeTask, observeInCh, resultCh, &wgObserve)

	go func() { wgPropose.Wait(); close(proposeOutCh) }()
	go func() { wgObserve.Wait(); close(resultCh) }()

	wgAdapter.Go(func() {
		r.adapterFn(proposeOutCh, observeInCh)
	})
	go func() { wgAdapter.Wait(); close(observeInCh) }()

	pipeline.ControlLoop(dispatch, propagate, shouldTerminate, proposeInCh, resultCh, state)
}
