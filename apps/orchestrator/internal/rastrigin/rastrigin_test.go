package rastrigin

import (
	"fmt"
	"testing"

	"alphaprobe/orchestrator/internal/bilevel"
)

func TestRastriginWithIslandV2Runner(t *testing.T) {
	// --- Configuration ---
	const (
		islandPopulation   = 50
		numIslands         = 5
		totalEvaluations   = 250000
		migrationInterval  = 25
		migrationSize      = 5
		proposeConcurrency = 8 // 処理速度の差により、たとえproposeとobserveが1:1対応していても、並列数を分けた方が良い場合がある。
		observeConcurrency = 8
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
		ProposeConcurrency: proposeConcurrency,
		ObserveConcurrency: observeConcurrency,
	}
	run := bilevel.New[*GaState](
		runnerConfig,
		Propose,
		Observe,
	)

	// --- Execution ---
	fmt.Println("--- Starting Rastrigin GA with redesigned island_v2 Runner ---")
	run(
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
