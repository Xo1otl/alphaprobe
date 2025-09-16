package main_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"alphaprobe/orchestrator/internal/islandga"
	"alphaprobe/orchestrator/internal/rastrigin"
)

const (
	// Test-specific parameters
	populationSize = 50
	concurrency    = 8
	numIslands     = 5
)

func TestGAExecutionWithRastrigin(t *testing.T) {
	// 1. Configure the runner
	config := islandga.RunnerConfig{
		NumIslands:        numIslands,
		TotalEvaluations:  250000,
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

	// 3. Create a new runner and inject all dependencies
	runner := islandga.NewRunner(
		config,
		rastrigin.Propose,
		rastrigin.Observe,
		initFn,
		cloneFn,
		rastrigin.Fitness(math.MaxFloat64),
	)

	// 4. Run the GA
	startTime := time.Now()
	finalState := runner.Run()
	duration := time.Since(startTime)

	// 5. Print results and verify
	fmt.Printf("\n--- Search Finished in %s ---\n", duration)
	fmt.Printf("Total Evaluations: %d\n", finalState.EvaluationsCount)
	fmt.Printf("Final Global Best Fitness: %.8f\n", finalState.GlobalBest.Fitness)

	if finalState.GlobalBest.Fitness > 1.0 {
		t.Errorf("Expected best fitness to be close to 0, but got %f", finalState.GlobalBest.Fitness)
	}
}
