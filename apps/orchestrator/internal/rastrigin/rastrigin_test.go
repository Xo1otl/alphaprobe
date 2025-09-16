package rastrigin_test

import (
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"alphaprobe/orchestrator/internal/islandga"
	"alphaprobe/orchestrator/internal/rastrigin"
)

const (
	populationSize   = 50
	concurrency      = 8
	numIslands       = 5
	totalEvaluations = 250000
)

func TestGAExecutionWithRastrigin(t *testing.T) {
	// 1. Configure the runner
	config := islandga.RunnerConfig{
		MigrationInterval: rastrigin.MigrationInterval,
		MigrationSize:     rastrigin.MigrationSize,
		Concurrency:       concurrency,
	}

	// 2. Create and initialize the islands
	islands := make([]islandga.Island[rastrigin.Gene, rastrigin.Fitness], numIslands)
	for i := range numIslands {
		initialPopulation := rastrigin.NewInitialPopulation(populationSize)
		islands[i] = rastrigin.NewIsland(i, initialPopulation)
	}

	// 3. Create the initial state
	initialState := islandga.NewInitialState(
		islands,
		rastrigin.Observe,
		rastrigin.Fitness(math.MaxFloat64),
		totalEvaluations,
	)

	// 4. Setup logging
	logCh := make(chan islandga.LogEntry[rastrigin.Gene, rastrigin.Fitness], config.Concurrency*2)
	var wgLog sync.WaitGroup
	wgLog.Add(1)
	var loggedEvaluations, loggedMigrations int
	go func() {
		defer wgLog.Done()
		for entry := range logCh {
			switch entry.Type {
			case islandga.LogTypeEvaluation:
				loggedEvaluations++
			case islandga.LogTypeMigration:
				loggedMigrations++
			}
		}
	}()

	// 5. Define the clone function
	cloneFn := func(g rastrigin.Gene) rastrigin.Gene {
		newSlice := make(rastrigin.Gene, len(g))
		copy(newSlice, g)
		return newSlice
	}

	// 6. Create a new runner
	runner := islandga.NewRunner(
		config,
		rastrigin.Propose,
		rastrigin.Observe,
		cloneFn,
		islandga.UseReducer(rastrigin.SimpleReducer),
		rastrigin.Fitness(math.MaxFloat64),
		logCh,
		initialState,
	)

	// 7. Run the GA
	startTime := time.Now()
	finalState := runner.Run()
	duration := time.Since(startTime)
	wgLog.Wait()

	// 8. Print results and verify
	fmt.Printf("\n--- Search Finished in %s ---\n", duration)
	fmt.Printf("Total Evaluations: %d\n", finalState.EvaluationsCount)
	fmt.Printf("Final Global Best Fitness: %.8f\n", finalState.GlobalBest.Fitness)

	if finalState.GlobalBest.Fitness > 1.0 {
		t.Errorf("Expected best fitness to be close to 0, but got %f", finalState.GlobalBest.Fitness)
	}

	// 9. Verify counts from logs
	initialEvals := 0
	for _, island := range islands {
		initialEvals += len(island.Population())
	}
	runtimeEvals := finalState.EvaluationsCount - initialEvals
	if loggedEvaluations != runtimeEvals {
		t.Errorf("Logged evaluations mismatch: got %d, want %d", loggedEvaluations, runtimeEvals)
	}

	expectedMigrations := 0
	for i := initialEvals + 1; i <= finalState.EvaluationsCount; i++ {
		if i > 0 && i%config.MigrationInterval == 0 {
			expectedMigrations++
		}
	}
	if loggedMigrations != expectedMigrations {
		t.Errorf("Logged migrations mismatch: got %d, want %d", loggedMigrations, expectedMigrations)
	}
	fmt.Printf("Logged Evaluations: %d (runtime)\n", loggedEvaluations)
	fmt.Printf("Logged Migrations: %d\n", loggedMigrations)
}
