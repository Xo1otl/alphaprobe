# **NOTE**
* The `Update`/`Next` functions will be called unpredictably within a single goroutine, so no locks are needed.
* The code uses the latest Go syntax and compiles successfully.

# MyConcern
* esモジュールを作る
* es.State Interfaceはbilevel stateの拡張であるから、RunWithAdapterにそのまま渡せる
* bilevel.Stateに加えてEventRecordを出力できることを 

# Your Task
リファクタリング案を考えてください
