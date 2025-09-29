# package bilevel
```go
// Package bilevel provides a framework for orchestrating two-level concurrent proposal and observation tasks.
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

// State manages the overall progress and generates high-level tasks.
type State[PReq, ORes any] interface {
	// Update updates the state with a result from the observation stage.
	Update(res ORes) (done bool)
	// Next provides the next request for the proposal stage.
	Next() (req PReq, ok bool)
	// Sent confirms the dispatch of the request last provided by Next.
	Sent(req PReq)
}

// Adapter transforms proposal results into observation requests.
type Adapter[PRes, OReq any] interface {
	// Recv receives a result from the proposal stage to be processed.
	Recv(res PRes) (done bool)
	// Next provides the next request for the observation stage.
	Next() (req OReq, ok bool)
	// Commit confirms the dispatch of the request last provided by Next.
	Commit(req OReq)
}

// --- Example ---

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
```

# Go 1.25+ WaitGroup
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

# Your Task
Please implement AdapterState in mock.go
