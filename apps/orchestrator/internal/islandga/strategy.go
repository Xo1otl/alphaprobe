package islandga

import (
	"cmp"
	"math/rand"
	"sync"

	"alphaprobe/orchestrator/internal/pipeline"
)

// --- Logging ---

type LogType int

const (
	LogTypeEvaluation LogType = iota
	LogTypeMigration
	LogTypeGlobalBestUpdate
)

type LogEntry[G any, R cmp.Ordered] struct {
	Type       LogType
	Evaluation int
	IslandID   int
	Fitness    R
	GlobalBest Individual[G, R]
}

// --- DI Function Types ---

type ProposeFunc[G any, S any] func(state S) (offspring G)

// --- Runner ---

// RunnerConfig holds the configuration for the GA runner.
type RunnerConfig struct {
	MigrationInterval int
	MigrationSize     int
	Concurrency       int
}

// Runner orchestrates the execution of the island model GA.
type Runner[G any, R cmp.Ordered, S any] struct {
	config      RunnerConfig
	proposeFn   ProposeFunc[G, S]
	observeFn   ObserveFunc[G, R]
	cloneFn     CloneFunc[G]
	initialBest R
	logCh       chan<- LogEntry[G, R]
	state       *State[G, R, S]
}

// NewRunner creates a new GA runner with the given functions and configuration.
func NewRunner[G any, R cmp.Ordered, S any](
	config RunnerConfig,
	proposeFn ProposeFunc[G, S],
	observeFn ObserveFunc[G, R],
	cloneFn CloneFunc[G],
	initialBest R,
	logCh chan<- LogEntry[G, R],
	state *State[G, R, S],
) *Runner[G, R, S] {
	return &Runner[G, R, S]{
		config:      config,
		proposeFn:   proposeFn,
		observeFn:   observeFn,
		cloneFn:     cloneFn,
		initialBest: initialBest,
		logCh:       logCh,
		state:       state,
	}
}

// Run executes the entire GA process using the pipeline module.
func (r *Runner[G, R, S]) Run() *State[G, R, S] {
	proposeCh := make(chan Island[G, R, S], r.config.Concurrency)
	observeCh := make(chan Query[G, R], r.config.Concurrency)
	resultCh := make(chan Evidence[G, R], r.config.Concurrency)

	var wgPropose, wgObserve sync.WaitGroup

	proposeTask := func(req Island[G, R, S]) Query[G, R] {
		islandState := req.InternalState()
		offspring := r.proposeFn(islandState)
		return Query[G, R]{IslandID: req.ID(), Offspring: offspring}
	}
	observeTask := func(ctx Query[G, R]) Evidence[G, R] {
		fitness := r.observeFn(ctx.Offspring)
		return Evidence[G, R]{
			IslandID:       ctx.IslandID,
			EvaluatedChild: Individual[G, R]{Gene: ctx.Offspring, Fitness: fitness},
		}
	}

	pipeline.WorkerPool(r.config.Concurrency, proposeTask, proposeCh, observeCh, &wgPropose)
	pipeline.WorkerPool(r.config.Concurrency, observeTask, observeCh, resultCh, &wgObserve)

	go func() { wgPropose.Wait(); close(observeCh) }()
	go func() { wgObserve.Wait(); close(resultCh) }()

	pipeline.ControlLoop(r.dispatch, r.propagate, r.shouldTerminate, proposeCh, resultCh, r.state)

	close(proposeCh)
	if r.logCh != nil {
		close(r.logCh)
	}
	return r.state
}

func (r *Runner[G, R, S]) dispatch(state *State[G, R, S], reqCh chan<- Island[G, R, S]) {
	for {
		if state.EvaluationsCount >= state.TotalEvaluations || len(state.AvailableIslandIDs) == 0 {
			return
		}
		randIndex := rand.Intn(len(state.AvailableIslandIDs))
		islandID := state.AvailableIslandIDs[randIndex]

		state.AvailableIslandIDs = append(state.AvailableIslandIDs[:randIndex], state.AvailableIslandIDs[randIndex+1:]...)
		state.PendingIslands[islandID] = true

		var selectedIsland Island[G, R, S]
		for _, island := range state.Islands {
			if island.ID() == islandID {
				selectedIsland = island
				break
			}
		}

		if selectedIsland != nil {
			reqCh <- selectedIsland
		}
	}
}

func (r *Runner[G, R, S]) propagate(state *State[G, R, S], result Evidence[G, R]) {
	islandID := result.IslandID
	evaluatedChild := result.EvaluatedChild

	delete(state.PendingIslands, islandID)
	state.EvaluationsCount++
	state.AvailableIslandIDs = append(state.AvailableIslandIDs, islandID)

	if r.logCh != nil {
		r.logCh <- LogEntry[G, R]{
			Type:       LogTypeEvaluation,
			Evaluation: state.EvaluationsCount,
			IslandID:   islandID,
			Fitness:    evaluatedChild.Fitness,
		}
	}

	var island Island[G, R, S]
	for _, i := range state.Islands {
		if i.ID() == islandID {
			island = i
			break
		}
	}
	if island == nil {
		return // Should not happen
	}

	island.Incorporate([]Individual[G, R]{evaluatedChild})

	if evaluatedChild.Fitness < state.GlobalBest.Fitness {
		state.GlobalBest = evaluatedChild
		if r.logCh != nil {
			r.logCh <- LogEntry[G, R]{
				Type:       LogTypeGlobalBestUpdate,
				Evaluation: state.EvaluationsCount,
				GlobalBest: state.GlobalBest,
			}
		}
	}

	if state.EvaluationsCount%r.config.MigrationInterval == 0 && state.EvaluationsCount > 0 {
		r.migrate(state.Islands)
		if r.logCh != nil {
			r.logCh <- LogEntry[G, R]{Type: LogTypeMigration, Evaluation: state.EvaluationsCount}
		}
	}
}

func (r *Runner[G, R, S]) shouldTerminate(state *State[G, R, S]) bool {
	isEvaluationLimitReached := state.EvaluationsCount >= state.TotalEvaluations
	areAllTasksDone := len(state.PendingIslands) == 0
	return isEvaluationLimitReached && areAllTasksDone
}

func (r *Runner[G, R, S]) migrate(islands []Island[G, R, S]) {
	if len(islands) <= 1 {
		return
	}

	allMigrants := make([][]Individual[G, R], len(islands))
	for i, island := range islands {
		allMigrants[i] = island.SelectMigrants(r.config.MigrationSize, r.cloneFn)
	}

	for i, sourceIslandMigrants := range allMigrants {
		targetIslandIndex := (i + 1) % len(islands)
		targetIsland := islands[targetIslandIndex]
		targetIsland.Incorporate(sourceIslandMigrants)
	}
}
