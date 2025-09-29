# alphaprobe

## 環境構築
1. `git clone git@github.com:Xo1otl/alphaprobe.git`する
2. `secrets.tar.gz`をproject rootに配置する
3. vscodeの指示に従い、open in containerする

## Memo
* 推敲の時に、`Please remove any modifiers or exaggerations and revise your writing to focus on describing only the "how" and "what" necessary to reproduce the logic.`を追記するといい感じになる。GEMINI.mdでこの手順追加しようかな
* orchestratorの実装側での型引数名の例
    ```go
    // B(asis): Proposeの入力
    // C(andidates): Proposeの主な出力
    // D(ata): Proposeの出力のうち、Observeで使わないもの
    // Q(uery): Observeの入力
    // E(vidence): Observeの出力
    ```
