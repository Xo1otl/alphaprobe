package llmsr

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
)

// --- Type Aliases for Clarity ---
type ProgramSkeleton = string
type Score = float64
type Program struct {
	Skeleton ProgramSkeleton
	Score    Score
}

// --- Concrete Data Structures ---

// State holds the entire state and logic for the LLMSR algorithm.
// It is a passive data structure managed by the pipeline controller.
type State struct {
	Programs         []Program
	EvaluationsCount int
	MaxEvaluations   int
	BestScore        Score
	PendingParents   map[string]bool // Tracks parents used for proposal.
}

// Lineage is the metadata object passed through the pipeline.
// It tracks the parent-child relationships between programs.
type Lineage struct {
	ParentSkeletons []ProgramSkeleton
}

// --- State Initialization ---

// NewState initializes the state for the LLMSR search.
func NewState(initialSkeleton ProgramSkeleton, maxEvaluations int) *State {
	initialProgram := Program{
		Skeleton: initialSkeleton,
		Score:    1e9, // A very large number representing an unevaluated score.
	}
	return &State{
		Programs:         []Program{initialProgram},
		EvaluationsCount: 0,
		MaxEvaluations:   maxEvaluations,
		BestScore:        1e9,
		PendingParents:   make(map[string]bool),
	}
}

// --- Core Update Logic ---

// GetInitialTask bootstraps the search process by creating the first task.
func (s *State) GetInitialTask() [][]Program {
	// Handle the initial bootstrap case.
	if len(s.Programs) != 1 || s.EvaluationsCount != 0 {
		return nil // Should only be called at the start.
	}

	initialProgram := s.Programs[0]
	s.PendingParents[initialProgram.Skeleton] = true
	nextTask := []Program{initialProgram, initialProgram}
	return [][]Program{nextTask}
}

// Update is the central function that drives the LLMSR process. It incorporates results,
// checks for termination, and dispatches new tasks.
// Its signature matches bilevel.UpdateFunc.
func (s *State) Update(ctx context.Context, skeleton ProgramSkeleton, score Score, lineage Lineage) ([][]Program, bool) {
	// --- 1. Incorporate the result (Propagate logic) ---
	s.EvaluationsCount++
	newProgram := Program{
		Skeleton: skeleton,
		Score:    score,
	}
	s.Programs = append(s.Programs, newProgram)

	// Keep population size fixed.
	const maxPopulation = 10
	if len(s.Programs) > maxPopulation {
		sort.Slice(s.Programs, func(i, j int) bool {
			return s.Programs[i].Score < s.Programs[j].Score // Best first
		})
		s.Programs = s.Programs[:maxPopulation]
	}

	if score < s.BestScore {
		s.BestScore = score
		fmt.Printf("New best score: %f (Evaluation #%d)\n", s.BestScore, s.EvaluationsCount)
	}

	// Release the parents from the pending state.
	for _, p := range lineage.ParentSkeletons {
		delete(s.PendingParents, p)
	}

	// --- 2. Check for termination (ShouldTerminate logic) ---
	if s.EvaluationsCount >= s.MaxEvaluations {
		return nil, true // No new tasks, and terminate.
	}

	// --- 3. Prepare the next task (Dispatch logic) ---
	// Only dispatch a new task if the previous proposal generation is fully complete.
	if len(s.PendingParents) > 0 {
		return nil, false // Wait for other results from the same batch to be processed.
	}

	// Filter for available programs.
	availablePrograms := make([]Program, 0, len(s.Programs))
	for _, p := range s.Programs {
		if !s.PendingParents[p.Skeleton] {
			availablePrograms = append(availablePrograms, p)
		}
	}

	// If not enough parents are available, the search space is exhausted.
	if len(availablePrograms) < 2 {
		return nil, true
	}

	// Shuffle and select two distinct parents.
	rand.Shuffle(len(availablePrograms), func(i, j int) {
		availablePrograms[i], availablePrograms[j] = availablePrograms[j], availablePrograms[i]
	})
	parent1 := availablePrograms[0]
	parent2 := availablePrograms[1]
	s.PendingParents[parent1.Skeleton] = true
	s.PendingParents[parent2.Skeleton] = true

	nextTask := []Program{parent1, parent2}
	return [][]Program{nextTask}, false
}

// --- GA Logic (Propose/Observe) & Adapter ---

// Propose mocks the LLM call. It takes parent programs and returns a BATCH of new skeletons.
func Propose(ctx context.Context, parents []Program) ([]ProgramSkeleton, Lineage) {
	batchSize := rand.Intn(4) + 1 // Generate 1 to 4 new skeletons
	newSkeletons := make([]ProgramSkeleton, 0, batchSize)
	for range batchSize {
		newSkeleton := fmt.Sprintf("%s\n# Mutated %d", parents[0].Skeleton, rand.Intn(100))
		newSkeletons = append(newSkeletons, newSkeleton)
	}

	parentSkeletons := make([]ProgramSkeleton, len(parents))
	for i, p := range parents {
		parentSkeletons[i] = p.Skeleton
	}

	lineage := Lineage{
		ParentSkeletons: parentSkeletons,
	}
	return newSkeletons, lineage
}

// FanOut provides the core transformation for the adapter. It takes a batch of skeletons
// and creates a separate query for each one.
func FanOut(pout []ProgramSkeleton, data Lineage) []ProgramSkeleton {
	// In this case, the queries are the skeletons themselves.
	// We just need to ensure the context is correctly passed along.
	// The bilevel.NewFanOutAdapter handles attaching the context to each query.
	return pout
}

// Observe mocks the optimizer and evaluation call. It takes a single skeleton and returns a score.
func Observe(ctx context.Context, skeleton ProgramSkeleton) Score {
	// In a real scenario, we might check ctx.Done() here if the evaluation is long.
	return rand.Float64()
}