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

Regarding the `Run` function in `controller.go`. We are currently using `ControllerWithQueue` to handle complex logic, which feels forced. All processing is concentrated in the `Update` method. I believe it would be better to decompose this logic into `HandleResult`, `NextTask`, and `TaskSent`, using `rastrigin.go` as an example.

I would like to perform the following refactoring:
1.  Create a new `State` type with decomposed responsibilities, aligned with a new bilevel.State signature.
2.  Use a new `GoControllerWithState` in the orchestrator's `Run` function.

To start, please consider how to separate the responsibilities of `rastrigin.go`'s `State.Update`. Since using a queue will no longer be mandatory, the `State` can behave more like a true state object. This would allow task generation to be moved to the `NextTask` function's responsibility, and `HandleResult` could focus solely on applying results.

# **Your Task**

Following "My Concern," please think about how to refactor `rastrigin.go`'s `State.Update` by separating its responsibilities into `HandleResult`, `NextTask`, and `TaskSent`.
