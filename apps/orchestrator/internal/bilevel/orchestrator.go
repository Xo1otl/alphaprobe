package bilevel

import (
	"context"

	"alphaprobe/orchestrator/internal/pipeline"
)

// --- Public API ---

type ProposeFunc[PReq, PRes any] func(ctx context.Context, req PReq) PRes
type ObserveFunc[OReq, ORes any] func(ctx context.Context, req OReq) ORes
type FanOutFunc[PRes, OReq any] func(res PRes) []OReq

// --- Orchestrator ---

type Orchestrator[PReq, PRes, OReq, ORes any] struct {
	proposeFn          ProposeFunc[PReq, PRes]
	observeFn          ObserveFunc[OReq, ORes]
	proposeConcurrency int
	observeConcurrency int
}

func NewOrchestrator[PReq, PRes, OReq, ORes any](
	proposeFn ProposeFunc[PReq, PRes],
	observeFn ObserveFunc[OReq, ORes],
	proposeConcurrency int,
	observeConcurrency int,
) *Orchestrator[PReq, PRes, OReq, ORes] {
	return &Orchestrator[PReq, PRes, OReq, ORes]{
		proposeFn:          proposeFn,
		observeFn:          observeFn,
		proposeConcurrency: proposeConcurrency,
		observeConcurrency: observeConcurrency,
	}
}

func Run[PReq, PRes, ORes any](
	orchestrator *Orchestrator[PReq, PRes, PRes, ORes],
	ctx context.Context,
	state State[PReq, ORes],
) {
	proposeReqCh := make(chan PReq, orchestrator.proposeConcurrency)
	proposeResCh := make(chan PRes, orchestrator.proposeConcurrency)
	observeResCh := make(chan ORes, orchestrator.observeConcurrency)

	ring := pipeline.NewRing(ctx)
	pipeline.GoWorkers(ring, orchestrator.proposeConcurrency, orchestrator.proposeFn, proposeReqCh, proposeResCh)
	pipeline.GoWorkers(ring, orchestrator.observeConcurrency, orchestrator.observeFn, proposeResCh, observeResCh)
	GoControllerWithState(ring, state, proposeReqCh, observeResCh)

	ring.Wait()
}

func RunWithFanOut[PReq, PRes, OReq, ORes any](
	orchestrator *Orchestrator[PReq, PRes, OReq, ORes],
	ctx context.Context,
	state State[PReq, ORes],
	fanOutFn FanOutFunc[PRes, OReq],
) {
	proposeReqCh := make(chan PReq, orchestrator.proposeConcurrency)
	proposeResCh := make(chan PRes, orchestrator.proposeConcurrency)
	observeReqCh := make(chan OReq, orchestrator.observeConcurrency)
	observeResCh := make(chan ORes, orchestrator.observeConcurrency)

	ring := pipeline.NewRing(ctx)
	pipeline.GoWorkers(ring, orchestrator.proposeConcurrency, orchestrator.proposeFn, proposeReqCh, proposeResCh)
	GoFanOutController(ring, fanOutFn, proposeResCh, observeReqCh)
	pipeline.GoWorkers(ring, orchestrator.observeConcurrency, orchestrator.observeFn, observeReqCh, observeResCh)
	GoControllerWithState(ring, state, proposeReqCh, observeResCh)

	ring.Wait()
}
