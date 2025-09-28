package bilevel

import (
	"alphaprobe/orchestrator/internal/pipeline"
	"log"
)

type State[Req, Res any] interface {
	onResult(res Res) (done bool)
	onNextTask() (task Req, ok bool)
	onTaskSent()
}

func GoControllerWithState[Req, Res any](
	r *pipeline.Ring,
	state State[Req, Res],
	initialTasks []Req,
	cancel func(),
	reqCh chan<- Req,
	resCh <-chan Res,
) {
	// TODO: これ実装したい、initialTasksの投入処理やcancelなどが必要か
	// GoControllerはdoneでreturnするので、returnしてからcancelを呼ぶのが正しい終了方法なのでは？
}

func GoControllerWithQueue[Req, Res any](
	r *pipeline.Ring,
	onResult func(res Res) (newTasks []Req, done bool),
	initialTasks []Req, // TODO: この関数はadapterとしてのcontrollerのためなので、initialTasksは不要にしたい
	maxQueueSize int,
	cancel func(), // cancelもadapterでは不要でいい
	reqCh chan<- Req,
	resCh <-chan Res,
) {
	// TODO: 現在state controllerにするために複雑な処理が多いが、できるだけシンプルにしたい
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

	pipeline.GoController(r, onResultWrapper, onNextTask, onTaskSent, resCh, reqCh)
}
