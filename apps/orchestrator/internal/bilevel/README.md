# prompt
1.  Please read `pipeline.go` and `README.md`.
2.  Please read `controller.go` and `orchestrator.go`.
3.  Please read `rastrigin.go` and `rastrigin_test.go`.
4.  Please read `task.md` and then consider "Your Task".
It is important that you read these in this exact order. Let's begin.

# TODO
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
