package llmsr

import (
	"alphaprobe/orchestrator/internal/bilevel"
	"context"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunLLMSR_Deterministic(t *testing.T) {
	const (
		maxEvaluations     = 100
		numIslands         = 4
		migrationInterval  = 25
		proposeConcurrency = 2
		observeConcurrency = 4
		testTimeout        = 5 * time.Second
		initialScore       = 100
	)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	logger := log.Default()

	initialSkeleton := "100"
	state := NewState(initialSkeleton, maxEvaluations, numIslands, migrationInterval, logger)
	adapter := NewAdapter(logger)

	orchestrator := bilevel.NewOrchestrator(
		MockPropose,
		MockObserve,
		proposeConcurrency,
		observeConcurrency,
	)

	// Run the GA loop using the standard bilevel orchestrator with the adapter.
	bilevel.RunWithAdapter(orchestrator, ctx, state, adapter)

	// Check for timeout, which would indicate a deadlock or a hang.
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("Test timed out, indicating a potential deadlock.")
	}

	// Assertions to verify the run.
	assert.True(t, state.EvaluationsCount >= maxEvaluations, "Should have completed at least the specified number of evaluations")
	assert.Less(t, state.getBestScore(), float64(initialScore), "The final best score should be better (less) than the initial score")

	t.Logf("Test finished. Initial score: %d, Best score found: %f", initialScore, state.getBestScore())
}
