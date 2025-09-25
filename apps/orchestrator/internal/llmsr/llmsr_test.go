package llmsr_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"alphaprobe/orchestrator/internal/bilevel"
	"alphaprobe/orchestrator/internal/llmsr"
)

func TestLLMSRWithBilevelRunner(t *testing.T) {
	// --- Configuration ---
	const (
		maxEvaluations     = 1000
		proposeConcurrency = 100
		observeConcurrency = 100 // A reasonable number for a mock test
		maxQueueSize       = 10
		testTimeout        = 5 * time.Second
	)

	// --- Context and State Initialization ---
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	state := llmsr.NewState("def initial_program(x): return x", maxEvaluations)

	// --- Runner Setup ---
	adapter := bilevel.NewFanOutAdapter(llmsr.FanOut)

	run := bilevel.RunWithAdapter(
		state.Update,
		llmsr.Propose,
		adapter,
		llmsr.Observe,
		proposeConcurrency,
		observeConcurrency,
		maxQueueSize,
	)

	// --- Execution ---
	fmt.Println("--- Starting Mock LLMSR Search with adapted bilevel Runner ---")
	initialTasks := state.GetInitialTask()
	run(ctx, initialTasks)
	fmt.Println("--- Mock LLMSR Search Finished ---")

	// --- Verification ---
	fmt.Printf("Final best score: %f\n", state.BestScore)
	fmt.Printf("Total evaluations: %d\n", state.EvaluationsCount)

	if state.EvaluationsCount < maxEvaluations {
		t.Errorf("Expected at least %d evaluations, but got %d", maxEvaluations, state.EvaluationsCount)
	}
	if state.BestScore > 1.0 { // Random scores are between 0 and 1
		t.Errorf("Expected best score to be less than 1.0, but got %f", state.BestScore)
	}
	if len(state.Programs) > 10 {
		t.Errorf("Expected population size to be at most 10, but got %d", len(state.Programs))
	}
}
