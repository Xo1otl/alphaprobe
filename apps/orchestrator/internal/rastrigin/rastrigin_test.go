package rastrigin

import (
	"fmt"
	"testing"

	"alphaprobe/orchestrator/internal/bilevel"
)

func TestRastriginWithIslandV2Runner(t *testing.T) {
	// --- Configuration ---
	const (
		islandPopulation  = 50
		numIslands        = 5
		totalEvaluations  = 250000
		migrationInterval = 25
		migrationSize     = 5
		concurrency       = 8
	)

	// --- State Initialization ---
	initialState := NewInitialState(
		islandPopulation,
		numIslands,
		totalEvaluations,
		migrationInterval,
		migrationSize,
	)

	// --- Runner Setup ---
	runnerConfig := bilevel.RunnerConfig{
		Concurrency: concurrency,
	}
	// The runner is now fully generic. We provide the concrete types for our Rastrigin problem.
	runner := bilevel.NewRunner[*GaState](
		runnerConfig,
		Propose,
		Observe,
	)

	// --- Execution ---
	fmt.Println("--- Starting Rastrigin GA with redesigned island_v2 Runner ---")
	runner.Run(
		initialState,
		Dispatch,
		Propagate,
		ShouldTerminate,
	)
	fmt.Println("--- Rastrigin GA Finished ---")

	// --- Verification ---
	var bestFitness Fitness = 1e6 // A very large number
	for _, island := range initialState.Islands {
		for _, individual := range island.Population {
			if individual.Fitness < bestFitness {
				bestFitness = individual.Fitness
			}
		}
	}

	fmt.Printf("Final best fitness: %f\n", bestFitness)
	fmt.Printf("Total evaluations: %d\n", initialState.EvaluationsCount)

	if initialState.EvaluationsCount < totalEvaluations {
		t.Errorf("Expected at least %d evaluations, but got %d", totalEvaluations, initialState.EvaluationsCount)
	}
	if bestFitness > 0.001 { // Rastrigin's global minimum is 0. Expecting a value close to it.
		t.Errorf("Expected best fitness to be less than 0.001, but got %f", bestFitness)
	}
}
