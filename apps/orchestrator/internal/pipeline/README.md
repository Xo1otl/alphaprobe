# Introduction

In Go's CSP model, managing the lifecycle of goroutines—avoiding deadlocks, synchronizing state, and ensuring a graceful shutdown of the entire system—requires careful, albeit boilerplate, implementation.

This `pipeline` package provides simple, composable primitives that encapsulate these concerns. This allows users to focus on implementing their application's core logic.

# Overview

This package offers three main components for building concurrent pipelines, based on a "ring architecture" model where data circulates through channels.

-   **`Ring`**: A container that manages the entire lifecycle (creation, execution, termination) of the pipeline.
-   **`GoWorkers`**: An asynchronous component that executes time-consuming tasks, such as I/O-bound operations, in parallel across multiple goroutines.
-   **`GoController`**: A synchronous component that runs on a single goroutine to handle state management and task distribution logic.

By connecting these components with channels, you can construct a pipeline like the one shown below.

```mermaid
graph TD
    subgraph Pipeline Ring
        StateController[GoController: State]
        ProposeWorkers[GoWorkers: Propose]
        AdapterController[GoController: Adapter]
        ObserveWorkers[GoWorkers: Observe]
    end

    StateController -- Propose Tasks --> ProposeWorkers;
    ProposeWorkers -- Propose Results --> AdapterController;
    AdapterController -- Observe Tasks --> ObserveWorkers;
    ObserveWorkers -- Observe Results --> StateController;
```

# Ring

The `Ring` manages the lifetime of all components within the pipeline. Its responsibilities are to handle cancellation via a `context` and to wait for the graceful termination of all goroutines using a `sync.WaitGroup`.

### Usage

1.  Create a `Ring` instance from a `context` using `pipeline.NewRing(ctx)`.
2.  Pass the created `Ring` instance to all `GoController` and `GoWorkers` components to start their respective goroutines.
3.  Call `ring.Wait()` at the end of your main logic. This is a blocking call that waits until all goroutines in the pipeline have gracefully terminated.

Termination can be triggered in two ways:
- **Graceful Shutdown**: A `GoController` signals it's done, closing its output channel. This closure propagates through the pipeline, causing each subsequent component to finish its work and shut down.
- **Forced Shutdown**: The `context` provided to the `Ring` is canceled.

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel() // Good practice to ensure context is always cancelled.

ring := pipeline.NewRing(ctx)

// ... Start GoWorkers and GoController with the ring ...

// The pipeline will now run.
// It will stop either when a graceful shutdown is initiated by a component,
// or when the context is cancelled.

ring.Wait()  // Blocks until all goroutines have finished.
```

# GoWorkers

`GoWorkers` launches and manages a group of worker goroutines that execute a specific task (`taskFn`) with a specified degree of parallelism (`concurrency`). It is primarily used for asynchronous operations where parallelization can improve throughput, such as network I/O or heavy computations.

It receives tasks from the `reqCh` channel, executes the `taskFn`, and sends the results to the `resCh` channel. When the `Ring`'s `context` is canceled, all workers terminate safely. The `resCh` is closed after all worker goroutines have finished.

# GoController

`GoController` manages state and task distribution. It runs on a single goroutine to prevent data races without locks. It receives results from one channel, processes them, and sends new tasks to another.

This behavior is defined by three callback functions:

* `onResult`: Processes a result.
* `onNextTask`: Returns the next task to be sent.
* `onTaskSent`: Confirms a task has been sent.

## Callback Guarantees

The callbacks have specific invocation rules:

* The task returned by `onNextTask` is a **candidate**. It is **not guaranteed** to be sent, as it can be preempted by an incoming result.
* `onTaskSent` is called **only after** a task is successfully sent. State changes, such as removing an item from a queue, should be performed here.

This design supports two implementation patterns:

1.  **Latest-State Reflection**: Unsent tasks are automatically discarded.
    * **`onNextTask`**: Calculates a task from the current state.
    * **`onTaskSent`**: (Optional) Updates status.

2.  **Queuing**: Guarantees no task loss via an external queue.
    * **`onNextTask`**: Returns the task at the queue's head **without removing it**.
    * **`onTaskSent`**: Removes the task from the queue's head.

# Shutdown Sequence

The pipeline supports two shutdown mechanisms: graceful and forced.

### Graceful Shutdown

A graceful shutdown is initiated when a `GoController` determines that the pipeline's work is complete.

1.  A `GoController`'s `onResult` callback returns `true`.
2.  The `GoController` immediately returns, and its `defer` statement closes its output channel (`reqCh`).
3.  In the subsequent `GoWorkers` stage, each worker goroutine is listening on the `reqCh`. When the channel is closed, the `select` statement's read operation returns a zero value and `ok == false`. This causes the worker's processing loop to terminate.
4.  Some workers may be in the middle of executing a task when the channel closes. The `GoWorkers` component waits for these in-flight tasks to complete.
5.  After all worker goroutines in the stage have finished and exited, the `GoWorkers` component closes its own output channel (`resCh`).
6.  **This closure of `resCh` serves as the shutdown signal for the next stage in the pipeline.** This process repeats, creating a chain reaction that gracefully shuts down each component in sequence.
7.  The `ring.Wait()` call unblocks only after every component has shut down and all their goroutines have terminated.

### Forced Shutdown

A forced shutdown occurs when the `context` passed to `pipeline.NewRing(ctx)` is canceled.

1.  The `context`'s `Done()` channel is closed.
2.  All `select` statements within `GoController` and `GoWorkers` are listening for this cancellation.
3.  Upon cancellation, each goroutine immediately returns, terminating its execution.

This provides a mechanism to forcibly stop the entire pipeline from an external signal.