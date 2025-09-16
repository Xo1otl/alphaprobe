package main_test

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
	// Test-specific parameters
	populationSize   = 50
	concurrency      = 8
	numIslands       = 5
	totalEvaluations = 250000
)

func TestGAExecutionWithRastrigin(t *testing.T) {
	// 1. Configure the runner
	config := islandga.RunnerConfig{
		NumIslands:        numIslands,
		TotalEvaluations:  totalEvaluations,
		MigrationInterval: rastrigin.MigrationInterval,
		MigrationSize:     rastrigin.MigrationSize,
		Concurrency:       concurrency,
	}

	// 2. Define the initialization and clone functions using the rastrigin package
	initFn := func(islandID int) []islandga.Individual[rastrigin.Gene, rastrigin.Fitness] {
		return rastrigin.NewInitialPopulation(populationSize)
	}

	cloneFn := func(g rastrigin.Gene) rastrigin.Gene {
		newSlice := make(rastrigin.Gene, len(g))
		copy(newSlice, g)
		return newSlice
	}

	// 3. Setup logging and a monitoring goroutine
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

	// 4. Create a new runner and inject all dependencies
	runner := islandga.NewRunner(
		config,
		rastrigin.Propose,
		rastrigin.Observe,
		initFn,
		cloneFn,
		rastrigin.Fitness(math.MaxFloat64),
		logCh,
	)

	// 5. Run the GA
	startTime := time.Now()
	finalState := runner.Run()
	duration := time.Since(startTime)

	wgLog.Wait() // Wait for the logger to finish processing

	// 6. Print results and verify
	fmt.Printf("\n--- Search Finished in %s ---\n", duration)
	fmt.Printf("Total Evaluations: %d\n", finalState.EvaluationsCount)
	fmt.Printf("Final Global Best Fitness: %.8f\n", finalState.GlobalBest.Fitness)

	if finalState.GlobalBest.Fitness > 1.0 {
		t.Errorf("Expected best fitness to be close to 0, but got %f", finalState.GlobalBest.Fitness)
	}

	// 7. Verify counts from logs
	initialEvals := numIslands * populationSize
	runtimeEvals := finalState.EvaluationsCount - initialEvals
	if loggedEvaluations != runtimeEvals {
		t.Errorf("Logged evaluations mismatch: got %d, want %d", loggedEvaluations, runtimeEvals)
	}

	// Calculate the expected number of migrations based on the actual logic.
	// The check happens for evaluation counts from (initialEvals + 1) to finalState.EvaluationsCount.
	expectedMigrations := 0
	for i := initialEvals + 1; i <= finalState.EvaluationsCount; i++ {
		if i%config.MigrationInterval == 0 {
			expectedMigrations++
		}
	}
	if loggedMigrations != expectedMigrations {
		t.Errorf("Logged migrations mismatch: got %d, want %d", loggedMigrations, expectedMigrations)
	}
	fmt.Printf("Logged Evaluations: %d (runtime)\n", loggedEvaluations)
	fmt.Printf("Logged Migrations: %d\n", loggedMigrations)
}
