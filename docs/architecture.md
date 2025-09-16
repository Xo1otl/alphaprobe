# 導入

最近LLMを用いた解の発見が流行っている、Alpha EvolveやDeep Researcher with Test-Time Diffusionなどがgoogleによって開発され、大成功を収めている。

このような手法のうち、化学・物理モデルの発見に特化したものの開発に取り組んでいる。

# Abstract

化学・物理モデルの発見では、データセットに適合する数式や微分方程式を構成することが目的である。

機械学習でブラックボックスモデルを構築することも可能だが、そうではなく、解釈可能な理論式を見つけたい。

データや理論値とのフィッティング度合いなどはある程度決定的に測れる。しかし、化学方程式の妥当性や微分方程式の近似解の発見など、計算精度だけが指標でない場合も多い。オッカムの剃刀の原則に則ったモデルのコンパクトさ（記述長）もまた、主要な指標となりうる。

先行研究で考案されている手法も様々であり、本稿では、できるだけ広いカバー範囲を持つシステムの構想を行う。

# C4 Model

## Level 1: System Context Diagram (システムコンテキスト図)

Alpha Probeシステム、ユーザー（研究者）、主要な外部システムとの関係性を示す。

```plantuml
@startuml
!include https://raw.githubusercontent.com/plantuml-stdlib/C4-PlantUML/master/C4_Context.puml

' 図のタイトル
title Level 1: System Context Diagram for Alpha Probe

' 要素の定義
Person(researcher, "研究者", "化学・物理モデルを発見するユーザー")
System(alpha_probe, "Alpha Probe Framework", "数式・理論モデル発見ライブラリ")
System_Ext(llm, "LLMやGPU", "仮説生成・評価に用いる外部基盤モデル")
System_Ext(external_analysis, "外部分析基盤", "分析・データ保存用プラットフォーム (任意)")

' 関係性の定義
Rel(researcher, alpha_probe, "探索実行・結果分析")
Rel(alpha_probe, llm, "仮説生成・評価をAPI依頼")
Rel(alpha_probe, external_analysis, "分析データを転送")

@enduml
```

## Level 2: Container Diagram (コンテナ図)

Alpha Probeライブラリを構成するサービスをコンテナとして示す。認証・認可はGCPなどにオフロードする予定。

```plantuml
@startuml
!include https://raw.githubusercontent.com/plantuml-stdlib/C4-PlantUML/master/C4_Container.puml

' 図のタイトル
title Level 2: Container Diagram for Alpha Probe

' 要素の定義 (外部)
Person(researcher, "研究者", "Alpha Probeフレームワークを利用して研究を行う")
System_Ext(llm, "LLMやGPU", "外部の計算リソースや大規模言語モデル")

' クライアントサイドのシステム境界
System_Boundary(client, "研究者のPC/ブラウザ") {
    Container(spa, "Web UI", "Single-Page App", "JavaScript, React/Vueなど。ユーザーインターフェースを提供")
}

' サーバーサイドのシステム境界
System_Boundary(alpha_probe, "Alpha Probe Framework (Backend)") {
    Container(command_service, "Command Service", "Backend", "探索プロセスの実行。ログを生成。")
    Container(projection_service, "Projection Service", "Data Processor", "状態データの変換・転送")
    Container(query_service, "Query Service", "API Service", "分析データへのクエリ実行")

    ' --- リアルタイムロギング用のコンテナを追加 ---
    Container(realtime_service, "Real-time Service", "Real-time Push Protocol", "クライアント接続を管理し、Brokerからログをストリーミング")
    ContainerQueue(message_broker, "Message Broker", "Durable Pub/Sub (Redpandaなら普通に軽い.)", "ログメッセージの永続化とメッセージバス")
    ' -----------------------------------------

    ContainerDb(primary_db, "Primary Datastore", "MongoDB", "探索プロセスの状態を永続化")
    ContainerDb(analysis_db, "Analysis Datastore", "DWH/VDB/RDP", "分析・可視化用に最適化されたデータ")
}

' 関係性の定義
' 研究者はブラウザ上のSPAを利用する
Rel_D(researcher, spa, "利用する", "HTTPS")

' SPAはバックエンドAPIと通信する
Rel(spa, command_service, "探索リクエスト", "JSON/HTTPS")
Rel(spa, query_service, "結果照会", "JSON/HTTPS")

' --- リアルタイムロギングの関係性を追加 ---
' 1. UIがReal-time Serviceと永続的な接続を確立する
Rel(spa, realtime_service, "ログ購読", "WSS / HTTPS")

' 2. Command ServiceがログをMessage Brokerに発行する
Rel_R(command_service, message_broker, "ログを発行", "Async API")

' 3. Real-time ServiceがMessage Brokerからログを購読する
Rel_L(realtime_service, message_broker, "ログを購読")
' -----------------------------------------


' バックエンド内部の通信
Rel_D(command_service, primary_db, "R/W", "状態の永続化・復元")
Rel_L(command_service, llm, "APIリクエスト", "仮説生成・評価")

Rel(primary_db, projection_service, "Push (Change Streams)", "変更を通知")
Rel_D(projection_service, analysis_db, "Write", "変換データ保存")

Rel(query_service, analysis_db, "Query", "データ照会")
@enduml
```

## Level 3: Component Diagram for Command Service

`Command Service`の内部コンポーネントを示す。

```plantuml
@startuml
!include https://raw.githubusercontent.com/plantuml-stdlib/C4-PlantUML/master/C4_Component.puml

title Level 3: Component Diagram for I. Command Service

Container(ui_service, "UI Service")
ContainerDb(primary_db, "Primary DB", "永続化")
System_Ext(external_services, "External Services", "LLM, GPUなど")

Container_Boundary(command_service, "Command Service") {
    Component(application, "Application", "エントリーポイント")
    Component(control_loop, "ControlLoop", "中央コントローラー")
    Component(state, "State", "メモリ内状態 (Originator)")
    Component(repository, "Repository", "永続化層 (Caretaker)")

    Boundary(strategy, "Strategy (戦略)") {
        Component(dispatch, "Dispatch", "タスク発行ロジック")
        Component(propagate, "Propagate", "状態更新ロジック")
        Component(should_terminate, "ShouldTerminate", "終了判定ロジック")
    }
    
    Boundary(pipeline, "Execution Pipeline (実行パイプライン)") {
        Component(task1_pool, "Task1 Pool", "ワーカー群")
        Component(aggregator, "Aggregator", "集約・バッチ処理")
        Component(task2_pool, "Task2 Pool", "ワーカー群")
    }
}

' --- 初期化フロー ---
Rel(ui_service, application, "1. 検索リクエスト")
Rel(application, repository, "2. 状態ロード指示")
Rel(repository, primary_db, "読み込み")
Rel(repository, state, "3. 状態を復元")
Rel(application, pipeline, "4. パイプライン初期化")
Rel(application, control_loop, "5. ループ開始")


' --- メインループのフロー ---
Rel(control_loop, should_terminate, "a. 終了確認")
Rel(control_loop, propagate, "c. 状態更新")
Rel(control_loop, dispatch, "d. タスク発行 (状態更新)")
Rel(control_loop, repository, "e. 状態保存 (定期的)")


' --- 状態アクセスとMementoパターン ---
Rel(dispatch, state, "読み書き")
Rel(propagate, state, "読み書き")
Rel(should_terminate, state, "参照")
Rel_Back(state, repository, "Memento作成")


' --- パイプラインのデータフロー ---
Rel(dispatch, task1_pool, "リクエスト送信")
Rel(task1_pool, aggregator, "中間結果送信")
Rel(aggregator, task2_pool, "集約結果送信")
Rel(task2_pool, control_loop, "b. 最終結果を通知")
Rel(pipeline, external_services, "利用")
@enduml
```
