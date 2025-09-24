package llmsr

import (
	"fmt"
	"testing"

	"alphaprobe/orchestrator/internal/bilevel"
)

func TestLLMSRWithBilevelRunner(t *testing.T) {
	// --- Configuration ---
	const (
		maxEvaluations     = 100
		proposeConcurrency = 4
		observeConcurrency = 100
	)

	// --- State Initialization ---
	initialState := NewInitialState("def initial_program(x): return x", maxEvaluations)

	// --- Runner Setup ---
	// The new NewLLMSR factory encapsulates the creation of the runner with the fan-out adapter.
	// No more workarounds are needed; the standard functions can be used directly.
	runnerConfig := bilevel.RunnerConfig{
		ProposeConcurrency: proposeConcurrency,
		ObserveConcurrency: observeConcurrency * 4,
	}
	run := NewLLMSR(runnerConfig)

	// --- Execution ---
	fmt.Println("--- Starting Mock LLMSR Search with adapted bilevel Runner ---")
	run(
		initialState,
		Dispatch,
		Propagate,
		ShouldTerminate,
	)
	fmt.Println("--- Mock LLMSR Search Finished ---")

	// --- Verification ---
	fmt.Printf("Final best score: %f\n", initialState.BestScore)
	fmt.Printf("Total evaluations: %d\n", initialState.EvaluationsCount)

	if initialState.EvaluationsCount < maxEvaluations {
		t.Errorf("Expected at least %d evaluations, but got %d", maxEvaluations, initialState.EvaluationsCount)
	}
	if initialState.BestScore > 1.0 { // Random scores are between 0 and 1
		t.Errorf("Expected best score to be less than 1.0, but got %f", initialState.BestScore)
	}
	if len(initialState.Programs) > 10 {
		t.Errorf("Expected population size to be at most 10, but got %d", len(initialState.Programs))
	}
}
