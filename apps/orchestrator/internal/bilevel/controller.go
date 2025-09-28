package bilevel

import "alphaprobe/orchestrator/internal/pipeline"

type State[Req, Res any] interface {
	Update(res Res) (done bool)
	Next() (task Req, ok bool)
	Sent(task Req)
}

func GoFanOutController[Res, Req any](
	r *pipeline.Ring,
	fanOutFn func(res Res) []Req,
	resCh <-chan Res,
	reqCh chan<- Req,
) {
	r.Wg.Go(func() {
		defer close(reqCh)
		for {
			select {
			case <-r.Ctx.Done():
				return
			case res, ok := <-resCh:
				if !ok {
					return
				}
				reqs := fanOutFn(res)
				for _, req := range reqs {
					select {
					case <-r.Ctx.Done():
						return
					case reqCh <- req:
					}
				}
			}
		}
	})
}

func GoControllerWithState[Req, Res any](
	r *pipeline.Ring,
	state State[Req, Res],
	reqCh chan<- Req,
	resCh <-chan Res,
) {
	pipeline.GoController(r, state.Update, state.Next, state.Sent, resCh, reqCh)
}
