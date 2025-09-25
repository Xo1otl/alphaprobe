package rastrigin_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"alphaprobe/orchestrator/internal/bilevel"
	"alphaprobe/orchestrator/internal/rastrigin"
)

func TestRastriginWithRunner(t *testing.T) {
	const (
		islandPopulation   = 50
		numIslands         = 5
		totalEvaluations   = 250000
		migrationInterval  = 25
		migrationSize      = 5
		proposeConcurrency = 5
		observeConcurrency = 5
		maxQueueSize       = 1000
		testTimeout        = 10 * time.Second
	)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	state := rastrigin.NewState(
		islandPopulation,
		numIslands,
		totalEvaluations,
		migrationInterval,
		migrationSize,
	)

	run := bilevel.Run(
		state.Update,
		rastrigin.Propose,
		rastrigin.Observe,
		proposeConcurrency,
		observeConcurrency,
		maxQueueSize,
	)

	fmt.Println("--- Starting Rastrigin GA with Runner ---")
	initialTasks, _ := state.Update(ctx, nil, 0, rastrigin.Metadata{})
	run(ctx, initialTasks)
	fmt.Println("--- Rastrigin GA Finished ---")

	var bestFitness rastrigin.Fitness = 1e6
	for _, island := range state.Islands {
		for _, individual := range island.Population {
			if individual.Fitness < bestFitness {
				bestFitness = individual.Fitness
			}
		}
	}

	fmt.Printf("Final best fitness: %f\n", bestFitness)
	fmt.Printf("Total evaluations: %d\n", state.EvaluationsCount)

	if state.EvaluationsCount < totalEvaluations {
		t.Errorf("Expected at least %d evaluations, but got %d", totalEvaluations, state.EvaluationsCount)
	}
	if bestFitness > 0.001 {
		t.Errorf("Expected best fitness to be less than 0.001, but got %f", bestFitness)
	}
}
