package pipeline

import (
	"log"
	"sync"
)

type UpdateFunc[Req, Res any] func(result Res) (newTasks []Req, done bool)

func ControlLoop[Req, Res any](
	update UpdateFunc[Req, Res],
	initialTasks []Req,
	reqCh chan<- Req,
	resCh <-chan Res,
	maxQueueSize int,
) {
	defer func() {
		log.Println("[ControlLoop] Closing request channel")
		close(reqCh)
	}()

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

		// log.Printf("[ControlLoop] START: Queue=%d, sendCh_nil=%v, recvCh_nil=%v", len(taskQueue), sendCh == nil, recvCh == nil)

		select {
		case res, ok := <-recvCh:
			if !ok {
				// log.Println("[ControlLoop] EXIT: Result channel closed")
				break Loop
			}
			// log.Printf("[ControlLoop] RECEIVED: Result=%v", res)

			newTasks, done := update(res)
			if done {
				// log.Println("[ControlLoop] EXIT: Update function returned done")
				break Loop
			}

			taskQueue = append(taskQueue, newTasks...)
			// log.Printf("[ControlLoop] UPDATED: Queue=%d", len(taskQueue))

		case sendCh <- nextTask:
			taskQueue = taskQueue[1:]
			// log.Printf("[ControlLoop] SENT: Queue=%d", len(taskQueue))
		}
	}
	log.Println("[ControlLoop] END")
}

func WorkerPool[Req, Res any](
	numWorkers int,
	taskFn func(Req) Res,
	reqCh <-chan Req,
	resCh chan<- Res,
	wg *sync.WaitGroup,
) {
	for range numWorkers {
		wg.Go(func() {
			for req := range reqCh {
				resCh <- taskFn(req)
			}
		})
	}
}
