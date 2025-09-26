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
		maxEvaluations     = 100
		proposeConcurrency = 2
		observeConcurrency = 3
		maxQueueSize       = 2
		testTimeout        = 5 * time.Second
	)

	doneCh := make(chan error, 1) // Buffered channel to prevent goroutine leak on timeout
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	go func() {
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
		err := run(ctx, initialTasks)
		if err != nil {
			doneCh <- fmt.Errorf("Runner terminated with error: %w", err)
			return
		}
		fmt.Println("--- Mock LLMSR Search Finished ---")

		fmt.Printf("Final best score: %f\n", state.BestScore)
		fmt.Printf("Total evaluations: %d\n", state.EvaluationsCount)

		if state.EvaluationsCount < maxEvaluations {
			doneCh <- fmt.Errorf("Expected at least %d evaluations, but got %d", maxEvaluations, state.EvaluationsCount)
			return
		}
		if state.BestScore > 1.0 {
			doneCh <- fmt.Errorf("Expected best score to be less than 1.0, but got %f", state.BestScore)
			return
		}
		if len(state.Programs) > 10 {
			doneCh <- fmt.Errorf("Expected population size to be at most 10, but got %d", len(state.Programs))
			return
		}
		close(doneCh)
	}()

	select {
	case err := <-doneCh:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(testTimeout):
		t.Fatal("Test timed out after 5 seconds (potential deadlock)")
	}
}
