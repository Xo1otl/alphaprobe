package llmsr

import (
	"context"
	"errors"
	"strconv"

	"alphaprobe/orchestrator/internal/pb"
)

// MockPropose generates a predictable set of new skeletons for deterministic testing.
func MockPropose(ctx context.Context, req ProposeRequest) ProposeResult {
	// time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)

	if len(req.Parents) == 0 {
		return ProposeResult{Err: errors.New("no parents provided")}
	}

	bestParent := req.Parents[0]
	for _, p := range req.Parents[1:] {
		if p.Score > bestParent.Score {
			bestParent = p
		}
	}

	parentSkeleton := bestParent.Skeleton
	val, err := strconv.Atoi(parentSkeleton)
	if err != nil {
		return ProposeResult{Err: err}
	}

	newSkeletons := []ProgramSkeleton{
		strconv.Itoa(val + 1),
		strconv.Itoa(val + 1),
	}

	return ProposeResult{
		Skeletons: newSkeletons,
		Metadata:  Metadata{IslandID: req.IslandID},
	}
}

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
			return ProposeResult{Err: err}
		}

		skeletons := make([]ProgramSkeleton, len(resp.Skeletons))
		for i, s := range resp.Skeletons {
			skeletons[i] = ProgramSkeleton(s)
		}

		return ProposeResult{
			Skeletons: skeletons,
			Metadata:  Metadata{IslandID: req.IslandID},
		}
	}
}
