package islandga

import "cmp"

// --- Pipeline Data ---

type Query[G any, R cmp.Ordered] struct {
	IslandID  int
	Offspring G
}

type Evidence[G any, R cmp.Ordered] struct {
	IslandID       int
	EvaluatedChild Individual[G, R]
}

// --- State Management ---

type State[G any, R cmp.Ordered, S any] struct {
	Islands            []Island[G, R, S]
	GlobalBest         Individual[G, R]
	PendingIslands     map[int]bool
	EvaluationsCount   int
	AvailableIslandIDs []int
	TotalEvaluations   int
}

// --- Data Structures ---

type Island[G any, R cmp.Ordered, S any] interface {
	ID() int
	InternalState() S
	Incorporate(individuals []Individual[G, R])
	SelectMigrants(n int) []Individual[G, R]
}

type Individual[G any, R cmp.Ordered] struct {
	Gene    G
	Fitness R
}

// --- DI Function Types ---

type ObserveFunc[G any, R cmp.Ordered] func(gene G) R

// NewInitialState creates the initial state from a slice of pre-initialized islands.
func NewInitialState[G any, R cmp.Ordered, S any](
	islands []Island[G, R, S],
	initialGlobalBest Individual[G, R],
	totalEvaluations int,
) *State[G, R, S] {
	availableIDs := make([]int, len(islands))
	for i, island := range islands {
		availableIDs[i] = island.ID()
	}

	return &State[G, R, S]{
		Islands:            islands,
		GlobalBest:         initialGlobalBest,
		PendingIslands:     make(map[int]bool),
		EvaluationsCount:   0, // Evaluations start from 0, initial population is pre-evaluated.
		AvailableIslandIDs: availableIDs,
		TotalEvaluations:   totalEvaluations,
	}
}
