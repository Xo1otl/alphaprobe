package island_v2

import (
	"sync"

	"alphaprobe/orchestrator/internal/pipeline"
)

// --- Data Structures ---

// Island defines the interface for a subpopulation in the island model.
// IS is the type of the island's internal state.
type Island[IS any] interface {
	ID() int
	InternalState() IS
}

// ObserveCtx is the context passed from the proposal stage to the observation stage.
// Q is the type of the query or candidate(s) to be evaluated.
type ObserveCtx[Q any] struct {
	IslandID int
	Query    Q
}

// ResultCtx is the context passed from the observation stage to the propagation stage.
// E is the type of the evidence or result from the evaluation.
type ResultCtx[E any] struct {
	IslandID int
	Evidence E
}

// --- DI Function Types ---

// ProposeFunc generates a new query (candidate) from an island's internal state.
type ProposeFunc[IS any, Q any] func(state IS) (query Q)

// ObserveFunc evaluates a query and returns the evidence (result).
type ObserveFunc[Q any, E any] func(query Q) (evidence E)

// --- Runner ---

// RunnerConfig holds the configuration for the island model runner.
type RunnerConfig struct {
	Concurrency int
}

// Runner orchestrates the two-stage (propose-observe) execution of the island model.
// It is stateless and relies on functions passed to the Run method for state manipulation.
// S:  Type of the overall state object being managed.
// IS: Type of an island's internal state.
// Q:  Type of the query produced by the propose function.
// E:  Type of the evidence produced by the observe function.
type Runner[S any, IS any, Q any, E any] struct {
	config    RunnerConfig
	proposeFn ProposeFunc[IS, Q]
	observeFn ObserveFunc[Q, E]
}

// NewRunner creates a new island model runner.
func NewRunner[S any, IS any, Q any, E any](
	config RunnerConfig,
	proposeFn ProposeFunc[IS, Q],
	observeFn ObserveFunc[Q, E],
) *Runner[S, IS, Q, E] {
	return &Runner[S, IS, Q, E]{
		config:    config,
		proposeFn: proposeFn,
		observeFn: observeFn,
	}
}

// Run executes the entire island model process using the pipeline module.
// It sets up the worker pools and delegates the control loop to the pipeline module.
func (r *Runner[S, IS, Q, E]) Run(
	state S,
	dispatch func(state S, reqCh chan<- Island[IS]),
	propagate func(state S, result ResultCtx[E]),
	shouldTerminate func(state S) bool,
) {
	proposeCh := make(chan Island[IS], r.config.Concurrency)
	observeCh := make(chan ObserveCtx[Q], r.config.Concurrency)
	resultCh := make(chan ResultCtx[E], r.config.Concurrency)

	var wgPropose, wgObserve sync.WaitGroup

	// Propose Task: Takes an Island, returns context for the Observe task.
	proposeTask := func(island Island[IS]) ObserveCtx[Q] {
		query := r.proposeFn(island.InternalState())
		return ObserveCtx[Q]{IslandID: island.ID(), Query: query}
	}

	// Observe Task: Takes observation context, returns result context.
	observeTask := func(ctx ObserveCtx[Q]) ResultCtx[E] {
		evidence := r.observeFn(ctx.Query)
		return ResultCtx[E]{IslandID: ctx.IslandID, Evidence: evidence}
	}

	// Start the worker pools that connect the channels.
	pipeline.WorkerPool(r.config.Concurrency, proposeTask, proposeCh, observeCh, &wgPropose)
	pipeline.WorkerPool(r.config.Concurrency, observeTask, observeCh, resultCh, &wgObserve)

	go func() { wgPropose.Wait(); close(observeCh) }()
	go func() { wgObserve.Wait(); close(resultCh) }()

	// Delegate state management and loop control to the generic pipeline.
	pipeline.ControlLoop(dispatch, propagate, shouldTerminate, proposeCh, resultCh, state)

	close(proposeCh)
}