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
proposeResを直接observeReqに転送するだけでよい場合もある。
rastrigin.go見てほしい。

# TODO
OrchestratorのメソッドレシーバとしてRunを作っていると、Orchestratorの型引数でPResとOReqを異なる型として受け取った時点で、内部のメソッドではRunとRunWithAdapterの二種類を用意するのが難しい、goではstructで決定した型がそのままメソッドの型として確定するので、structでPResとOReqが異なるものを取れるようにしている時点でメソッドから一致するか判定する方法が存在しない。

それよりも、OrchestratorをRunが受け取るようにして、使う側では
```
o := bilevel.NewOrchestrator(...)
bilevel.Run(o)
```
とか
```
o := bilevel.NewOrchestrator(...)
bilevel.RunWithAdapter(o)
```
とかにするのはどうなんだろう.

Runの部分でPResとOReqの型が一致してるOrchestratorだけが受け取れるようになって、自然に両方対応できたりしないかな。
型システム的に実現可能かどうか、厳密に検討してみてほしい
