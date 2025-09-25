# FIXME

## structure
* pipeline.go: <TODO: 説明>
* runner.go: <TODO: 説明>
* llmsr.go/rastrigin.go: <TODO: 説明>
* llmsr_test.go/rastrigin_test.go: <TODO: 説明>

## Idea
* propose/observeは時間のかかる処理を想定している、context.Contextをpipelineやrunnerをはじめとした中心的なコンポーネントを含めてすべてが引き回す必要あるか？それとも参照を工夫すればpipelineまで変更せずに対応できるのか？
* errorハンドリング全くやってないけどどうする？
* まてよ？そもそも今停止処理がupdateがdoneを返す感じになってるけど、ここwg.Done使った方がいいんか？
 