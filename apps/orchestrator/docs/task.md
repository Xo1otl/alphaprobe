# Go 1.25+ WaitGroup usage
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"internal/race"
	"internal/synctest"
	"sync/atomic"
	"unsafe"
)

// A WaitGroup is a counting semaphore typically used to wait
// for a group of goroutines or tasks to finish.
//
// Typically, a main goroutine will start tasks, each in a new
// goroutine, by calling [WaitGroup.Go] and then wait for all tasks to
// complete by calling [WaitGroup.Wait]. For example:
//
//	var wg sync.WaitGroup
//	wg.Go(task1)
//	wg.Go(task2)
//	wg.Wait()
//
// A WaitGroup may also be used for tracking tasks without using Go to
// start new goroutines by using [WaitGroup.Add] and [WaitGroup.Done].
//
// The previous example can be rewritten using explicitly created
// goroutines along with Add and Done:
//
//	var wg sync.WaitGroup
//	wg.Add(1)
//	go func() {
//		defer wg.Done()
//		task1()
//	}()
//	wg.Add(1)
//	go func() {
//		defer wg.Done()
//		task2()
//	}()
//	wg.Wait()
//
// This pattern is common in code that predates [WaitGroup.Go].
//
// A WaitGroup must not be copied after first use.
```

# `pipeline.go`
```go
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
```

# `runner.go`
```go
package bilevel

import (
	"alphaprobe/orchestrator/internal/pipeline"
	"context"
)

// --- Public API ---

type RunFunc[PReq any] func(ctx context.Context, initialTasks []PReq) error
type UpdateFunc[Q, E, D, PReq any] func(ctx context.Context, query Q, evidence E, data D) (newTasks []PReq, done bool)
type ProposeFunc[PReq any, POut any, D any] func(ctx context.Context, preq PReq) (pout POut, data D)
type ObserveFunc[Q any, E any] func(ctx context.Context, query Q) (evidence E)
type AdapterFunc[POut any, Q any, D any] func(in <-chan proposeRes[POut, D], out chan<- *observeReq[Q, D])
type FanOutFunc[POut any, Q any, D any] func(pout POut, data D) []Q

// --- Internal Data Structures ---

type proposeRes[POut any, D any] struct {
	POut POut
	Data D
}

type observeReq[Q any, D any] struct {
	Query Q
	Data  D
}

type observeRes[Q, E, D any] struct {
	Query    Q
	Evidence E
	Data     D
}

// --- Factories ---

func NewFanOutAdapter[POut any, Q any, D any](
	fanOut FanOutFunc[POut, Q, D],
) AdapterFunc[POut, Q, D] {
	return func(in <-chan proposeRes[POut, D], out chan<- *observeReq[Q, D]) {
		defer close(out)
		for pRes := range in {
			queries := fanOut(pRes.POut, pRes.Data)
			for _, q := range queries {
				out <- &observeReq[Q, D]{
					Query: q,
					Data:  pRes.Data,
				}
			}
		}
	}
}

func Run[PReq, Q, D, E any](
	updateFn UpdateFunc[Q, E, D, PReq],
	proposeFn ProposeFunc[PReq, Q, D],
	observeFn ObserveFunc[Q, E],
	proposeConcurrency int,
	observeConcurrency int,
	maxQueueSize int,
) RunFunc[PReq] {
	r := &simpleRunner[PReq, Q, D, E]{
		updateFn:           updateFn,
		proposeFn:          proposeFn,
		observeFn:          observeFn,
		proposeConcurrency: proposeConcurrency,
		observeConcurrency: observeConcurrency,
		maxQueueSize:       maxQueueSize,
	}
	return r.Run
}

func RunWithAdapter[PReq, POut, Q, D, E any](
	updateFn UpdateFunc[Q, E, D, PReq],
	proposeFn ProposeFunc[PReq, POut, D],
	adapterFn AdapterFunc[POut, Q, D],
	observeFn ObserveFunc[Q, E],
	proposeConcurrency int,
	observeConcurrency int,
	maxQueueSize int,
) RunFunc[PReq] {
	r := &adaptedRunner[PReq, POut, Q, D, E]{
		updateFn:           updateFn,
		proposeFn:          proposeFn,
		adapterFn:          adapterFn,
		observeFn:          observeFn,
		proposeConcurrency: proposeConcurrency,
		observeConcurrency: observeConcurrency,
		maxQueueSize:       maxQueueSize,
	}
	return r.Run
}

// --- Private Runner Implementations ---

type simpleRunner[PReq, Q, D, E any] struct {
	updateFn           UpdateFunc[Q, E, D, PReq]
	proposeFn          ProposeFunc[PReq, Q, D]
	observeFn          ObserveFunc[Q, E]
	proposeConcurrency int
	observeConcurrency int
	maxQueueSize       int
}

func (r *simpleRunner[PReq, Q, D, E]) Run(ctx context.Context, initialTasks []PReq) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	proposeReqCh := make(chan PReq, r.proposeConcurrency)
	observeReqCh := make(chan *observeReq[Q, D], r.observeConcurrency)
	observeResCh := make(chan *observeRes[Q, E, D], r.observeConcurrency)

	proposeTask := func(ctx context.Context, req PReq) *observeReq[Q, D] {
		q, data := r.proposeFn(ctx, req)
		return &observeReq[Q, D]{Query: q, Data: data}
	}

	observeTask := func(ctx context.Context, obsIn *observeReq[Q, D]) *observeRes[Q, E, D] {
		evidence := r.observeFn(ctx, obsIn.Query)
		return &observeRes[Q, E, D]{Query: obsIn.Query, Evidence: evidence, Data: obsIn.Data}
	}

	update := func(ctx context.Context, res *observeRes[Q, E, D]) ([]PReq, bool) {
		return r.updateFn(ctx, res.Query, res.Evidence, res.Data)
	}

	controller := pipeline.NewController[PReq, *observeRes[Q, E, D]](ctx, 2)
	pipeline.LaunchWorkers(controller, r.proposeConcurrency, proposeTask, proposeReqCh, observeReqCh, func() { close(observeReqCh) })
	pipeline.LaunchWorkers(controller, r.observeConcurrency, observeTask, observeReqCh, observeResCh, nil)

	defer func() {
		cancel()
		controller.Wait()
	}()

	return controller.Loop(update, initialTasks, proposeReqCh, observeResCh, r.maxQueueSize)
}

type adaptedRunner[PReq, POut, Q, D, E any] struct {
	updateFn           UpdateFunc[Q, E, D, PReq]
	proposeFn          ProposeFunc[PReq, POut, D]
	adapterFn          AdapterFunc[POut, Q, D]
	observeFn          ObserveFunc[Q, E]
	proposeConcurrency int
	observeConcurrency int
	maxQueueSize       int
}

func (r *adaptedRunner[PReq, POut, Q, D, E]) Run(ctx context.Context, initialTasks []PReq) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	proposeReqCh := make(chan PReq, r.proposeConcurrency)
	proposeResCh := make(chan proposeRes[POut, D], r.proposeConcurrency)
	observeReqCh := make(chan *observeReq[Q, D], r.observeConcurrency)
	observeResCh := make(chan *observeRes[Q, E, D], r.observeConcurrency)

	proposeTask := func(ctx context.Context, req PReq) proposeRes[POut, D] {
		pout, data := r.proposeFn(ctx, req)
		return proposeRes[POut, D]{POut: pout, Data: data}
	}

	observeTask := func(ctx context.Context, obsReq *observeReq[Q, D]) *observeRes[Q, E, D] {
		evidence := r.observeFn(ctx, obsReq.Query)
		return &observeRes[Q, E, D]{Query: obsReq.Query, Evidence: evidence, Data: obsReq.Data}
	}

	update := func(ctx context.Context, res *observeRes[Q, E, D]) ([]PReq, bool) {
		return r.updateFn(ctx, res.Query, res.Evidence, res.Data)
	}

	controller := pipeline.NewController[PReq, *observeRes[Q, E, D]](ctx, 2)
	pipeline.LaunchWorkers(controller, r.proposeConcurrency, proposeTask, proposeReqCh, proposeResCh, func() { close(proposeResCh) })
	pipeline.LaunchWorkers(controller, r.observeConcurrency, observeTask, observeReqCh, observeResCh, nil)
	go r.adapterFn(proposeResCh, observeReqCh)

	defer func() {
		cancel()
		controller.Wait()
	}()

	return controller.Loop(update, initialTasks, proposeReqCh, observeResCh, r.maxQueueSize)
}
```

# Your Task
pipelineの大幅な変更について考えている。
結果が来た時になにかする関数(onRes)と、入力パイプラインが空いている時になにかする関数(onReq)をDIする方が汎用性高くない？
