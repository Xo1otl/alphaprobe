package pipeline

import (
	"context"
	"log"
	"sync"
)

type UpdateFunc[Req, Res any] func(ctx context.Context, result Res) (newTasks []Req, done bool)

func LaunchWorkers[Req, Res any](
	ctx context.Context,
	wg *sync.WaitGroup,
	numWorkers int,
	taskFn func(ctx context.Context, req Req) Res,
	reqCh <-chan Req,
	resCh chan<- Res,
) {
	for range numWorkers {
		wg.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				case req, ok := <-reqCh:
					if !ok {
						return
					}
					select {
					case <-ctx.Done():
						return
					case resCh <- taskFn(ctx, req):
					}
				}
			}
		})
	}
}

func Loop[Req, Res any](
	ctx context.Context,
	update UpdateFunc[Req, Res],
	initialTasks []Req,
	reqCh chan<- Req,
	resCh <-chan Res,
	maxQueueSize int,
) {
	defer close(reqCh)
	taskQueue := make([]Req, 0, maxQueueSize)
	taskQueue = append(taskQueue, initialTasks...)

Loop:
	for {
		var sendCh chan<- Req
		var nextTask Req
		if len(taskQueue) > 0 {
			sendCh = reqCh
			nextTask = taskQueue[0]
		}

		var recvCh <-chan Res
		if len(taskQueue) < maxQueueSize {
			recvCh = resCh
		}

		select {
		case <-ctx.Done():
			break Loop
		case res, ok := <-recvCh:
			if !ok {
				break Loop
			}

			newTasks, done := update(ctx, res)
			if done {
				break Loop
			}

			taskQueue = append(taskQueue, newTasks...)

		case sendCh <- nextTask:
			taskQueue = taskQueue[1:]
		}
	}
	log.Println("[Pipeline.Loop] END")
}
