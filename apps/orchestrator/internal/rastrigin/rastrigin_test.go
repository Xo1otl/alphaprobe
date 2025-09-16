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
	islands := make([]islandga.Island[rastrigin.Gene, rastrigin.Fitness, rastrigin.InternalState], numIslands)
	for i := range numIslands {
		// NewInitialPopulation now returns evaluated individuals
		initialPopulation := rastrigin.NewInitialPopulation(populationSize)
		islands[i] = rastrigin.NewIsland(i, initialPopulation)
	}

	// 3. Find the initial global best individual from all islands
	initialGlobalBest := islandga.Individual[rastrigin.Gene, rastrigin.Fitness]{
		Fitness: rastrigin.Fitness(math.MaxFloat64),
	}
	for _, island := range islands {
		for _, ind := range island.InternalState() {
			if ind.Fitness < initialGlobalBest.Fitness {
				initialGlobalBest = ind
			}
		}
	}

	// 4. Create the initial state
	initialState := islandga.NewInitialState(
		islands,
		initialGlobalBest,
		totalEvaluations,
	)

	// 5. Setup logging
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

	// 6. Create a new runner
	runner := islandga.NewRunner(
		config,
		rastrigin.Propose,
		rastrigin.Observe,
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
	// Since the initial population is now pre-evaluated, the runner's evaluation count should match the logged evaluations.
	if loggedEvaluations != finalState.EvaluationsCount {
		t.Errorf("Logged evaluations mismatch: got %d, want %d", loggedEvaluations, finalState.EvaluationsCount)
	}

	expectedMigrations := 0
	// The evaluation count starts from 1 during the run.
	for i := 1; i <= finalState.EvaluationsCount; i++ {
		if i > 0 && i%config.MigrationInterval == 0 {
			expectedMigrations++
		}
	}
	if loggedMigrations != expectedMigrations {
		t.Errorf("Logged migrations mismatch: got %d, want %d", loggedMigrations, expectedMigrations)
	}
	fmt.Printf("Logged Evaluations: %d\n", loggedEvaluations)
	fmt.Printf("Logged Migrations: %d\n", loggedMigrations)
}
