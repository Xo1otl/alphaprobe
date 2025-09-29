package mock_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"alphaprobe/orchestrator/internal/bilevel"
	"alphaprobe/orchestrator/internal/mock"
)

func TestLLMSRWithBilevelRunner(t *testing.T) {
	const (
		maxEvaluations     = 1000
		proposeConcurrency = 2
		observeConcurrency = 3
		testTimeout        = 5 * time.Second
	)

	doneCh := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	go func() {
		state := mock.NewState("def initial_program(x): return x", maxEvaluations)
		adapter := mock.NewAdapter()

		orchestrator := bilevel.NewOrchestrator(
			mock.Propose,
			mock.Observe,
			proposeConcurrency,
			observeConcurrency,
		)

		fmt.Println("--- Starting Mock LLMSR Search with bilevelv2 Runner ---")
		bilevel.RunWithAdapter(orchestrator, ctx, state, adapter)
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
	case <-ctx.Done():
		t.Fatal("Test timed out (potential deadlock)")
	}
}
