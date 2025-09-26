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

// --- Runner ---

type Runner[PReq, PRes, OReq, ORes any] struct {
	updateFn           UpdateFunc[ORes, PReq]
	proposeFn          ProposeFunc[PReq, PRes]
	adapterFn          AdapterFunc[PRes, OReq]
	observeFn          ObserveFunc[OReq, ORes]
	proposeConcurrency int
	observeConcurrency int
	maxQueueSize       int
}

func NewRunner[PReq, PRes, OReq, ORes any](
	updateFn UpdateFunc[ORes, PReq],
	proposeFn ProposeFunc[PReq, PRes],
	adapterFn AdapterFunc[PRes, OReq],
	observeFn ObserveFunc[OReq, ORes],
	proposeConcurrency int,
	observeConcurrency int,
	maxQueueSize int,
) *Runner[PReq, PRes, OReq, ORes] {
	return &Runner[PReq, PRes, OReq, ORes]{
		updateFn:           updateFn,
		proposeFn:          proposeFn,
		adapterFn:          adapterFn,
		observeFn:          observeFn,
		proposeConcurrency: proposeConcurrency,
		observeConcurrency: observeConcurrency,
		maxQueueSize:       maxQueueSize,
	}
}

func (r *Runner[PReq, PRes, OReq, ORes]) Run(ctx context.Context, initialTasks []PReq) {
	ctx, cancel := context.WithCancel(ctx)

	proposeReqCh := make(chan PReq, r.proposeConcurrency)
	proposeResCh := make(chan PRes, r.proposeConcurrency)
	observeReqCh := make(chan OReq, r.observeConcurrency)
	observeResCh := make(chan ORes, r.observeConcurrency)

	// --- Launch Ring Pipeline ---
	ring := pipeline.NewRing(ctx)
	pipeline.GoWorkers(ring, r.proposeConcurrency, r.proposeFn, proposeReqCh, proposeResCh)
	pipeline.GoStatefulController(ring, r.adapterFn, nil, r.maxQueueSize, cancel, observeReqCh, proposeResCh)
	pipeline.GoWorkers(ring, r.observeConcurrency, r.observeFn, observeReqCh, observeResCh)
	pipeline.GoStatefulController(ring, r.updateFn, initialTasks, r.maxQueueSize, cancel, proposeReqCh, observeResCh)

	ring.Loop()
	ring.Wait()
}
