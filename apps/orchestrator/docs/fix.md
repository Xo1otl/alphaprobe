# FIXME

## NOTE
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

## structure
* pipeline.go: 汎用的な非同期パイプラインを構築するための`ControlLoop`と`WorkerPool`を提供。
* runner.go: `pipeline`を使い、Propose/Observeモデルの具体的な実行エンジンを構築。
* llmsr.go: `runner`のロジックを実装したLLMSRアルゴリズムのコアロジック。
* llmsr_test.go: `llmsr`と`runner`を結合した統合テスト。

## Idea
* propose/observeは時間のかかる処理を想定している、context.Contextをpipelineやrunnerをはじめとした中心的なコンポーネントを含めてすべてが引き回す必要あるか？それとも参照を工夫すればpipelineまで変更せずに対応できるのか？
* errorハンドリング全くやってないけどどうする？
* まてよ？そもそも今停止処理がupdateがdoneを返す感じになってるけど、ここwg.Done使った方がいいんか？
* もし直すならば、`controller := pipeline.NewController(ctx)` `controller.LaunchWorkers(args...)` `controller.Loop(args...)`的な感じになっていくのかな
