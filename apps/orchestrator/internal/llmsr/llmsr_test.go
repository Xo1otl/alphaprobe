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
		maxEvaluations     = 1000
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

	bilevel.RunWithAdapter(orchestrator, ctx, state, adapter)

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("Test timed out, indicating a potential deadlock.")
	}

	assert.True(t, state.EvaluationsCount >= maxEvaluations, "Should have completed at least the specified number of evaluations")
	assert.Less(t, state.getBestScore(), float64(initialScore), "The final best score should be better (less) than the initial score")

	t.Logf("Test finished. Initial score: %d, Best score found: %f", initialScore, state.getBestScore())
}
