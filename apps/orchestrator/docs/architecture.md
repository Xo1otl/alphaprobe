# Orchestrator 利用ガイド: 依存性注入 (DI) による拡張

`orchestrator`は、依存性注入（DI）を基本設計とした階層的なアーキテクチャを採用しています。利用者は各レイヤーが要求するインターフェースや関数を実装・提供することで、独自の探索アルゴリズムを構築できます。このガイドでは、各レイヤーで何を実装・注入する必要があるかを解説します。

## Layer 1: `pipeline` - コア実行エンジン

`pipeline`は、`orchestrator`の心臓部であり、特定のアルゴリズムに依存しない汎用的な並列実行基盤です。

`pipeline.ControlLoop`を動作させるには、以下の3つの振る舞いを関数として注入する必要があります。

-   `dispatch func(state S, reqCh chan<- Req)`
    -   **役割**: タスクの発行ロジック。
    -   **処理**: 現在の状態 `state` を見て、実行すべきタスク `Req` を生成し、リクエストチャネル `reqCh` に送信します。
-   `propagate func(state S, result Res)`
    -   **役割**: 結果の反映ロジック。
    -   **処理**: ワーカーが処理を終えた結果 `result` を受け取り、状態 `state` を更新します。
-   `shouldTerminate func(state S) bool`
    -   **役割**: 終了判定ロジック。
    -   **処理**: 現在の状態 `state` を見て、ループを終了すべきかどうかを判定します。

## Layer 2: `islandga` - GA戦略レイヤー

`islandga`は、`pipeline`の汎用実行エンジンを利用して、島モデル遺伝的アルゴリズム（Island Model GA）という具体的な戦略を実装したレイヤーです。利用者は、この`islandga`が要求するコンポーネントを実装することになります。

`islandga.NewRunner`をセットアップするには、以下の3つの要素を実装・定義する必要があります。

### 1. データ型の定義
問題領域に固有のデータ型を3つ定義します。
-   **`Gene`**: 個体の遺伝子（例: `[]float64`）。
-   **`Fitness`**: 評価値。`cmp.Ordered`を満たす必要があります（例: `float64`）。
-   **`Sample`**: 各島が内部で持つ状態（例: `[]islandga.Individual[Gene, Fitness]`）。

### 2. `islandga.Island` インターフェースの実装
単一の島の振る舞いを定義するインターフェースです。これを実装した構造体を作成します。
-   `ID() int`: 島のIDを返します。
-   `Sample() S`: 島の内部状態を返します。これは後述の`ProposeFunc`に渡されます。
-   `Incorporate(...)`: 評価済みの新個体や移住者を、自島の個体群にどう統合するかを定義します。
-   `SelectMigrants(n int)`: 他島へ移住させる個体を`n`体選択するロジックを定義します。

### 3. `islandga.Runner`へ注入する関数
問題固有の遺伝的操作と評価ロジックを実装した関数を2つ作成し、`islandga.NewRunner`に渡します。
-   `ProposeFunc: func(state Sample) (offspring Gene)`
    -   **役割**: 「提案」ロジック。島の状態`Sample`から新しい子個体`Gene`を1つ生成します。
-   `ObserveFunc: func(gene Gene) Fitness`
    -   **役割**: 「観測」ロジック。`Gene`の適応度`Fitness`を計算する目的関数です。

## Layer 3: `rastrigin` - 具体的な問題実装

`rastrigin`はDIを要求する側ではなく、`islandga`が要求するインターフェースと関数を具体的に実装したパッケージです。これは、新しい問題を解く際の実装サンプルとなります。

### `islandga.Island` の具体的な実装方法
-   `Incorporate`: 渡された個体を既存の個体群に加える際、「個体群を適応度の悪い順にソートし、新個体の方が優れていれば、最も劣る個体と入れ替える」という生存戦略を実装しています。
-   `SelectMigrants`: 「個体群を適応度の良い順にソートし、上位`n`個体を移住者として選択する」という移住戦略を実装しています。

### `islandga.Runner`へ注入する関数の具体例
-   `Propose`関数 (ProposeFuncの実装):
    -   **親選択**: トーナメント選択方式で2体の親を選択します。
    -   **交叉**: `CrossoverRate`の確率でBLX-α交叉を実行します。
    -   **突然変異**: 各遺伝子を`MutationRate`の確率で、標準正規分布に従うノイズを加えて変異させます。
    -   これら一連の遺伝的操作によって新しい`Gene`を生成し、返却します。
-   `Observe`関数 (ObserveFuncの実装):
    -   与えられた`Gene`（`[]float64`のベクトル）を引数として、数学的なラスタリン関数 `a * n + Σ(x_i^2 - a * cos(2πx_i))` を計算し、その結果を`Fitness`として返します。