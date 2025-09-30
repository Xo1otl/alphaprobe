# Introduction

近年、Alpha EvolveやDeep Researcher with Test-Time Diffusionのように、大規模言語モデル（LLM）を用いて科学的な発見を自動化するアプローチが成功を収めています。本プロジェクト「Alpha Probe」は、この流れを汲み、特に化学や物理学の分野における解釈可能な理論モデル（数式や微分方程式など）の発見に特化したフレームワークを構築することを目的とします。機械学習によるブラックボックス的な予測モデルではなく、人間が理解できる普遍的な法則や式を見つけ出すことがゴールです。

# Overview

Alpha Probeのシステムアーキテクチャは、C4モデルを用いて設計されています。本ドキュメントでは、システムの全体像から詳細に至るまでを段階的に説明します。まず、システムが外部環境とどのように関わるかを示す **System Context** を定義します。次に、システムを構成する主要なサービス群を描く **Container Architecture** を示します。最後に、中核機能である探索プロセスを実行する **Command Service Component Architecture** の内部構造を詳述します。

# System Context

システムコンテキスト図は、Alpha Probeフレームワーク、それを利用する「研究者」、および連携する外部システムとの関係性を示します。研究者はAlpha Probeを通じてモデル探索を実行し、システムは内部でLLMやGPUなどの外部計算リソースを活用します。

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

# Container Architecture

コンテナ図は、Alpha Probeフレームワークを構成する主要なサービス（コンテナ）を視覚化したものです。ユーザーからのリクエストを受け付けるUI、探索プロセスを司るCommand Service、データの変換や問い合わせを担う各種サービス、そしてリアルタイムなログ配信基盤などが連携して動作します。

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

# Command Service Component Architecture

Command Serviceは、モデル探索プロセスの実行を担う中核コンポーネントです。その内部は、GoのCSP（Communicating Sequential Processes）モデルに基づいたリングアーキテクチャで設計されています。このアーキテクチャでは、「State」「Propose」「Adapter」「Observe」という責務を持つコンポーネントがリング状に接続され、チャネルを通じてデータを循環させながら処理を進めます。これにより、並列処理と状態管理を安全かつ効率的に行います。

```plantuml
@startuml
!include https://raw.githubusercontent.com/plantuml-stdlib/C4-PlantUML/master/C4_Component.puml

title Level 3: Component Diagram for Command Service (Ring Architecture)

Container(ui_service, "UI Service", "Web UI")
ContainerDb(primary_db, "Primary DB", "永続化")
System_Ext(external_services, "External Services", "LLM, GPUなど")

Container_Boundary(command_service, "Command Service") {
    Component(application, "Application", "エントリーポイント。Orchestratorを生成し、パイプラインリングを構築・実行する。")
    Component(repository, "Repository", "永続化層 (Caretaker)")

    Boundary(pipeline_ring, "Pipeline Ring (bilevel実行モデル)") {
        Component(state, "State Controller", "GoController", "状態管理。Proposeタスクを生成し、Observe結果を処理する。")
        Component(propose, "Propose Workers", "GoWorkers", "Proposeタスクを並列実行する。")
        Component(adapter, "Adapter Controller", "GoController", "Propose結果を集約・変換し、Observeタスクを生成する。")
        Component(observe, "Observe Workers", "GoWorkers", "Observeタスクを並列実行する。")
    }
}

' --- 初期化フロー ---
Rel(ui_service, application, "1. 検索リクエスト")
Rel(application, repository, "2. 状態ロード指示")
Rel(repository, primary_db, "読み込み")
Rel(repository, state, "3. 状態を復元")
Rel(application, pipeline_ring, "4. パイプラインを開始")


' --- パイプラインのデータフロー (リングアーキテクチャ) ---
Rel(state, propose, "Proposeリクエストを送信")
Rel(propose, adapter, "Propose結果を送信")
Rel(adapter, observe, "Observeリクエストを送信")
Rel(observe, state, "Observe結果を送信")


' --- 永続化 ---
Rel(application, repository, "状態保存 (定期的)")
Rel(repository, state, "Mementoを作成")
Rel_Back(state, repository, "状態を永続化")


' --- 外部サービス利用 ---
Rel(propose, external_services, "利用")
Rel(observe, external_services, "利用")

@enduml
```