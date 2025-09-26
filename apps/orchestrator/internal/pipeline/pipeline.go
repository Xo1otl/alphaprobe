package pipeline

import (
	"context"
	"fmt"
	"log"
	"sync"
)

type UpdateFunc[Req, Res any] func(ctx context.Context, result Res) (newTasks []Req, done bool)

type stage struct {
	wg      sync.WaitGroup
	closeFn func()
}

type workerManager interface {
	addStage(closeFn func()) *sync.WaitGroup
	getContext() context.Context
}

type Controller[Req, Res any] struct {
	ctx    context.Context
	stages []stage
	idx    int
}

func NewController[Req, Res any](ctx context.Context, numStages int) *Controller[Req, Res] {
	return &Controller[Req, Res]{
		ctx:    ctx,
		stages: make([]stage, numStages),
	}
}

func (c *Controller[_, _]) addStage(closeFn func()) *sync.WaitGroup {
	c.stages[c.idx] = stage{closeFn: closeFn}
	wg := &c.stages[c.idx].wg
	c.idx++
	return wg
}

func (c *Controller[_, _]) getContext() context.Context {
	return c.ctx
}

func LaunchWorkers[Req, Res any](
	c workerManager,
	numWorkers int,
	taskFn func(ctx context.Context, req Req) Res,
	reqCh <-chan Req,
	resCh chan<- Res,
	closeResCh func(),
) {
	wg := c.addStage(closeResCh)
	ctx := c.getContext()

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

func (c *Controller[Req, Res]) Loop(
	update UpdateFunc[Req, Res],
	initialTasks []Req,
	reqCh chan<- Req,
	resCh <-chan Res,
	maxQueueSize int,
) error {
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

		if len(taskQueue) > maxQueueSize {
			return fmt.Errorf("task queue overflow: current size (%d) exceeds max size (%d)", len(taskQueue), maxQueueSize)
		}

		select {
		case <-c.ctx.Done():
			break Loop
		case res, ok := <-resCh:
			if !ok {
				break Loop
			}

			newTasks, done := update(c.ctx, res)
			if done {
				break Loop
			}

			taskQueue = append(taskQueue, newTasks...)

		case sendCh <- nextTask:
			taskQueue = taskQueue[1:]
		}
	}
	log.Println("[Pipeline.Loop] END")
	return nil
}

func (c *Controller[_, _]) Wait() {
	for i := range c.stages {
		s := &c.stages[i]
		s.wg.Wait()
		if s.closeFn != nil {
			s.closeFn()
		}
	}
}
