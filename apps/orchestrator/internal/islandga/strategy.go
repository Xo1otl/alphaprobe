package islandga

import (
	"cmp"
	"math/rand"
	"sort"
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

// ProposeFunc defines the function signature for creating a new offspring.
// Implementers of this function MUST return a new, independent gene (deep copy).
type ProposeFunc[G any, R cmp.Ordered] func(population []Individual[G, R]) (offspring G)

// CloneFunc defines the function signature for deep-copying a gene.
type CloneFunc[G any] func(g G) G

// --- Runner ---

// RunnerConfig holds the configuration for the GA runner.
type RunnerConfig struct {
	NumIslands        int
	TotalEvaluations  int
	MigrationInterval int
	MigrationSize     int
	Concurrency       int
}

// Runner orchestrates the execution of the island model GA.
type Runner[G any, R cmp.Ordered] struct {
	config      RunnerConfig
	proposeFn   ProposeFunc[G, R]
	observeFn   ObserveFunc[G, R]
	initFn      InitFunc[G, R]
	cloneFn     CloneFunc[G]
	initialBest R
	logCh       chan<- LogEntry[G, R]
}

// NewRunner creates a new GA runner with the given functions and configuration.
func NewRunner[G any, R cmp.Ordered](
	config RunnerConfig,
	proposeFn ProposeFunc[G, R],
	observeFn ObserveFunc[G, R],
	initFn InitFunc[G, R],
	cloneFn CloneFunc[G],
	initialBest R,
	logCh chan<- LogEntry[G, R],
) *Runner[G, R] {
	return &Runner[G, R]{
		config:      config,
		proposeFn:   proposeFn,
		observeFn:   observeFn,
		initFn:      initFn,
		cloneFn:     cloneFn,
		initialBest: initialBest,
		logCh:       logCh,
	}
}

// Run executes the entire GA process using the pipeline module.
func (r *Runner[G, R]) Run() *State[G, R] {
	proposeCh := make(chan ProposeReq[G, R], r.config.Concurrency)
	observeCh := make(chan Query[G, R], r.config.Concurrency)
	resultCh := make(chan Evidence[G, R], r.config.Concurrency)

	var wgPropose, wgObserve sync.WaitGroup

	// Define the task functions for the pipeline
	proposeTask := func(req ProposeReq[G, R]) Query[G, R] {
		offspring := r.proposeFn(req.Population)
		return Query[G, R]{
			IslandID:  req.IslandID,
			Offspring: offspring,
		}
	}
	observeTask := func(ctx Query[G, R]) Evidence[G, R] {
		fitness := r.observeFn(ctx.Offspring)
		return Evidence[G, R]{
			IslandID: ctx.IslandID,
			EvaluatedChild: Individual[G, R]{
				Gene:    ctx.Offspring,
				Fitness: fitness,
			},
		}
	}

	// Setup worker pools using the pipeline module
	pipeline.WorkerPool(r.config.Concurrency, proposeTask, proposeCh, observeCh, &wgPropose)
	pipeline.WorkerPool(r.config.Concurrency, observeTask, observeCh, resultCh, &wgObserve)

	go func() {
		wgPropose.Wait()
		close(observeCh)
	}()
	go func() {
		wgObserve.Wait()
		close(resultCh)
	}()

	state := NewInitialState(r.config.NumIslands, r.initFn, r.observeFn, r.initialBest)

	// Execute the control loop using the pipeline module
	pipeline.ControlLoop(
		r.dispatch,
		r.propagate,
		r.shouldTerminate,
		proposeCh,
		resultCh,
		state,
	)

	close(proposeCh)
	if r.logCh != nil {
		close(r.logCh)
	}

	return state
}

// --- Control Loop Logic (passed to pipeline) ---

func (r *Runner[G, R]) dispatch(state *State[G, R], reqCh chan<- ProposeReq[G, R]) {
	for {
		if state.EvaluationsCount >= r.config.TotalEvaluations || len(state.AvailableIslandIDs) == 0 {
			return
		}
		randIndex := rand.Intn(len(state.AvailableIslandIDs))
		islandID := state.AvailableIslandIDs[randIndex]

		state.AvailableIslandIDs = append(state.AvailableIslandIDs[:randIndex], state.AvailableIslandIDs[randIndex+1:]...)
		state.PendingIslands[islandID] = true

		reqCh <- ProposeReq[G, R]{
			IslandID:   islandID,
			Population: state.Islands[islandID].Population,
		}
	}
}

func (r *Runner[G, R]) propagate(state *State[G, R], result Evidence[G, R]) {
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

	island := &state.Islands[islandID]
	worstIndex := 0
	for i := 1; i < len(island.Population); i++ {
		if island.Population[i].Fitness > island.Population[worstIndex].Fitness {
			worstIndex = i
		}
	}
	if evaluatedChild.Fitness < island.Population[worstIndex].Fitness {
		island.Population[worstIndex] = evaluatedChild
	}

	if evaluatedChild.Fitness < state.GlobalBest.Fitness {
		// No clone needed here, as proposeFn guarantees a new gene.
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
			r.logCh <- LogEntry[G, R]{
				Type:       LogTypeMigration,
				Evaluation: state.EvaluationsCount,
			}
		}
	}
}

func (r *Runner[G, R]) shouldTerminate(state *State[G, R]) bool {
	isEvaluationLimitReached := state.EvaluationsCount >= r.config.TotalEvaluations
	areAllTasksDone := len(state.PendingIslands) == 0
	return isEvaluationLimitReached && areAllTasksDone
}

func (r *Runner[G, R]) migrate(islands []Island[G, R]) {
	if len(islands) <= 1 {
		return
	}

	migrantsPerIsland := make([][]Individual[G, R], len(islands))
	for i, island := range islands {
		sort.Slice(island.Population, func(a, b int) bool {
			return island.Population[a].Fitness < island.Population[b].Fitness
		})
		migrants := make([]Individual[G, R], r.config.MigrationSize)
		for j := 0; j < r.config.MigrationSize; j++ {
			// Use the injected clone function for a safe copy.
			original := island.Population[j]
			migrants[j] = Individual[G, R]{
				Gene:    r.cloneFn(original.Gene),
				Fitness: original.Fitness,
			}
		}
		migrantsPerIsland[i] = migrants
	}

	for i := range islands {
		targetIslandIndex := (i + 1) % len(islands)
		migrants := migrantsPerIsland[i]
		targetIsland := &islands[targetIslandIndex]

		sort.Slice(targetIsland.Population, func(a, b int) bool {
			return targetIsland.Population[a].Fitness > targetIsland.Population[b].Fitness
		})
		for j := 0; j < r.config.MigrationSize && j < len(targetIsland.Population); j++ {
			targetIsland.Population[j] = migrants[j]
		}
	}
}
