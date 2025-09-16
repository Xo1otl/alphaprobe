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
	TotalEvaluations   int
}

// --- Data Structures ---

type Island[G any, R cmp.Ordered] interface {
	ID() int
	Population() []Individual[G, R]
}

type Individual[G any, R cmp.Ordered] struct {
	Gene    G
	Fitness R
}

// --- DI Function Types ---

type ObserveFunc[G any, R cmp.Ordered] func(gene G) R

// NewInitialState creates the initial state from a slice of pre-initialized islands.
func NewInitialState[G any, R cmp.Ordered](
	islands []Island[G, R],
	observeFn ObserveFunc[G, R],
	initialBest R,
	totalEvaluations int,
) *State[G, R] {
	globalBest := Individual[G, R]{Fitness: initialBest}
	initialEvaluationCount := 0
	availableIDs := make([]int, len(islands))

	for i, island := range islands {
		availableIDs[i] = island.ID()
		population := island.Population()
		for j, ind := range population {
			// Assuming initial population might not have fitness values.
			if ind.Fitness == *new(R) {
				fitness := observeFn(ind.Gene)
				population[j].Fitness = fitness
				initialEvaluationCount++
			}
			if population[j].Fitness < globalBest.Fitness {
				globalBest = population[j]
			}
		}
	}

	return &State[G, R]{
		Islands:            islands,
		GlobalBest:         globalBest,
		PendingIslands:     make(map[int]bool),
		EvaluationsCount:   initialEvaluationCount,
		AvailableIslandIDs: availableIDs,
		TotalEvaluations:   totalEvaluations,
	}
}
