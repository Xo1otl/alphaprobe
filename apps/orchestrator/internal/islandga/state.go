package islandga

import "cmp"

// --- Pipeline Data ---

type ProposeReq[G any, R cmp.Ordered] struct {
	IslandID   int
	Population []Individual[G, R]
}

type Query[G any, R cmp.Ordered] struct {
	IslandID  int
	Offspring G
}

type Evidence[G any, R cmp.Ordered] struct {
	IslandID       int
	EvaluatedChild Individual[G, R]
}

// --- State Management ---

type State[G any, R cmp.Ordered] struct {
	Islands            []Island[G, R]
	GlobalBest         Individual[G, R]
	PendingIslands     map[int]bool
	EvaluationsCount   int
	AvailableIslandIDs []int
}

// --- Data Structures ---

type Individual[G any, R cmp.Ordered] struct {
	Gene    G
	Fitness R
}

type Island[G any, R cmp.Ordered] struct {
	ID         int
	Population []Individual[G, R]
}

// --- DI Function Types ---

type InitFunc[G any, R cmp.Ordered] func(islandID int) []Individual[G, R]
type ObserveFunc[G any, R cmp.Ordered] func(gene G) R

// NewInitialState creates and initializes the state for the GA using a provided init function.
func NewInitialState[G any, R cmp.Ordered](
	numIslands int,
	initFn InitFunc[G, R],
	observeFn ObserveFunc[G, R],
	initialBest R,
) *State[G, R] {
	islands := make([]Island[G, R], numIslands)
	globalBest := Individual[G, R]{Fitness: initialBest}
	initialEvaluationCount := 0
	availableIDs := make([]int, numIslands)

	for i := range numIslands {
		availableIDs[i] = i
		population := initFn(i)
		for j, ind := range population {
			fitness := observeFn(ind.Gene)
			population[j].Fitness = fitness
			initialEvaluationCount++
			if fitness < globalBest.Fitness {
				globalBest = population[j]
			}
		}
		islands[i] = Island[G, R]{ID: i, Population: population}
	}

	return &State[G, R]{
		Islands:            islands,
		GlobalBest:         globalBest,
		PendingIslands:     make(map[int]bool),
		EvaluationsCount:   initialEvaluationCount,
		AvailableIslandIDs: availableIDs,
	}
}
