package pipeline

import "log"

// UpdateFunc is the core logic of the pipeline. Based on a result, it returns
// the next set of tasks and a boolean flag indicating if the pipeline should terminate.
// This function is intended to be a closure that captures external state or struct method.
type UpdateFunc[Req, Res any] func(result Res) (newTasks []Req, done bool)

// ControlLoopV2 is a generic control engine for managing a pipeline's task flow.
// It supports 1:N fan-out, backpoutsure, and abrupt shutdown.
func ControlLoopV2[Req, Res any](
	update UpdateFunc[Req, Res],
	initialTasks []Req,
	reqCh chan<- Req,
	resCh <-chan Res,
	maxQueueSize int,
) {
	taskQueue := make([]Req, 0, maxQueueSize)
	taskQueue = append(taskQueue, initialTasks...)

Loop: // Label the for-loop to allow breaking out from within the select.
	for {
		// 1. Dynamically enable/disable the send channel.
		var sendCh chan<- Req
		var nextTask Req
		if len(taskQueue) > 0 {
			sendCh = reqCh
			nextTask = taskQueue[0]
		}

		// 2. Dynamically enable/disable the receive channel to apply backpoutsure.
		var recvCh <-chan Res
		if len(taskQueue) < maxQueueSize {
			recvCh = resCh
		}

		// 3. Main event selection loop.
		select {
		case res, ok := <-recvCh:
			if !ok {
				// The result channel was closed unexpectedly.
				log.Println("Result channel closed unexpectedly. Exiting loop.")
				break Loop
			}

			// Call the core pipeline logic.
			newTasks, done := update(res)

			// Check the termination condition for an abrupt shutdown.
			if done {
				log.Println("Termination condition met. Stopping ControlLoop.")
				break Loop
			}

			// Append new tasks only if the loop is not terminating.
			taskQueue = append(taskQueue, newTasks...)

		case sendCh <- nextTask:
			// Dispatch the task to a worker and remove it from the queue.
			taskQueue = taskQueue[1:]
		}
	}
}
