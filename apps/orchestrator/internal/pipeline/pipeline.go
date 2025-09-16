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
	dispatch func(state S, reqCh chan<- Req),
	propagate func(state S, result Res),
	shouldTerminate func(state S) bool,
	reqCh chan<- Req,
	resCh <-chan Res,
	state S,
) {
	fmt.Println("--- Starting Event-Driven Control Loop ---")

	// 1. Initial pipeline fill
	dispatch(state, reqCh)

	// 2. Event-driven loop
	for result := range resCh {
		propagate(state, result)
		if shouldTerminate(state) {
			break
		}
		dispatch(state, reqCh) // Dispatch new tasks after processing a result
	}

	// 3. Clean shutdown messaging
	fmt.Println("\n--- All pending tasks finished. ---")
	fmt.Println("--- Control Loop Finished ---")
}
