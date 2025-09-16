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

type ProposeFunc[G any, R cmp.Ordered] func(population []Individual[G, R]) (offspring G)
type CloneFunc[G any] func(g G) G
type ReducerFunc[G any, R cmp.Ordered] func(island Island[G, R]) []Individual[G, R]
type dispatchFunc[G any, R cmp.Ordered] func(state *State[G, R], reqCh chan<- ProposeReq[G, R])

func UseReducer[G any, R cmp.Ordered](reducer ReducerFunc[G, R]) dispatchFunc[G, R] {
	return func(state *State[G, R], reqCh chan<- ProposeReq[G, R]) {
		for {
			if state.EvaluationsCount >= state.TotalEvaluations || len(state.AvailableIslandIDs) == 0 {
				return
			}
			randIndex := rand.Intn(len(state.AvailableIslandIDs))
			islandID := state.AvailableIslandIDs[randIndex]

			state.AvailableIslandIDs = append(state.AvailableIslandIDs[:randIndex], state.AvailableIslandIDs[randIndex+1:]...)
			state.PendingIslands[islandID] = true

			var selectedIsland Island[G, R]
			for _, island := range state.Islands {
				if island.ID() == islandID {
					selectedIsland = island
					break
				}
			}

			if selectedIsland != nil {
				populationForPropose := reducer(selectedIsland)
				reqCh <- ProposeReq[G, R]{
					IslandID:   islandID,
					Population: populationForPropose,
				}
			}
		}
	}
}

// --- Runner ---

// RunnerConfig holds the configuration for the GA runner.
type RunnerConfig struct {
	MigrationInterval int
	MigrationSize     int
	Concurrency       int
}

// Runner orchestrates the execution of the island model GA.
type Runner[G any, R cmp.Ordered] struct {
	config      RunnerConfig
	proposeFn   ProposeFunc[G, R]
	observeFn   ObserveFunc[G, R]
	cloneFn     CloneFunc[G]
	dispatchFn  dispatchFunc[G, R]
	initialBest R
	logCh       chan<- LogEntry[G, R]
	state       *State[G, R]
}

// NewRunner creates a new GA runner with the given functions and configuration.
func NewRunner[G any, R cmp.Ordered](
	config RunnerConfig,
	proposeFn ProposeFunc[G, R],
	observeFn ObserveFunc[G, R],
	cloneFn CloneFunc[G],
	dispatchFn dispatchFunc[G, R],
	initialBest R,
	logCh chan<- LogEntry[G, R],
	state *State[G, R],
) *Runner[G, R] {
	return &Runner[G, R]{
		config:      config,
		proposeFn:   proposeFn,
		observeFn:   observeFn,
		cloneFn:     cloneFn,
		dispatchFn:  dispatchFn,
		initialBest: initialBest,
		logCh:       logCh,
		state:       state,
	}
}

// Run executes the entire GA process using the pipeline module.
func (r *Runner[G, R]) Run() *State[G, R] {
	proposeCh := make(chan ProposeReq[G, R], r.config.Concurrency)
	observeCh := make(chan Query[G, R], r.config.Concurrency)
	resultCh := make(chan Evidence[G, R], r.config.Concurrency)

	var wgPropose, wgObserve sync.WaitGroup

	proposeTask := func(req ProposeReq[G, R]) Query[G, R] {
		offspring := r.proposeFn(req.Population)
		return Query[G, R]{IslandID: req.IslandID, Offspring: offspring}
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

	pipeline.ControlLoop(r.dispatchFn, r.propagate, r.shouldTerminate, proposeCh, resultCh, r.state)

	close(proposeCh)
	if r.logCh != nil {
		close(r.logCh)
	}
	return r.state
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

	var island Island[G, R]
	for _, i := range state.Islands {
		if i.ID() == islandID {
			island = i
			break
		}
	}
	if island == nil {
		return // Should not happen
	}

	population := island.Population()
	worstIndex := -1
	var worstFitness R
	for i, ind := range population {
		if i == 0 || ind.Fitness > worstFitness {
			worstIndex = i
			worstFitness = ind.Fitness
		}
	}
	if worstIndex != -1 && evaluatedChild.Fitness < worstFitness {
		population[worstIndex] = evaluatedChild
	}

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

func (r *Runner[G, R]) shouldTerminate(state *State[G, R]) bool {
	isEvaluationLimitReached := state.EvaluationsCount >= state.TotalEvaluations
	areAllTasksDone := len(state.PendingIslands) == 0
	return isEvaluationLimitReached && areAllTasksDone
}

func (r *Runner[G, R]) migrate(islands []Island[G, R]) {
	if len(islands) <= 1 {
		return
	}

	migrantsPerIsland := make([][]Individual[G, R], len(islands))
	for i, island := range islands {
		population := island.Population()
		sort.Slice(population, func(a, b int) bool {
			return population[a].Fitness < population[b].Fitness
		})
		migrants := make([]Individual[G, R], r.config.MigrationSize)
		for j := 0; j < r.config.MigrationSize; j++ {
			original := population[j]
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
		targetIsland := islands[targetIslandIndex]
		targetPopulation := targetIsland.Population()

		sort.Slice(targetPopulation, func(a, b int) bool {
			return targetPopulation[a].Fitness > targetPopulation[b].Fitness
		})
		for j := 0; j < r.config.MigrationSize && j < len(targetPopulation); j++ {
			targetPopulation[j] = migrants[j]
		}
	}
}
