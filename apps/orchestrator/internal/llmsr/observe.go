package llmsr

import (
	"context"
	"fmt"
	"strconv"

	"alphaprobe/orchestrator/internal/pb"
)

// MockObserve provides a deterministic, predictable score based on the skeleton's content.
func MockObserve(ctx context.Context, req ObserveRequest) ObserveResult {
	// time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
	if req.Err != nil {
		return ObserveResult{
			Metadata: req.Metadata,
			Err:      req.Err,
		}
	}

	val, err := strconv.Atoi(req.Query)
	if err != nil {
		return ObserveResult{
			Query:    req.Query,
			Metadata: req.Metadata,
			Err:      fmt.Errorf("invalid skeleton: (%v): %w", err, ErrInObserve),
		}
	}

	score := float64(val)
	return ObserveResult{
		Query:    req.Query,
		Evidence: score,
		Metadata: req.Metadata,
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
				Err:      fmt.Errorf("gRPC observe error (%v): %w", err, ErrInObserve),
			}
		}

		return ObserveResult{
			Query:    ProgramSkeleton(resp.Skeleton),
			Evidence: resp.Score,
			Metadata: req.Metadata,
		}
	}
}
