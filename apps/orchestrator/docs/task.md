# package bilevel
```go
// Package bilevel provides a framework for orchestrating two-level concurrent proposal and observation tasks.
// It is designed with the assumption that the Propose and Observe functions are I/O-bound,
// such as performing network requests or GPU operations.
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

# Rules
* Do not modify the pipeline or bilevel packages.
* Your implementations must adhere to the contracts of the bilevel package. Note that State and Adapter methods are guaranteed to be called from a single goroutine, so they do not need to be internally thread-safe. However, you must carefully manage their internal state, as the order of method calls (e.g., Update vs. Next) is not at all guaranteed to be alternating.

# Your Task
llmsr_test.goとstate.goの実装を読んでください。そして、これがREADME.mdのロジックを正確に再現しているか、厳密に検証してください。
