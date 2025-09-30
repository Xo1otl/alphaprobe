package llmsr

import (
	"context"
	"math/rand"
	"strconv"
	"time"

	"alphaprobe/orchestrator/internal/pb"
)

// MockObserve provides a deterministic, predictable score based on the skeleton's content.
func MockObserve(ctx context.Context, req ObserveRequest) ObserveResult {
	time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
	val, err := strconv.Atoi(req.Query)
	if err != nil {
		return ObserveResult{
			Query:    req.Query,
			Metadata: req.Metadata,
			Err:      err,
		}
	}

	score := float64(val)
	return ObserveResult{
		Query:    req.Query,
		Evidence: score,
		Metadata: req.Metadata, // Pass metadata through
	}
}

// NewGRPCObserve creates a new Observe function that communicates over gRPC.
func NewGRPCObserve(client pb.LLMSRClient) func(context.Context, ObserveRequest) ObserveResult {
	return func(ctx context.Context, req ObserveRequest) ObserveResult {
		pbReq := &pb.ObserveRequest{Skeleton: string(req.Query)}
		resp, err := client.Observe(ctx, pbReq)
		if err != nil {
			return ObserveResult{
				Query:    req.Query,
				Metadata: req.Metadata,
				Err:      err,
			}
		}

		return ObserveResult{
			Query:    ProgramSkeleton(resp.Skeleton),
			Evidence: resp.Score,
			Metadata: req.Metadata,
		}
	}
}
