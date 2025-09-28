package bilevel

import (
	"context"

	"alphaprobe/orchestrator/internal/pipeline"
)

// --- Public API ---

type UpdateFunc[ORes, PReq any] = func(res ORes) (newTasks []PReq, done bool)
type ProposeFunc[PReq, PRes any] func(ctx context.Context, req PReq) PRes
type AdapterFunc[PRes, OReq any] = func(res PRes) ([]OReq, bool)
type ObserveFunc[OReq, ORes any] func(ctx context.Context, req OReq) ORes

// --- Orchestrator ---

type Orchestrator[PReq, PRes, OReq, ORes any] struct {
	updateFn           UpdateFunc[ORes, PReq]
	proposeFn          ProposeFunc[PReq, PRes]
	observeFn          ObserveFunc[OReq, ORes]
	proposeConcurrency int
	observeConcurrency int
	maxQueueSize       int
}

func NewOrchestrator[PReq, PRes, OReq, ORes any](
	updateFn UpdateFunc[ORes, PReq],
	proposeFn ProposeFunc[PReq, PRes],
	observeFn ObserveFunc[OReq, ORes],
	proposeConcurrency int,
	observeConcurrency int,
	maxQueueSize int,
) *Orchestrator[PReq, PRes, OReq, ORes] {
	return &Orchestrator[PReq, PRes, OReq, ORes]{
		updateFn:           updateFn,
		proposeFn:          proposeFn,
		observeFn:          observeFn,
		proposeConcurrency: proposeConcurrency,
		observeConcurrency: observeConcurrency,
		maxQueueSize:       maxQueueSize,
	}
}

func Run[PReq, PRes, ORes any](
	orchestrator *Orchestrator[PReq, PRes, PRes, ORes],
	ctx context.Context,
	initialTasks []PReq,
) {
	ctx, cancel := context.WithCancel(ctx)

	proposeReqCh := make(chan PReq, orchestrator.proposeConcurrency)
	proposeResCh := make(chan PRes, orchestrator.proposeConcurrency)
	observeResCh := make(chan ORes, orchestrator.observeConcurrency)

	// --- Launch Ring Pipeline ---
	ring := pipeline.NewRing(ctx)
	pipeline.GoWorkers(ring, orchestrator.proposeConcurrency, orchestrator.proposeFn, proposeReqCh, proposeResCh)
	pipeline.GoWorkers(ring, orchestrator.observeConcurrency, orchestrator.observeFn, proposeResCh, observeResCh)
	pipeline.GoControllerWithQueue(ring, orchestrator.updateFn, initialTasks, orchestrator.maxQueueSize, cancel, proposeReqCh, observeResCh)

	ring.Loop()
	ring.Wait()
}

func RunWithAdapter[PReq, PRes, OReq, ORes any](
	orchestrator *Orchestrator[PReq, PRes, OReq, ORes],
	ctx context.Context,
	initialTasks []PReq,
	adapterFn AdapterFunc[PRes, OReq],
) {
	ctx, cancel := context.WithCancel(ctx)

	proposeReqCh := make(chan PReq, orchestrator.proposeConcurrency)
	proposeResCh := make(chan PRes, orchestrator.proposeConcurrency)
	observeReqCh := make(chan OReq, orchestrator.observeConcurrency)
	observeResCh := make(chan ORes, orchestrator.observeConcurrency)

	// --- Launch Ring Pipeline ---
	ring := pipeline.NewRing(ctx)
	pipeline.GoWorkers(ring, orchestrator.proposeConcurrency, orchestrator.proposeFn, proposeReqCh, proposeResCh)
	pipeline.GoControllerWithQueue(ring, adapterFn, nil, orchestrator.maxQueueSize, cancel, observeReqCh, proposeResCh)
	pipeline.GoWorkers(ring, orchestrator.observeConcurrency, orchestrator.observeFn, observeReqCh, observeResCh)
	pipeline.GoControllerWithQueue(ring, orchestrator.updateFn, initialTasks, orchestrator.maxQueueSize, cancel, proposeReqCh, observeResCh)

	ring.Loop()
	ring.Wait()
}
