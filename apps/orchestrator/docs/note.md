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

```mermaid
graph TD
    subgraph Propose Stage
        Propose_Pool[Propose Worker Pool]
    end
    subgraph Observe Stage
        Observe_Pool[Observe Worker Pool]
    end
    
    Update_Adapter(Stateful Update Adapter<br/>- Manages Task Queue<br/>- Calls updateFn<br/>- Decides when to stop)
    
    Propose_Pool -- POut/Q --> Observe_Pool
    Observe_Pool -- ObserveRes --> Update_Adapter
    Update_Adapter -- PReq --> Propose_Pool
```
