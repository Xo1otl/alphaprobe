package pipeline

import (
	"context"
	"sync"
)

type Ring struct {
	ctx context.Context
	wg  sync.WaitGroup
}

func NewRing(ctx context.Context) *Ring {
	return &Ring{ctx: ctx}
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

	r.wg.Go(func() {
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
	r.wg.Go(func() {
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
			case <-r.ctx.Done():
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

func GoFanOutController[Res, Req any](
	r *Ring,
	fanOutFn func(res Res) []Req,
	resCh <-chan Res,
	reqCh chan<- Req,
) {
	r.wg.Go(func() {
		defer close(reqCh)
		for {
			select {
			case <-r.ctx.Done():
				return
			case res, ok := <-resCh:
				if !ok {
					return
				}
				reqs := fanOutFn(res)
				for _, req := range reqs {
					select {
					case <-r.ctx.Done():
						return
					case reqCh <- req:
					}
				}
			}
		}
	})
}
