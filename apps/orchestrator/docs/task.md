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
* Do not modify the `pipeline` and `bilevel` packages.
* Adhere to the `bilevel` package contract. `State` and `Adapter` methods do not need to be thread-safe, as they are called from a single goroutine. However, you must manage their state carefully, as the method call order (e.g., `Update` vs. `Next`) is unpredictable.

# Your Task
@docs/architecture.md は古い設計案に基づいて書かれており、実際のCommand Serviceの実装と異なっています.
完全版のcommand serviceは、@apps/orchestrator/internal/bilevel/orchestrator.go のRunWithAdapterの仕様に基づいて設計されます.

* task1 pool -> propose
* task2 pool -> observe
* control loop -> orchestrator
* aggregator -> adapter
* propagate,shouldTerminate,dispatch -> Update,NewRequest
* repositoryやmemento patternは維持する

bilevel packageの仕様を元にして、Command ServiceのC4 Modelを書き直してほしい.
