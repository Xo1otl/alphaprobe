# State Persistence Architecture (Unit of Work & Memento Pattern)

## 概要
Alpha Probeフレームワークにおいて、探索状態（State）の中断・再開や定期保存を担う**永続化層（Persistence Layer）**の設計方針についてまとめたドキュメントです。

非同期並列に行われる探索プロセスにおいて、いかにして「Domain（アルゴリズム）」、「Application（オーケストレータ・ユースケース）」、「Infrastructure（DBの実装）」の責務を綺麗に分離（Clean Architecture）しつつ、新しいアルゴリズムにも柔軟に対応させるかを定義しています。

## 設計の基本方針

### 1. 責務の完全な分離
永続化に関わる以下の3つの要素を、レイヤーごとに完全に独立させます。

*   **「何を」保存するか (What to save)**
    *   **担当モジュール**: `llmsr`, `MAP-Elite` 等の具体的な探索モジュール
    *   **実装内容**: 内部の非公開状態（小文字変数など）を安全なデータ構造（Memento/DTO）に詰め替える `CreateMemento()` メソッドだけを提供します。自ら保存処理は呼び出しません。
*   **「いつ」保存するか (When to save)**
    *   **担当モジュール**: `application` モジュールの汎用ラッパー
    *   **実装内容**: トランザクション処理の境界（Unit of Work）を定義します。例えば「100回のUpdateごと」や「プロセスの終了（Ctrl+C）時」に保存の依頼をかけます。
*   **「どう」保存するか (How to save)**
    *   **担当モジュール**: `mongo`, `postgres` 用の Repositoryモジュール
    *   **実装内容**: Applicationから渡されたMementoをただDBに書き込みます。

### 2. Mementoパターンを用いたカプセル化の維持
Domainである `State` の内部変数をパブリックにする（情報隠蔽を壊す）ことは避け、Mementoパターンを採用します。State自身が現在の状態の「スナップショット（Memento）」を生成して返す設計とします。

## インターフェース設計と Generics の活用

Goの型の安全性を保ちつつ、未知の探索アルゴリズム（State）や保存先（Repository）に対しても汎用的に機能するよう、Genericsを用いてインターフェースを合成・定義します。

```go
package application

import "context"
import "orchestrator/internal/bilevel"

// ---------------------------------------------------------
// 1. 汎用インターフェース定義
// ---------------------------------------------------------

// Snapshotter: 自身の状態を Memento 型として切り出せる能力
type Snapshotter[M any] interface {
    CreateMemento() M
}

// PersistableState: 実行パイプラインの能力 ＋ Memento作成の能力 を併せ持つ合成インターフェース
type PersistableState[PReq, ORes, M any] interface {
    bilevel.State[PReq, ORes] // (Issue と Update メソッド)
    Snapshotter[M]
}

// Repository: Memento型を永続化できる能力
type Repository[M any] interface {
    Save(ctx context.Context, m M) error
}
```

## デコレータパターンによる拡張 (WithUnitOfWork)

`bilevel` 本体のループ構造を変えることなく、Application層のポリシー（いつ保存するか）を注入するために、「状態ラッパー（Decoratorパターン）」を使用します。

```go
// ---------------------------------------------------------
// 2. Unit of Work の自動実行ラッパー
// ---------------------------------------------------------

// WithUnitOfWork は、指定されたタイミングで自動的に保存を行う純粋な `bilevel.State` を返します。
// ※ 対象の baseState は CreateMemento を持っている (PersistableState である) 必要があります。
package application

func WithUnitOfWork[PReq, ORes, M any](
    ctx context.Context,
    baseState PersistableState[PReq, ORes, M],
    repo Repository[M],
    commitInterval int,
) bilevel.State[PReq, ORes] {
    return &uowWrapper[PReq, ORes, M]{
        baseState:      baseState,
        repo:           repo,
        ctx:            ctx,
        commitInterval: commitInterval,
        updateCount:    0,
    }
}

type uowWrapper[PReq, ORes, M any] struct {
    baseState      PersistableState[PReq, ORes, M]
    repo           Repository[M]
    ctx            context.Context
    commitInterval int
    updateCount    int
}

// Updateの実行をフックし、トランザクションの区切り（Unit of Work）を判定する
func (w *uowWrapper[PReq, ORes, M]) Update(res ORes) (done bool, err error) {
    // 1. 本来のドメイン処理を委譲
    done, err = w.baseState.Update(res)
    
    w.updateCount++
    
    // 2. 保存条件（Unit of Work の終了タイミング）の判定
    isIntervalReached := w.updateCount%w.commitInterval == 0
    isInterrupted := w.ctx.Err() != nil

    if done || isIntervalReached || isInterrupted {
        // 3. Domain に何(What)を保存するか尋ねる
        memento := w.baseState.CreateMemento()
        
        // 4. Infrastructure にどう(How)保存するか伝える
        _ = w.repo.Save(w.ctx, memento) // TODO: エラーハンドリング
    }
    
    return done, err
}

func (w *uowWrapper[PReq, ORes, M]) Issue() (req PReq, ok bool, err error) {
    return w.baseState.Issue()
}
```

## 統合時の実行フロー (Wiring)

最も外側のレイヤー（Mainやテストコード）で依存を解決し、マトリョーシカのようにラッパーを被せてパイプラインに流し込みます。

```go
// 1. 具象クラスの生成
myState := llmsr.NewDeterministicState(...) 
myRepo := mongo.NewLLMSRRepository(...)

// 2. Unit of Work のポリシー（Application）を被せる
// myState は PersistableState (CreateMemento持ち) なので直接渡せる
storingState := application.WithUnitOfWork(ctx, myState, myRepo, 100) 

// 3. インメモリの EventSourcing ポリシーなどを追加で被せることも可能
// storingState は戻り値上 bilevel.State となっているため安全に連鎖可能
esState, _ := bilevel.WithEventSourcing(storingState)

// 4. 実行！ (パイプラインリングは永続化の存在を一切知らない)
bilevel.RunWithAdapter(orchestrator, ctx, esState, adapter, errCh)
```

## メリット
1. **堅牢性**: 予期せぬクラッシュやGraceful Shutdown時でも、完全に整ったスナップショット（Unit of Work）から探索を100%確実に再開できます。
2. **高い拡張性**: 今後、新しいアルゴリズム（例:MAP-Elite）を追加する場合でも、各クラスに `CreateMemento` メソッドをたった1つ追加するだけで済みます。保存・中断・再開に関する煩雑なマネジメントコードは全て `WithUnitOfWork` で共通化されているため、一切書く必要がありません。
