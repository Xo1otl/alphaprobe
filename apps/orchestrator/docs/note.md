# pipeline並列の一般化

今回のようなpropose/observeループはリング構造をなす

要するに、propose から observeへのadapterが存在する場合、そのadapterと、探索アルゴリズムの中核である状態更新同期GenServerループは、このモデルにおいて等価であり、observeからproposeへのadapterとみなすことができる

つまり、pipelineモジュールではNewAdapterとlaunchworkersの二種類があればよい...？

```mermaid
graph TD
    subgraph Ring Structure
        A[Worker A]
        Ad_AB(Adapter A→B)
        B[Worker B]
        Ad_BC(Adapter B→C)
        C[Worker C]
        Ad_CA(Adapter C→A)
    end

    A -- "Output A" --> Ad_AB;
    Ad_AB -- "Input B" --> B;
    B -- "Output B" --> Ad_BC;
    Ad_BC -- "Input C" --> C;
    C -- "Output C" --> Ad_CA;
    Ad_CA -- "Input A" --> A;
```
