package rastrigin_test

import (
	"fmt"
	"testing"

	"alphaprobe/orchestrator/internal/bilevel"
	"alphaprobe/orchestrator/internal/rastrigin"
)

func TestRastriginWithRunner(t *testing.T) {
	// --- Configuration ---
	const (
		islandPopulation   = 50
		numIslands         = 5
		totalEvaluations   = 250000
		migrationInterval  = 25
		migrationSize      = 5
		proposeConcurrency = 5
		observeConcurrency = 5
		maxQueueSize       = 1000
	)

	// --- State Initialization ---
	controller := rastrigin.NewController(
		islandPopulation,
		numIslands,
		totalEvaluations,
		migrationInterval,
		migrationSize,
	)

	// --- Runner Setup ---
	run := bilevel.New(
		controller.Update,
		rastrigin.Propose,
		rastrigin.Observe,
		proposeConcurrency,
		observeConcurrency,
		maxQueueSize,
	)

	// --- Execution ---
	fmt.Println("--- Starting Rastrigin GA with Runner ---")
	// The runner is started with initial tasks, which we get from a zero-value call to Update.
	initialTasks, _ := controller.Update(0, rastrigin.Context{})
	run(initialTasks)
	fmt.Println("--- Rastrigin GA Finished ---")

	// --- Verification ---
	var bestFitness rastrigin.Fitness = 1e6 // A very large number
	for _, island := range controller.Islands {
		for _, individual := range island.Population {
			if individual.Fitness < bestFitness {
				bestFitness = individual.Fitness
			}
		}
	}

	fmt.Printf("Final best fitness: %f\n", bestFitness)
	fmt.Printf("Total evaluations: %d\n", controller.EvaluationsCount)

	if controller.EvaluationsCount < totalEvaluations {
		t.Errorf("Expected at least %d evaluations, but got %d", totalEvaluations, controller.EvaluationsCount)
	}
	if bestFitness > 0.001 { // Rastrigin's global minimum is 0. Expecting a value close to it.
		t.Errorf("Expected best fitness to be less than 0.001, but got %f", bestFitness)
	}
}
