package bilevel

import (
	"alphaprobe/orchestrator/internal/pipeline"
	"log"
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

func GoControllerWithQueue[Req, Res any](
	r *pipeline.Ring,
	onResult func(res Res) (newTasks []Req, done bool),
	initialTasks []Req, // TODO: この関数はadapterとしてのcontrollerのためなので、initialTasksは不要にしたい
	maxQueueSize int,
	reqCh chan<- Req,
	resCh <-chan Res,
) {
	// TODO: 現在state controllerにするために複雑な処理が多いが、できるだけシンプルにしたい
	taskQueue := make([]Req, 0, maxQueueSize)
	taskQueue = append(taskQueue, initialTasks...)

	onResultWrapper := func(res Res) (done bool) {
		newTasks, done := onResult(res)
		if done {
			// cancel()
			return true
		}
		taskQueue = append(taskQueue, newTasks...)
		if len(taskQueue) > maxQueueSize {
			// TODO: better error handling
			log.Printf("task queue overflow")
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

	onTaskSent := func(task Req) {
		taskQueue = taskQueue[1:]
	}

	pipeline.GoController(r, onResultWrapper, onNextTask, onTaskSent, resCh, reqCh)
}
