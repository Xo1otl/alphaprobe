```go
// Package bilevel provides a framework for orchestrating two-level concurrent proposal and observation tasks.
// It is designed with the assumption that the Propose and Observe functions are I/O-bound, such as performing network requests or GPU operations.
//
// This package delegates error handling to the implementer. Since the
// generic type parameters (PReq, PRes, etc.) are of type `any`, a common
// pattern is to embed an `error` field within the structs used for these types.
//
// The `ProposeFunc` or `ObserveFunc` implementations can then populate this `Err`
// field upon failure. Subsequently, the `State.Update` method can inspect
// this field. Based on the error, the implementation can choose to either
// record the error in its state or call the context's `cancel` function to
// terminate all ongoing goroutines gracefully.
package bilevel

import "context"

// --- Public API ---

type ProposeFunc[PReq, PRes any] func(ctx context.Context, req PReq) PRes
type ObserveFunc[OReq, ORes any] func(ctx context.Context, req OReq) ORes
type State[PReq, ORes any] interface {
	Update(res ORes) (done bool)
	NewRequest() (req PReq, ok bool)
}
type Adapter[PRes, OReq any] interface {
	Recv(res PRes)
	Next() (req OReq, ok bool)
}

// --- Examples ---

/*
ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
state := NewState()
adapter := NewAdapter()
orchestrator := bilevel.NewOrchestrator(
	Propose,
	Observe,
	proposeConcurrency,
	observeConcurrency,
)
bilevel.RunWithAdapter(orchestrator, ctx, state, adapter)
*/

/*
ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
state := NewState()
orchestrator := bilevel.NewOrchestrator(
	Propose,
	Observe,
	proposeConcurrency,
	observeConcurrency,
)
bilevel.Run(orchestrator, ctx, state)
*/
```