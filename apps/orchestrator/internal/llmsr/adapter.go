package llmsr

type Adapter struct {
	queue []ObserveRequest
}

func NewAdapter() *Adapter {
	return &Adapter{
		queue: make([]ObserveRequest, 0),
	}
}

func (a *Adapter) Recv(res ProposeResult) {
	if res.Err != nil {
		a.queue = append(a.queue, ObserveRequest{Metadata: res.Metadata, Err: res.Err})
		return
	}
	for _, skeleton := range res.Skeletons {
		req := ObserveRequest{
			Query:    skeleton,
			Metadata: res.Metadata,
		}
		a.queue = append(a.queue, req)
	}
}

func (a *Adapter) Next() (ObserveRequest, bool) {
	if len(a.queue) == 0 {
		return ObserveRequest{}, false
	}
	req := a.queue[0]
	a.queue = a.queue[1:]
	return req, true
}

type ProposeResult struct {
	Skeletons []Skeleton
	Metadata  Metadata
	Err       error
}

type ObserveRequest struct {
	Query    Skeleton
	Metadata Metadata
	Err      error
}
