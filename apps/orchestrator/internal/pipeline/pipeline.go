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

func (r *Ring) Loop() {
	<-r.ctx.Done()
}

func (r *Ring) Wait() {
	r.wg.Wait()
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
