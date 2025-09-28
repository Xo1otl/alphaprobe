# prompt
```
1. pipeline.goとREADME.mdを読んでください。
2. orchestrator.goを読んでください。
3. llmsr.goとllmsr_test.goを読んでください。
4. task.mdを読んで、Your Taskについて考察してください。
必ずこの順番で読み進めてください、それでは開始
```

# My Concern
proposeResを直接observeReqに転送するだけでよい場合もある。
rastrigin.go見てほしい。

# TODO
OrchestratorのメソッドレシーバとしてRunを作っていると、Orchestratorの型引数でPResとOReqを異なる型として受け取った時点で、内部のメソッドではRunとRunWithAdapterの二種類を用意するのが難しい。

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
