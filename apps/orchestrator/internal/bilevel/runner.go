package bilevel

import (
	"sync"

	"alphaprobe/orchestrator/internal/pipeline"
)

// --- DI Function Types ---

// ProposeFunc generates a query and a context object from a given input.
// PIn: The type of the input that triggers the proposal (e.g., an island ID, or an island's state).
// Q:   The type of the query to be evaluated (e.g., a Gene).
// C:   The type of the context to be passed through to the propagation stage.
type ProposeFunc[PIn any, Q any, C any] func(proposeIn PIn) (query Q, ctx C)

// ObserveFunc evaluates a query and returns the evidence.
// Q: The type of the query to be evaluated.
// E: The type of the evidence (evaluation result, e.g., a Fitness score).
type ObserveFunc[Q any, E any] func(query Q) (evidence E)

// --- Data Structures ---

// Result is the final output of the observe stage, containing both the
// evaluation evidence and the context from the propose stage.
type Result[E any, C any] struct {
	Evidence E
	Ctx      C
}

// observeIn is an internal struct to pass data from the propose to the observe worker.
type observeIn[Q any, C any] struct {
	Query Q
	Ctx   C
}

// --- Runner ---

// RunnerConfig holds the configuration for the runner.
type RunnerConfig struct {
	Concurrency int
}

// Runner orchestrates a generic, two-stage (propose-observe) pipeline
// that transparently passes a context object from the first stage to the final processing.
// S:   Type of the overall state object.
// PIn: Type of the input for the propose function.
// Q:   Type of the query produced by the propose function.
// C:   Type of the context object passed through the pipeline.
// E:   Type of the evidence produced by the observe function.
type Runner[S any, PIn any, Q any, C any, E any] struct {
	config    RunnerConfig
	proposeFn ProposeFunc[PIn, Q, C]
	observeFn ObserveFunc[Q, E]
}

// NewRunner creates a new generic two-stage pipeline runner.
func NewRunner[S any, PIn any, Q any, C any, E any](
	config RunnerConfig,
	proposeFn ProposeFunc[PIn, Q, C],
	observeFn ObserveFunc[Q, E],
) *Runner[S, PIn, Q, C, E] {
	return &Runner[S, PIn, Q, C, E]{
		config:    config,
		proposeFn: proposeFn,
		observeFn: observeFn,
	}
}

// Run executes the entire pipeline process.
func (r *Runner[S, PIn, Q, C, E]) Run(
	state S,
	dispatch func(state S, proposeCh chan<- PIn),
	propagate func(state S, result Result[E, C]),
	shouldTerminate func(state S) bool,
) {
	proposeCh := make(chan PIn, r.config.Concurrency)
	observeCh := make(chan observeIn[Q, C], r.config.Concurrency)
	resultCh := make(chan Result[E, C], r.config.Concurrency)

	var wgPropose, wgObserve sync.WaitGroup

	// Propose Task: Takes input from dispatch, produces a query and a context.
	proposeTask := func(proposeIn PIn) observeIn[Q, C] {
		query, ctx := r.proposeFn(proposeIn)
		return observeIn[Q, C]{Query: query, Ctx: ctx}
	}

	// Observe Task: Takes a query, evaluates it, and combines the result with the context.
	observeTask := func(obsIn observeIn[Q, C]) Result[E, C] {
		evidence := r.observeFn(obsIn.Query)
		return Result[E, C]{Evidence: evidence, Ctx: obsIn.Ctx}
	}

	// Start the worker pools.
	pipeline.WorkerPool(r.config.Concurrency, proposeTask, proposeCh, observeCh, &wgPropose)
	pipeline.WorkerPool(r.config.Concurrency, observeTask, observeCh, resultCh, &wgObserve)

	go func() { wgPropose.Wait(); close(observeCh) }()
	go func() { wgObserve.Wait(); close(resultCh) }()

	// Delegate control to the generic pipeline loop.
	pipeline.ControlLoop(dispatch, propagate, shouldTerminate, proposeCh, resultCh, state)

	close(proposeCh)
}
