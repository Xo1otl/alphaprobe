package llmsr

import (
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

// Controller holds the entire state and logic for the LLMSR algorithm.
type Controller struct {
	Programs         []Program
	EvaluationsCount int
	MaxEvaluations   int
	BestScore        Score
	PendingParents   map[string]bool // Tracks parents used for proposal.
}

// Context is the context object passed through the pipeline.
type Context struct {
	ParentSkeletons   []ProgramSkeleton
	EvaluatedSkeleton ProgramSkeleton
}

// --- Controller Initialization ---

// NewController initializes the state for the LLMSR search.
func NewController(initialSkeleton ProgramSkeleton, maxEvaluations int) *Controller {
	initialProgram := Program{
		Skeleton: initialSkeleton,
		Score:    1e9, // A very large number representing an unevaluated score.
	}
	return &Controller{
		Programs:         []Program{initialProgram},
		EvaluationsCount: 0,
		MaxEvaluations:   maxEvaluations,
		BestScore:        1e9,
		PendingParents:   make(map[string]bool),
	}
}

// --- Core Update Logic ---

// GetInitialTask bootstraps the search process by creating the first task.
func (c *Controller) GetInitialTask() [][]Program {
	// Handle the initial bootstrap case.
	if len(c.Programs) != 1 || c.EvaluationsCount != 0 {
		return nil // Should only be called at the start.
	}

	initialProgram := c.Programs[0]
	c.PendingParents[initialProgram.Skeleton] = true
	nextTask := []Program{initialProgram, initialProgram}
	return [][]Program{nextTask}
}

// Update is the central function that drives the LLMSR process. It incorporates results,
// checks for termination, and dispatches new tasks.
// Its signature matches bilevel.UpdateFunc.
func (c *Controller) Update(skeleton ProgramSkeleton, score Score, ctx Context) ([][]Program, bool) {
	// --- 1. Incorporate the result (Propagate logic) ---
	c.EvaluationsCount++
	newProgram := Program{
		Skeleton: skeleton,
		Score:    score,
	}
	c.Programs = append(c.Programs, newProgram)

	// Keep population size fixed.
	const maxPopulation = 10
	if len(c.Programs) > maxPopulation {
		sort.Slice(c.Programs, func(i, j int) bool {
			return c.Programs[i].Score < c.Programs[j].Score // Best first
		})
		c.Programs = c.Programs[:maxPopulation]
	}

	if score < c.BestScore {
		c.BestScore = score
		fmt.Printf("New best score: %f (Evaluation #%d)\n", c.BestScore, c.EvaluationsCount)
	}

	// Release the parents from the pending state.
	for _, p := range ctx.ParentSkeletons {
		delete(c.PendingParents, p)
	}

	// --- 2. Check for termination (ShouldTerminate logic) ---
	if c.EvaluationsCount >= c.MaxEvaluations {
		return nil, true // No new tasks, and terminate.
	}

	// --- 3. Prepare the next task (Dispatch logic) ---
	// Only dispatch a new task if the previous proposal generation is fully complete.
	if len(c.PendingParents) > 0 {
		return nil, false // Wait for other results from the same batch to be processed.
	}

	// Filter for available programs.
	availablePrograms := make([]Program, 0, len(c.Programs))
	for _, p := range c.Programs {
		if !c.PendingParents[p.Skeleton] {
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
	c.PendingParents[parent1.Skeleton] = true
	c.PendingParents[parent2.Skeleton] = true

	nextTask := []Program{parent1, parent2}
	return [][]Program{nextTask}, false
}

// --- GA Logic (Propose/Observe) & Adapter ---

// Propose mocks the LLM call. It takes parent programs and returns a BATCH of new skeletons.
func Propose(parents []Program) ([]ProgramSkeleton, Context) {
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

	ctx := Context{
		ParentSkeletons: parentSkeletons,
	}
	return newSkeletons, ctx
}

// FanOut provides the core transformation for the adapter. It takes a batch of skeletons
// and creates a separate query for each one.
func FanOut(pout []ProgramSkeleton, ctx Context) []ProgramSkeleton {
	// In this case, the queries are the skeletons themselves.
	// We just need to ensure the context is correctly passed along.
	// The bilevel.NewFanOutAdapter handles attaching the context to each query.
	return pout
}

// Observe mocks the optimizer and evaluation call. It takes a single skeleton and returns a score.
func Observe(skeleton ProgramSkeleton) Score {
	return rand.Float64()
}
