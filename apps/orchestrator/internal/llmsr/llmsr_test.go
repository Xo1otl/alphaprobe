package llmsr_test

import (
	"fmt"
	"testing"

	"alphaprobe/orchestrator/internal/bilevel"
	"alphaprobe/orchestrator/internal/llmsr"
)

func TestLLMSRWithBilevelRunner(t *testing.T) {
	// --- Configuration ---
	const (
		maxEvaluations     = 1000
		proposeConcurrency = 10
		observeConcurrency = 100 // A reasonable number for a mock test
		maxQueueSize       = 1000
	)

	// --- State Initialization ---
	controller := llmsr.NewController("def initial_program(x): return x", maxEvaluations)

	// --- Runner Setup ---
	adapterFn := bilevel.NewFanOutAdapter(llmsr.FanOut)

	run := bilevel.NewWithAdapter(
		controller.Update,
		llmsr.Propose,
		adapterFn,
		llmsr.Observe,
		proposeConcurrency,
		observeConcurrency,
		maxQueueSize,
	)

	// --- Execution ---
	fmt.Println("--- Starting Mock LLMSR Search with adapted bilevel Runner ---")
	initialTasks := controller.GetInitialTask()
	run(initialTasks)
	fmt.Println("--- Mock LLMSR Search Finished ---")

	// --- Verification ---
	fmt.Printf("Final best score: %f\n", controller.BestScore)
	fmt.Printf("Total evaluations: %d\n", controller.EvaluationsCount)

	if controller.EvaluationsCount < maxEvaluations {
		t.Errorf("Expected at least %d evaluations, but got %d", maxEvaluations, controller.EvaluationsCount)
	}
	if controller.BestScore > 1.0 { // Random scores are between 0 and 1
		t.Errorf("Expected best score to be less than 1.0, but got %f", controller.BestScore)
	}
	if len(controller.Programs) > 10 {
		t.Errorf("Expected population size to be at most 10, but got %d", len(controller.Programs))
	}
}
