package llmsr

import (
	"context"
	"fmt"

	"alphaprobe/orchestrator/internal/pb"
)

// NewGRPCPropose creates a new Propose function that communicates over gRPC.
func NewGRPCPropose(client pb.LLMSRClient) func(context.Context, ProposeRequest) ProposeResult {
	return func(ctx context.Context, req ProposeRequest) ProposeResult {
		pbParents := make([]*pb.Program, len(req.Parents))
		for i, p := range req.Parents {
			pbParents[i] = &pb.Program{Skeleton: string(p.Skeleton), Score: p.Score}
		}

		pbReq := &pb.ProposeRequest{Parents: pbParents}
		resp, err := client.Propose(ctx, pbReq)
		if err != nil {
			return ProposeResult{Err: fmt.Errorf("%w: gRPC propose error: %w", ErrInPropose, err)}
		}

		skeletons := make([]Skeleton, len(resp.Skeletons))
		for i, s := range resp.Skeletons {
			skeletons[i] = Skeleton(s)
		}

		return ProposeResult{
			Skeletons: skeletons,
			Metadata:  Metadata{IslandID: req.IslandID},
		}
	}
}

// NewGRPCObserve creates a new Observe function that communicates over gRPC.
func NewGRPCObserve(client pb.LLMSRClient) func(context.Context, ObserveRequest) ObserveResult {
	return func(ctx context.Context, req ObserveRequest) ObserveResult {
		if req.Err != nil {
			return ObserveResult{
				Metadata: req.Metadata,
				Err:      req.Err,
			}
		}

		pbReq := &pb.ObserveRequest{Skeleton: string(req.Query)}
		resp, err := client.Observe(ctx, pbReq)
		if err != nil {
			return ObserveResult{
				Query:    req.Query,
				Metadata: req.Metadata,
				Err:      fmt.Errorf("%w: gRPC observe error: %w", ErrInObserve, err),
			}
		}

		return ObserveResult{
			Query:    Skeleton(resp.Skeleton),
			Evidence: resp.Score,
			Metadata: req.Metadata,
		}
	}
}
