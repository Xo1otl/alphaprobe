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
	const (
		maxEvaluations     = 10000
		proposeConcurrency = 10
		observeConcurrency = 1
		maxQueueSize       = 1
		testTimeout        = 10 * time.Second
	)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	state := llmsr.NewState("def initial_program(x): return x", maxEvaluations)

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

	fmt.Println("--- Starting Mock LLMSR Search with adapted bilevel Runner ---")
	initialTasks := state.GetInitialTask()
	run(ctx, initialTasks)
	fmt.Println("--- Mock LLMSR Search Finished ---")

	fmt.Printf("Final best score: %f\n", state.BestScore)
	fmt.Printf("Total evaluations: %d\n", state.EvaluationsCount)

	if state.EvaluationsCount < maxEvaluations {
		t.Errorf("Expected at least %d evaluations, but got %d", maxEvaluations, state.EvaluationsCount)
	}
	if state.BestScore > 1.0 {
		t.Errorf("Expected best score to be less than 1.0, but got %f", state.BestScore)
	}
	if len(state.Programs) > 10 {
		t.Errorf("Expected population size to be at most 10, but got %d", len(state.Programs))
	}
}
