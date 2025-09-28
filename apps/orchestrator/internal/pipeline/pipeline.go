package pipeline

import (
	"context"
	"sync"
)

type Ring struct {
	Ctx context.Context
	Wg  sync.WaitGroup
}

func NewRing(ctx context.Context) *Ring {
	return &Ring{Ctx: ctx}
}

func (r *Ring) Wait() {
	r.Wg.Wait()
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
		r.Wg.Go(func() {
			defer stageWg.Done()
			for {
				select {
				case <-r.Ctx.Done():
					return
				case req, ok := <-reqCh:
					if !ok {
						return
					}
					result := taskFn(r.Ctx, req)
					select {
					case <-r.Ctx.Done():
						return
					case resCh <- result:
					}
				}
			}
		})
	}

	r.Wg.Go(func() {
		stageWg.Wait()
		close(resCh)
	})
}

func GoController[Req, Res any](
	r *Ring,
	onResult func(res Res) (done bool),
	onNextTask func() (task Req, ok bool),
	onTaskSent func(task Req),
	resCh <-chan Res,
	reqCh chan<- Req,
) {
	r.Wg.Go(func() {
		defer close(reqCh)

		var nextTask Req
		var hasTask bool
		var sendCh chan<- Req

		nextTask, hasTask = onNextTask()
		if hasTask {
			sendCh = reqCh
		}

		for {
			select {
			case <-r.Ctx.Done():
				return

			case res, ok := <-resCh:
				if !ok {
					return
				}
				if onResult(res) {
					return
				}
				nextTask, hasTask = onNextTask()
				if hasTask {
					sendCh = reqCh
				} else {
					sendCh = nil
				}

			case sendCh <- nextTask:
				onTaskSent(nextTask)
				nextTask, hasTask = onNextTask()
				if hasTask {
					sendCh = reqCh
				} else {
					sendCh = nil
				}
			}
		}
	})
}
