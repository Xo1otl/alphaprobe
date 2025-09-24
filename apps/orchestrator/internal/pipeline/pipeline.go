package pipeline

import (
	"fmt"
	"sync"
)

// WorkerPool starts a number of workers to process tasks from a request channel
// and send results to a response channel.
func WorkerPool[Req, Res any](
	numWorkers int,
	taskFn func(Req) Res,
	reqCh <-chan Req,
	resCh chan<- Res,
	wg *sync.WaitGroup,
) {
	for range numWorkers {
		wg.Go(func() {
			for req := range reqCh {
				resCh <- taskFn(req)
			}
		})
	}
}

// ControlLoop manages the overall process, dispatching tasks and propagating results.
func ControlLoop[S, Req, Res any](
	dispatch DispatchFunc[S, Req],
	propagate PropagateFunc[S, Res],
	shouldTerminate ShouldTerminateFunc[S],
	reqCh chan<- Req,
	resCh <-chan Res,
	state S,
) {
	fmt.Println("--- Starting Event-Driven Control Loop ---")

	// 1. Initial pipeline fill
	// Start by filling the pipeline with a number of tasks equal to the concurrency level.
	// This ensures that all workers are busy from the beginning.
	for i := 0; i < cap(reqCh); i++ {
		if shouldTerminate(state) {
			break
		}
		dispatch(state, reqCh)
	}

	// 2. Main processing loop
	// Continue processing results and dispatching new tasks as long as the termination
	// condition is not met.
	for !shouldTerminate(state) {
		result, ok := <-resCh
		if !ok {
			// This can happen if the worker pools panic and the pipeline shuts down prematurely.
			break
		}
		propagate(state, result)

		// Check the termination condition again after propagation, before dispatching a new task.
		if !shouldTerminate(state) {
			dispatch(state, reqCh)
		}
	}

	// 3. Graceful shutdown
	fmt.Println("\n--- Termination condition met. Closing dispatch channel... ---")
	close(reqCh) // Signal to the propose workers that no more tasks will be sent.

	fmt.Println("--- Draining remaining results from the pipeline... ---")
	// Drain any remaining results that were already in flight.
	// This loop will naturally terminate when `resCh` is closed by the runner,
	// which happens after all workers have finished.
	for result := range resCh {
		// Do nothing. Just receive to unblock the sender.
		propagate(state, result)
	}

	fmt.Println("--- All pending tasks finished. ---")
	fmt.Println("--- Control Loop Finished ---")
}

type DispatchFunc[S, Req any] = func(state S, reqCh chan<- Req)
type PropagateFunc[S, Res any] = func(state S, result Res)
type ShouldTerminateFunc[S any] = func(state S) bool
