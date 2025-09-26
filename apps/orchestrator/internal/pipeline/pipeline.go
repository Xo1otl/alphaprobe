package pipeline

import (
	"context"
	"log"
	"sync"
)

type Ring struct {
	ctx context.Context
	wg  sync.WaitGroup
}

func NewRing(ctx context.Context) *Ring {
	return &Ring{ctx: ctx}
}

func (r *Ring) Loop() {
	<-r.ctx.Done()
}

func (r *Ring) Wait() {
	r.wg.Wait()
}

func GoWorkers[Req, Res any](
	r *Ring,
	concurrency int,
	taskFn func(ctx context.Context, req Req) Res,
	reqCh <-chan Req,
	resCh chan<- Res,
) {
	var stageWg sync.WaitGroup
	stageWg.Add(concurrency)

	for range concurrency {
		r.wg.Go(func() {
			defer stageWg.Done()
			for {
				select {
				case <-r.ctx.Done():
					return
				case req, ok := <-reqCh:
					if !ok {
						return
					}
					result := taskFn(r.ctx, req)
					select {
					case <-r.ctx.Done():
						return
					case resCh <- result:
					}
				}
			}
		})
	}

	go func() {
		stageWg.Wait()
		close(resCh)
	}()
}

func GoController[Req, Res any](
	r *Ring,
	onResult func(res Res) (done bool),
	onNextTask func() (task Req, ok bool),
	onTaskSent func(),
	resCh <-chan Res,
	reqCh chan<- Req,
) {
	r.wg.Go(func() {
		defer close(reqCh)
		for {
			nextTask, hasTask := onNextTask()

			var sendCh chan<- Req
			if hasTask {
				sendCh = reqCh
			}

			select {
			case <-r.ctx.Done():
				return
			case res, ok := <-resCh:
				if !ok {
					return
				}
				if onResult(res) {
					return
				}
			case sendCh <- nextTask:
				onTaskSent()
			}
		}
	})
}

// --- Stateful Controller ---

func GoStatefulController[Req, Res any](
	r *Ring,
	onResult func(res Res) (newTasks []Req, done bool),
	initialTasks []Req,
	maxQueueSize int,
	cancel func(),
	reqCh chan<- Req,
	resCh <-chan Res,
) {
	taskQueue := make([]Req, 0, maxQueueSize)
	taskQueue = append(taskQueue, initialTasks...)

	onResultWrapper := func(res Res) (done bool) {
		newTasks, done := onResult(res)
		if done {
			cancel()
			return true
		}
		taskQueue = append(taskQueue, newTasks...)
		if len(taskQueue) > maxQueueSize {
			// TODO: better error handling
			log.Printf("task queue overflow")
			cancel()
			return true
		}
		return false
	}

	onNextTask := func() (task Req, ok bool) {
		if len(taskQueue) == 0 {
			return task, false
		}
		return taskQueue[0], true
	}

	onTaskSent := func() {
		taskQueue = taskQueue[1:]
	}

	GoController(r, onResultWrapper, onNextTask, onTaskSent, resCh, reqCh)
}
