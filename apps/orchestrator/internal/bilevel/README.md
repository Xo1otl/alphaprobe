# Introduction

The `bilevel` package provides a generic, concurrent framework for orchestrating two-level computational pipelines, commonly seen in producer-consumer patterns like propose-and-observe or search-and-evaluate workflows. It is designed for scenarios where the pipeline stages may be I/O-bound, such as making network requests or performing GPU computations.

# Overview

The system is built around a central **Orchestrator** that manages two concurrent stages of workers: a **Propose** stage and an **Observe** stage. The flow of data and control is managed by a central **State** object, which issues new requests and processes results. The package offers two primary execution models: **Run**, for direct propose-observe pipelines, and **RunWithAdapter**, which introduces an **Adapter** to handle transformations between the propose and observe stages. **Error Handling** is managed asynchronously via a dedicated channel.

# Orchestrator

The `Orchestrator` is the core component that executes the bilevel pipeline. It is configured with `ProposeFunc` and `ObserveFunc` implementations, along with their respective concurrency levels.

# State

The `State` interface is the control center of the pipeline. It is responsible for:
1.  Issuing new proposals (`Issue`).
2.  Processing observation results (`Update`).
3.  Signaling when the entire process is complete.

# Error Handling

The `bilevel` package delegates error handling to the caller. The `State.Update` and `State.Issue` methods can return errors. The `Run` and `RunWithAdapter` functions accept an `error` channel (`errCh`) as an argument.

When a method on the `State` object returns an error, the orchestrator sends this error to `errCh` without interrupting the pipeline. It is the caller's responsibility to listen on this channel and implement the desired error-handling logic, such as canceling the context to terminate all goroutines gracefully.

# Public API

```go
// ProposeFunc defines the signature for the first stage of the pipeline.
type ProposeFunc[PReq, PRes any] func(ctx context.Context, req PReq) PRes

// ObserveFunc defines the signature for the second stage of the pipeline.
type ObserveFunc[OReq, ORes any] func(ctx context.Context, req OReq) ORes

// State defines the control interface for the pipeline.
type State[PReq, ORes any] interface {
	Update(res ORes) (done bool, err error)
	Issue() (req PReq, ok bool, err error)
}

// Adapter defines an interface for transforming data between the Propose and Observe stages.
type Adapter[PRes, OReq any] interface {
	Recv(res PRes)
	Next() (req OReq, ok bool)
}
```

# Examples

### `Run` (Direct Pipeline)

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

errCh := make(chan error, 1)
go func() {
    err := <-errCh
    if err != nil {
        log.Printf("Pipeline error: %v", err)
        cancel()
    }
}()

state := NewState() // User-defined State implementation
orchestrator := bilevel.NewOrchestrator(
	Propose, // User-defined ProposeFunc
	Observe, // User-defined ObserveFunc
	proposeConcurrency,
	observeConcurrency,
)

bilevel.Run(orchestrator, ctx, state, errCh)
```

### `RunWithAdapter` (Pipeline with Transformation)

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

errCh := make(chan error, 1)
go func() {
    err := <-errCh
    if err != nil {
        log.Printf("Pipeline error: %v", err)
        cancel()
    }
}()

state := NewState()       // User-defined State implementation
adapter := NewAdapter()   // User-defined Adapter implementation
orchestrator := bilevel.NewOrchestrator(
	Propose,
	Observe,
	proposeConcurrency,
	observeConcurrency,
)

bilevel.RunWithAdapter(orchestrator, ctx, state, adapter, errCh)
```
