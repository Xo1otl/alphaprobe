## Architecture
* pipeline.go: 汎用的な非同期パイプラインを構築するための`Loop`と`LaunchWorkers`を提供。
* runner.go: `pipeline`を使い、Propose/Observeモデルの具体的な実行エンジンを構築。
* llmsr.go: `runner`のロジックを実装したLLMSRアルゴリズムのコアロジック。
* llmsr_test.go: `llmsr`と`runner`を結合した統合テスト。

## Go 1.25+ WaitGroup usage
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

## My Concern
* `pipeline.go`について
	* 処理が難しくなってきたから説明まとめた資料欲しい(README.md作ってDocument推敲)
	* updateが複数 or nilのreqを生成する、resもfanoutがあった場合いくつ帰ってくるかわからないということ？
	* pipelineの中が何段あろうと、どれだけ複雑なadapterがあろうと、pipelineは最初と最後だけを扱っており、それが長期的に見て1:1に収束していることが、long runningの条件となる？
	* queueは、長期的に見てこれが1:1の関係に収束するドメインロジックであれば、途中に多少のバーストがあっても対応できるようにするための処置？
	* 探索アルゴリズムが指数関数的にtaskを増やす不安定な性質であっても、例えばサイズの大きすぎない木構造などにおいて、ドメインロジックで停止判定を行えば問題ない？
	* 要するに、pipeline.goの現在の処理はあらゆるパイプライン並列化を適切に処理するベストプラクティとなっているか？確認したい

# Gemini Tasks
concernの、`pipeline.go`についてを考察してほしい。

# 事前準備
pipeline.go/runner.go/llmsr.go/llmsr_test.goを読んでください。
pipeline/README.mdもしっかり読みこんでください。

# pipelineモジュールが終了処理をカプセル化する案
## 新しいpipelineモジュールをrunner.goで使用する時の概念コード
```
controller := pipeline.NewController(ctx)
pipeline.LaunchWorkers(controller, r.proposeConcurrency, proposeTask, proposeReqCh, proposeResCh)
pipeline.LaunchWorkers(controller, r.observeConcurrency, observeTask, observeReqCh, observeResCh)
go r.adapterFn(proposeResCh, observeReqCh) // observeReqChをadapterが適切に閉じる. runner.goのNewFanOutAdapterを使えば問題ない.
controller.Loop(update, initialTasks, proposeReqCh, observeResCh, r.maxQueueSize)
controller.Wait()
```
## 一般化されたパイプライン並列におけるシャットダウン連鎖
1. `Loop`が終了: controller.Loop()が終了すると、proposeReqCh（最初の入力チャネル）をcloseします。
2. `propose`ステージが終了: proposeワーカーはproposeReqChを読み終え、すべてのタスクが完了します。
3. `controller.Wait()`が進行: controller.Wait()は、まずproposeステージのWaitGroupが完了するのを待ちます。完了すると、proposeステージの出力チャネルである `proposeResCh`を`close`します。
4. `adapterFn`が終了: adapterFnは内部でproposeResChをrangeでループしています。proposeResChが閉じられたことで、このループが正常に終了します。そして、adapterFnの責務に従い、自身の出力チャネルである`observeReqCh`を`close`します。
5. `observe`ステージが終了: observeワーカーはobserveReqChを読み終え、すべてのタスクが完了します。
6. `controller.Wait()`が完了: controller.Wait()は、次にobserveステージのWaitGroupが完了するのを待ちます。observeステージが終了したので、これも完了し、observeResChをcloseします。

# Your Task
事前準備をしっかり行ってから、案が妥当かどうか判断してほしい
