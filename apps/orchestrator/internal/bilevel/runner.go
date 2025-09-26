package bilevel

import (
	"context"
	"fmt"

	"alphaprobe/orchestrator/internal/pipeline"
)

// --- Public API ---

type UpdateFunc[ObsRes, PReq any] func(ctx context.Context, res ObsRes) (newTasks []PReq, done bool)
type ProposeFunc[PReq, PRes any] func(ctx context.Context, req PReq) PRes
type ExpandFunc[PRes, OReq any] func(ctx context.Context, res PRes) (newTasks []OReq, done bool)
type ObserveFunc[OReq, ORes any] func(ctx context.Context, req OReq) ORes

// --- Runner ---

type Runner[PReq, PRes, OReq, ORes any] struct {
	updateFn           UpdateFunc[ORes, PReq]
	proposeFn          ProposeFunc[PReq, PRes]
	expandFn           ExpandFunc[PRes, OReq]
	observeFn          ObserveFunc[OReq, ORes]
	proposeConcurrency int
	observeConcurrency int
	maxQueueSize       int
}

func NewRunner[PReq, PRes, OReq, ORes any](
	updateFn UpdateFunc[ORes, PReq],
	proposeFn ProposeFunc[PReq, PRes],
	expandFn ExpandFunc[PRes, OReq],
	observeFn ObserveFunc[OReq, ORes],
	proposeConcurrency int,
	observeConcurrency int,
	maxQueueSize int,
) *Runner[PReq, PRes, OReq, ORes] {
	return &Runner[PReq, PRes, OReq, ORes]{
		updateFn:           updateFn,
		proposeFn:          proposeFn,
		expandFn:           expandFn,
		observeFn:          observeFn,
		proposeConcurrency: proposeConcurrency,
		observeConcurrency: observeConcurrency,
		maxQueueSize:       maxQueueSize,
	}
}

func (r *Runner[PReq, PRes, OReq, ORes]) Run(ctx context.Context, initialTasks []PReq) error {
	ctx, cancel := context.WithCancel(ctx)
	ring := pipeline.NewRing(ctx)

	proposeReqQueue := make([]PReq, 0, r.maxQueueSize)
	proposeReqQueue = append(proposeReqQueue, initialTasks...)

	observeReqQueue := make([]OReq, 0, r.maxQueueSize)

	proposeReqCh := make(chan PReq, r.proposeConcurrency)
	proposeResCh := make(chan PRes, r.proposeConcurrency)
	observeReqCh := make(chan OReq, r.observeConcurrency)
	observeResCh := make(chan ORes, r.observeConcurrency)

	// --- State Controller Callbacks ---
	onObserveResult := func(res ORes) (done bool) {
		newTasks, done := r.updateFn(ctx, res)
		if done {
			cancel()
			return true
		}
		proposeReqQueue = append(proposeReqQueue, newTasks...)
		if len(proposeReqQueue) > r.maxQueueSize {
			fmt.Printf("propose task queue overflow\n")
			cancel()
			return true
		}
		return false
	}
	onNextProposeTask := func() (task PReq, ok bool) {
		if len(proposeReqQueue) == 0 {
			return task, false
		}
		return proposeReqQueue[0], true
	}
	onProposeTaskSent := func() {
		proposeReqQueue = proposeReqQueue[1:]
	}

	// --- Adapter Controller Callbacks ---
	onProposeResult := func(res PRes) (done bool) {
		newTasks, done := r.expandFn(ctx, res)
		if done {
			cancel()
			return true
		}
		observeReqQueue = append(observeReqQueue, newTasks...)
		if len(observeReqQueue) > r.maxQueueSize {
			fmt.Printf("observe task queue overflow\n")
			cancel()
			return true
		}
		return false
	}
	onNextObserveTask := func() (task OReq, ok bool) {
		if len(observeReqQueue) == 0 {
			return task, false
		}
		return observeReqQueue[0], true
	}
	onObserveTaskSent := func() {
		observeReqQueue = observeReqQueue[1:]
	}

	// --- Launch Pipeline ---
	pipeline.GoWorkers(ring, r.proposeConcurrency, r.proposeFn, proposeReqCh, proposeResCh)
	pipeline.GoController(ring, onProposeResult, onNextObserveTask, onObserveTaskSent, proposeResCh, observeReqCh)
	pipeline.GoWorkers(ring, r.observeConcurrency, r.observeFn, observeReqCh, observeResCh)
	pipeline.GoController(ring, onObserveResult, onNextProposeTask, onProposeTaskSent, observeResCh, proposeReqCh)

	ring.Loop()
	ring.Wait()

	return nil
}
