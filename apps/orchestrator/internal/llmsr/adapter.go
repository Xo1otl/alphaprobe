package llmsr

import "log"

type Adapter struct {
	queue  []ObserveRequest
	Logger *log.Logger
}

func NewAdapter(logger *log.Logger) *Adapter {
	return &Adapter{
		queue:  make([]ObserveRequest, 0),
		Logger: logger,
	}
}

func (a *Adapter) Recv(res ProposeResult) {
	if res.Err != nil {
		a.Logger.Printf("error in proposal: %v", res.Err)
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
