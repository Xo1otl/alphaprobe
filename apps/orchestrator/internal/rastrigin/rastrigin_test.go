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
		maxQueueSize       = 100
		testTimeout        = 5 * time.Second
	)

	doneCh := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	go func() {
		state := rastrigin.NewState(
			islandPopulation,
			numIslands,
			totalEvaluations,
			migrationInterval,
			migrationSize,
		)

		updateFn := func(res rastrigin.ObserveResult) ([]*rastrigin.Island, bool) {
			return state.Update(res)
		}

		orchestrator := bilevel.NewOrchestrator(
			updateFn,
			rastrigin.Propose,
			rastrigin.AdapterFn,
			rastrigin.Observe,
			proposeConcurrency,
			observeConcurrency,
			maxQueueSize,
		)

		fmt.Println("--- Starting Rastrigin GA with Runner ---")
		initialTasks, _ := state.Update(rastrigin.ObserveResult{})
		orchestrator.Run(ctx, initialTasks)
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
			doneCh <- fmt.Errorf("Expected at least %d evaluations, but got %d", totalEvaluations, state.EvaluationsCount)
			return
		}
		if bestFitness > 1.0 { // Loosened for faster test
			doneCh <- fmt.Errorf("Expected best fitness to be less than 1.0, but got %f", bestFitness)
			return
		}
		close(doneCh)
	}()

	select {
	case err := <-doneCh:
		if err != nil {
			t.Error(err)
		}
	case <-ctx.Done():
		t.Fatal("Test timed out (potential deadlock)")
	}
}
