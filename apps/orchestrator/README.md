# Orchestrator

`orchestrator`は、Alpha Probeフレームワークにおける探索プロセス全体を管理・実行するコアコンポーネントである。`docs/architecture.md`で定義されている`Command Service`の責務を担う。

## 概要

`orchestrator`は、探索アルゴリズムの心臓部として機能する。中央の`ControlLoop`が、探索の状態(`State`)を管理し、定義された戦略(`Strategy`)に基づいてタスクの発行(`Dispatch`)と結果の集約・反映(`Propagate`)を繰り返すことで、自律的な探索プロセスを実現する。

GoのCSP(Communicating Sequential Processes)モデルを全面的に採用しており、特にI/Oバウンドなタスク（LLMへのAPIリクエストや外部シミュレータの実行など）を効率的に並列処理するための、汎用的な実行パイプラインを提供する。

## 設計原則

`orchestrator`は、コードの安全性、予測可能性、テスト容易性を最大化するため、以下の原則に基づき設計されている。

-   **状態の一元管理:** 探索や実行に関する全ての状態は、単一の`State`構造体に集約される。`ControlLoop`が並列実行されたタスクの結果を一つずつ処理し`State`を更新するため、**状態変更の直列化**が保証され、競合状態を回避できる。これにより状態の参照や更新が予測可能になり、シリアライズによる中断・再開機能の実装も容易になる。

-   **副作用のない純粋関数:** 利用者が注入するビジネスロジック (`Propose`, `Observe`等) は、副作用のない純粋関数であることが推奨される。入力データを変更せず、常に新しいデータを生成して返すことで、システム全体の状態を健全に保つ。

-   **I/Oバウンド処理の重視:** 各処理ステップは、API連携やシミュレーション実行といった完了までに時間を要するI/Oバウンドな処理を想定している。そのため、データコピー等のわずかな計算コストよりも、アーキテクチャ全体の堅牢性とコードの明確性を優先する。

## 主要コンポーネント

`orchestrator`は、`docs/architecture.md`のComponent Diagramで示されている概念を、より具体的なGoのパッケージとして実装している。

-   **`pipeline`**:
    GoのChannelを用いた、汎用的なワーカープールと制御ループ(`ControlLoop`)を提供する基盤パッケージ。任意の`Dispatch`, `Propagate`, `ShouldTerminate`関数を注入することで、様々なイベント駆動型の探索プロセスを構築できる。

-   **`bilevel`**:
    `pipeline`を応用し、「提案(`Propose`)」と「観測(`Observe`)」の2段階からなる、より具体的なパイプライン実行基盤(`Runner`)を提供する。この`Runner`は、`Propose`ステージから`Observe`ステージを経て`Propagate`ステージまで、一貫したコンテキスト情報を透過的に引き回す機能を持ち、状態更新を容易にする。

-   **`rastrigin` (実装例)**:
    `bilevel.Runner`を利用して、島モデル(Island Model)に基づく遺伝的アルゴリズムを実装した具体例。Rastrigin関数の最小値探索問題を通じて、`orchestrator`の各コンポーネントの利用方法を示している。
    -   **`State`**: `GaState`としてアルゴリズム全体の状態を定義。
    -   **`Propose`**: `Propose`関数として交叉・突然変異による新個体の生成ロジックを実装。
    -   **`Observe`**: `Observe`関数としてRastrigin関数による適応度評価を実装。
    -   **`Dispatch`/`Propagate`/`ShouldTerminate`**: `bilevel.Runner`に注入する戦略関数群として、状態に基づいたタスク発行や結果の反映ロジックを実装。

## ディレクトリ構成

```
.
├── go.mod
├── go.sum
├── README.md
├── docs
│   └── architecture.md
└── internal
    ├── bilevel         # 「提案」「観測」の2段階パイプライン実行基盤
    ├── pipeline        # Go Channelベースの汎用的な並列実行パイプライン
    └── rastrigin       # bilevel Runnerを用いた遺伝的アルゴリズムの実装例
```
