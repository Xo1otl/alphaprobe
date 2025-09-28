package bilevel

import (
	"alphaprobe/orchestrator/internal/pipeline"
)

type State[Req, Res any] interface {
	HandleResult(res Res) (done bool)
	NextTask() (task Req, ok bool)
	TaskSent(task Req)
}

func GoControllerWithState[Req, Res any](
	r *pipeline.Ring,
	state State[Req, Res],
	reqCh chan<- Req,
	resCh <-chan Res,
) {
	pipeline.GoController(r, state.HandleResult, state.NextTask, state.TaskSent, resCh, reqCh)
}
