# Go 1.25+ WaitGroup usage
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

# **My Concern**
About making types more restrictive to clarify the framework's philosophy

The `GoController` handles channels and needs to be encapsulated in a struct. However, what if we allow users to pass simple functions, and then have the `Runner` be responsible for wrapping them into the required struct?

Could we adopt the API proposal below? Additionally, regarding the `data` returned by `propose`, perhaps the `Runner` could wrap the user-provided `observe` function to simply pass this data through at the end.

## API Proposal

```go
// B(asis): Input for the Propose function
// C(andidates): Primary output from the Propose function
// D(ata): Output from Propose that is not used by Observe
// Q(uery): Input for the Observe function
// E(vidence): Output from the Observe function
type ProposeFunc[B, C, D any] func(ctx context.Context, basis B) (C, D)
type ObserveFunc[Q, E, D any] func(ctx context.Context, query Q) E
type FanOutFunc[C, Q any] func(candidates C) []Q

// The State controller only cares about the initial input and final output, so C is not needed.
type State[B, Q, E, D any] interface {
	Update(query Q, evidence E, data D) (done bool)
	Next() (basis B, ok bool)
	Sent(basis B)
}

// The logic to construct the appropriate wrappers will be implemented within Run and RunWithFanOut.
```

This approach would result in an API that better reflects its philosophy as an exploration framework. It would also provide implementers with a clearer set of tasks compared to a design that is too flexible.

# **Your Task**
How do you think? Is it possible?
