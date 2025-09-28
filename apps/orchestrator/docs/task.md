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

# My Concern
* llmsr.goのUpdate関数の在り方について考え直したい
* GoControllerWithStateを作る。これは、GoControllerのonRequest/onNextTask/onTaskSentに渡される、Update/NextTask/<後処理の関数名>を持つState interfaceを用いて実行する。State Interfaceの実装は外部に委譲し、queueも持たず、maxQueueSizeという引数は不要となる
* Runでは、新しく用意したGoControllerWithStateのみを使用し、Update/NextTask/<関数名未定>を実装した、StateInferfaceの実装を受け取る
* **Updateという一つだけの関数になるわけではなく、三つに分解でき、queueの使用も強制でなくなったことから、llmsr.Updateの処理を論理的に分解することが可能となる**
* GoCtonrollerWithQueueからはinitialTasksの引数を削除できそう
* RunWithAdapterはRunWithFanOut(..., adapterFn, ...)に関数名を変更し、onResultに渡すadapterFnだけ取れればよい

# Your Task
MyConcernにあるようなリファクタリングを考えている。
コードのTODOに書いてある内容や、MyConcernを踏まえて、**まずGoControllerWithStateを導入することによりllmsr.Updateがどのように分離されてシンプルになるか**検討してみてほしい。fan-out adapterのためのcontrollerや、そのほかの修正は検討ができてから考える。
