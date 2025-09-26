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
) {
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

		// FIXME: Setting a nil value here will likely cause a deadlock. However, a bursting taskQueue indicates a logical flaw in the algorithm, so this should be treated and handled as an error.
		var recvCh <-chan Res
		if len(taskQueue) < maxQueueSize {
			recvCh = resCh
		}

		select {
		case <-c.ctx.Done():
			break Loop
		case res, ok := <-recvCh:
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
	"context"

	"alphaprobe/orchestrator/internal/pipeline"
)

// --- Public API ---

type RunFunc[PReq any] func(ctx context.Context, initialTasks []PReq)
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

func (r *simpleRunner[PReq, Q, D, E]) Run(ctx context.Context, initialTasks []PReq) {
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
	controller.Loop(update, initialTasks, proposeReqCh, observeResCh, r.maxQueueSize)

	cancel()
	controller.Wait()
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

func (r *adaptedRunner[PReq, POut, Q, D, E]) Run(ctx context.Context, initialTasks []PReq) {
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
	controller.Loop(update, initialTasks, proposeReqCh, observeResCh, r.maxQueueSize)

	cancel()
	controller.Wait()
}
```

# `llmsr.go`
```go
package llmsr

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
)

type ProgramSkeleton = string
type Score = float64
type Program struct {
	Skeleton ProgramSkeleton
	Score    Score
}

type State struct {
	Programs         []Program
	EvaluationsCount int
	MaxEvaluations   int
	BestScore        Score
	PendingParents   map[string]bool
}

type Metadata struct {
	ParentSkeletons []ProgramSkeleton
}

func NewState(initialSkeleton ProgramSkeleton, maxEvaluations int) *State {
	initialProgram := Program{
		Skeleton: initialSkeleton,
		Score:    1e9, // A very large number representing an unevaluated score.
	}
	return &State{
		Programs:         []Program{initialProgram},
		EvaluationsCount: 0,
		MaxEvaluations:   maxEvaluations,
		BestScore:        1e9,
		PendingParents:   make(map[string]bool),
	}
}

func (s *State) GetInitialTask() [][]Program {
	if len(s.Programs) != 1 || s.EvaluationsCount != 0 {
		return nil // Should only be called at the start.
	}

	initialProgram := s.Programs[0]
	s.PendingParents[initialProgram.Skeleton] = true
	nextTask := []Program{initialProgram, initialProgram}
	return [][]Program{nextTask}
}

func (s *State) Update(ctx context.Context, skeleton ProgramSkeleton, score Score, metadata Metadata) ([][]Program, bool) {
	s.EvaluationsCount++
	newProgram := Program{
		Skeleton: skeleton,
		Score:    score,
	}
	s.Programs = append(s.Programs, newProgram)

	const maxPopulation = 10
	if len(s.Programs) > maxPopulation {
		sort.Slice(s.Programs, func(i, j int) bool {
			return s.Programs[i].Score < s.Programs[j].Score
		})
		s.Programs = s.Programs[:maxPopulation]
	}

	if score < s.BestScore {
		s.BestScore = score
		fmt.Printf("New best score: %f (Evaluation #%d)\n", s.BestScore, s.EvaluationsCount)
	}

	for _, p := range metadata.ParentSkeletons {
		delete(s.PendingParents, p)
	}

	if s.EvaluationsCount >= s.MaxEvaluations {
		return nil, true
	}

	if len(s.PendingParents) > 0 {
		return nil, false
	}

	availablePrograms := make([]Program, 0, len(s.Programs))
	for _, p := range s.Programs {
		if !s.PendingParents[p.Skeleton] {
			availablePrograms = append(availablePrograms, p)
		}
	}

	if len(availablePrograms) < 2 {
		return nil, true
	}

	rand.Shuffle(len(availablePrograms), func(i, j int) {
		availablePrograms[i], availablePrograms[j] = availablePrograms[j], availablePrograms[i]
	})
	parent1 := availablePrograms[0]
	parent2 := availablePrograms[1]
	s.PendingParents[parent1.Skeleton] = true
	s.PendingParents[parent2.Skeleton] = true

	nextTask := []Program{parent1, parent2}
	return [][]Program{nextTask}, false
}

func Propose(ctx context.Context, parents []Program) ([]ProgramSkeleton, Metadata) {
	batchSize := rand.Intn(4) + 1
	newSkeletons := make([]ProgramSkeleton, 0, batchSize)
	for range batchSize {
		newSkeleton := fmt.Sprintf("%s\n# Mutated %d", parents[0].Skeleton, rand.Intn(100))
		newSkeletons = append(newSkeletons, newSkeleton)
	}

	parentSkeletons := make([]ProgramSkeleton, len(parents))
	for i, p := range parents {
		parentSkeletons[i] = p.Skeleton
	}

	metadata := Metadata{
		ParentSkeletons: parentSkeletons,
	}
	return newSkeletons, metadata
}

func FanOut(pout []ProgramSkeleton, data Metadata) []ProgramSkeleton {
	return pout
}

func Observe(ctx context.Context, skeleton ProgramSkeleton) Score {
	return rand.Float64()
}
```
# `llmsr_test.go`
```go
package llmsr_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"alphaprobe/orchestrator/internal/bilevel"
	"alphaprobe/orchestrator/internal/llmsr"
)

func TestLLMSRWithBilevelRunner(t *testing.T) {
	const (
		maxEvaluations     = 100
		proposeConcurrency = 2
		observeConcurrency = 3
		maxQueueSize       = 2
		testTimeout        = 5 * time.Second
	)

	doneCh := make(chan error, 1) // Buffered channel to prevent goroutine leak on timeout
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	go func() {
		state := llmsr.NewState("def initial_program(x): return x", maxEvaluations)

		adapter := bilevel.NewFanOutAdapter(llmsr.FanOut)

		run := bilevel.RunWithAdapter(
			state.Update,
			llmsr.Propose,
			adapter,
			llmsr.Observe,
			proposeConcurrency,
			observeConcurrency,
			maxQueueSize,
		)

		fmt.Println("--- Starting Mock LLMSR Search with adapted bilevel Runner ---")
		initialTasks := state.GetInitialTask()
		run(ctx, initialTasks)
		fmt.Println("--- Mock LLMSR Search Finished ---")

		fmt.Printf("Final best score: %f\n", state.BestScore)
		fmt.Printf("Total evaluations: %d\n", state.EvaluationsCount)

		if state.EvaluationsCount < maxEvaluations {
			doneCh <- fmt.Errorf("Expected at least %d evaluations, but got %d", maxEvaluations, state.EvaluationsCount)
			return
		}
		if state.BestScore > 1.0 {
			doneCh <- fmt.Errorf("Expected best score to be less than 1.0, but got %f", state.BestScore)
			return
		}
		if len(state.Programs) > 10 {
			doneCh <- fmt.Errorf("Expected population size to be at most 10, but got %d", len(state.Programs))
			return
		}
		close(doneCh)
	}()

	select {
	case err := <-doneCh:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(testTimeout):
		t.Fatal("Test timed out after 5 seconds (potential deadlock)")
	}
}
```

# Your Task
エラーハンドリングモデルをどうするか考えている
1. pipelineの時点からtaskFnがエラーを返せるようにすべて書き直す、
2. pipseine.goはqueueSizeのエラーだけを処理、runner.goがtaskFnのwrapper部分で、chiのAppHandlerで見られるような作法でエラーハンドリングを一元化すればよくね？
3. もしかしてpipeline.goもrunner.goもなんも変更する必要がなく、llmsr_test.goでchiのお作法を行えばよくね？
どうするのがいいんだろう？
