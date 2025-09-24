package llmsr

import (
	"fmt"
	"math/rand"
	"sort"

	"alphaprobe/orchestrator/internal/bilevel"
)

// --- Type Aliases for Clarity ---
// These types map the LLMSR problem to the generic types of the bilevel.Runner.
type ProgramSkeleton = string
type Score = float64
type Program struct {
	Skeleton ProgramSkeleton
	Score    Score
}

type State = *LLMSRState               // S
type ProposeIn = []Program             // PIn (List of parent programs)
type ProposeResult = []ProgramSkeleton // PRes (Result from Propose stage: a batch of skeletons)
type Query = ProgramSkeleton           // Q (Query for Observe stage: a single skeleton)
type Context = LLMSRContext            // C
type Evidence = Score                  // E

// --- Concrete Data Structures ---

// LLMSRState holds the overall state of the program search.
type LLMSRState struct {
	Programs         []Program
	EvaluationsCount int
	MaxEvaluations   int
	BestScore        Score
	// A map to track which parent programs are currently being used for proposal,
	// to avoid selecting them again until the process completes.
	PendingParents map[string]bool
}

// LLMSRContext is the context object passed through the pipeline.
// It carries information from the propose stage to the propagate stage.
type LLMSRContext struct {
	ParentSkeletons []ProgramSkeleton
	// The specific skeleton being evaluated, added by the adapter.
	EvaluatedSkeleton ProgramSkeleton
}

// --- State Initialization ---

func NewInitialState(initialSkeleton ProgramSkeleton, maxEvaluations int) *LLMSRState {
	initialProgram := Program{
		Skeleton: initialSkeleton,
		Score:    1e9, // A very large number representing unevaluated score
	}
	return &LLMSRState{
		Programs:         []Program{initialProgram},
		EvaluationsCount: 0,
		MaxEvaluations:   maxEvaluations,
		BestScore:        1e9,
		PendingParents:   make(map[string]bool),
	}
}

// --- LLMSR Runner Factory ---

// NewLLMSR creates the bilevel RunnerFunc for the LLMSR algorithm.
// It wires up the propose, observe, and a fan-out adapter function.
func NewLLMSR(config bilevel.RunnerConfig) bilevel.RunnerFunc[State, ProposeIn, Context, Evidence] {
	return bilevel.NewWithAdapter[State](
		config,
		proposeFn,
		fanOutAdapter,
		observeFn,
	)
}

// --- Mock Logic (Propose/Observe) & Adapter ---

// proposeFn mocks the LLM call. It takes parent programs and returns a BATCH of new skeletons.
func proposeFn(parents ProposeIn) (ProposeResult, Context) {
	// In a real scenario, this would make a gRPC call to the Python worker.
	// The worker would use the parent programs to construct a prompt for the LLM
	// and request multiple completions (e.g., n=4).
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

// fanOutAdapter takes a batch of skeletons from the propose stage and sends them
// individually to the observe stage.
func fanOutAdapter(
	in <-chan bilevel.ProposeOut[ProposeResult, Context],
	out chan<- bilevel.ObserveIn[Query, Context],
) {
	for proposeOut := range in {
		for _, skeleton := range proposeOut.PRes {
			// Create a new context for each individual skeleton to ensure
			// the correct one is available in the propagate stage.
			individualCtx := proposeOut.Ctx
			individualCtx.EvaluatedSkeleton = skeleton
			out <- bilevel.ObserveIn[Query, Context]{
				Query: skeleton,
				Ctx:   individualCtx,
			}
		}
	}
}

// observeFn mocks the optimizer and evaluation call. It takes a single skeleton and returns a score.
func observeFn(skeleton Query) Evidence {
	// In a real scenario, this would make a gRPC call to the Python worker.
	// The worker would optimize the parameters of the skeleton against a dataset
	// and return the final score.
	return rand.Float64()
}

// --- State Manipulation Functions for Runner ---

// Dispatch selects parent programs and sends them to the proposal channel.
func Dispatch(state *LLMSRState, proposeCh chan<- ProposeIn) {
	// Simple tournament selection to choose parents
	const tournamentSize = 2
	selectParent := func() Program {
		best := state.Programs[rand.Intn(len(state.Programs))]
		for i := 1; i < tournamentSize; i++ {
			p := state.Programs[rand.Intn(len(state.Programs))]
			// Assuming lower score is better
			if p.Score < best.Score {
				best = p
			}
		}
		return best
	}

	parent1 := selectParent()
	parent2 := selectParent()

	// For simplicity, we don't handle pending parents in this mock dispatch.
	// A real implementation would check state.PendingParents.

	proposeCh <- []Program{parent1, parent2}
}

// Propagate incorporates the evaluation result into the state.
func Propagate(state *LLMSRState, result bilevel.Result[Evidence, Context]) {
	state.EvaluationsCount++

	// The evaluated skeleton is now correctly passed in the context.
	newProgram := Program{
		Skeleton: result.Ctx.EvaluatedSkeleton,
		Score:    result.Evidence,
	}

	state.Programs = append(state.Programs, newProgram)

	// Keep the population size fixed by removing the worst program.
	const maxPopulation = 10
	if len(state.Programs) > maxPopulation {
		sort.Slice(state.Programs, func(i, j int) bool {
			return state.Programs[i].Score < state.Programs[j].Score // Best first
		})
		state.Programs = state.Programs[:maxPopulation]
	}

	if result.Evidence < state.BestScore {
		state.BestScore = result.Evidence
		fmt.Printf("New best score: %f (Evaluation #%d)\n", state.BestScore, state.EvaluationsCount)
	}
}

// ShouldTerminate checks if the evaluation limit has been reached.
func ShouldTerminate(state *LLMSRState) bool {
	return state.EvaluationsCount >= state.MaxEvaluations
}
