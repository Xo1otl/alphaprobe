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
フレームワークの思想を明確化するために型をより限定的にし、一部の処理をRunnerがWrapする
```go
// B(asis): Proposeの入力
// C(andidates): Proposeの主な出力
// D(ata): Proposeの出力のうち、Observeで使わないもの
// Q(uery): Observeの入力
// E(vidence): Observeの出力
type ProposeFunc[B, C, D any] func(ctx context.Context, basis B) C, D
type ObserveFunc[Q, E, D any] func(ctx context.Context, query Q) E
type FanOutFunc[C, Q any] func(candidates C) []Q

// State Controllerは最初と最後の入出力しか見ないのでCは不要
type State[B, Q, E, D any] interface {
	Update(query Q, evidence E, data D) (done bool)
	Next() (basis B, ok bool)
	Sent(basis B)
}

// RunnerでGoControllerに渡すwrappedObserveFnでは、渡されたobserveを呼び出しつつ最後にDataをくっつける
```

# **Your Task**
please refactor
