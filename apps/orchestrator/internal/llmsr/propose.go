package llmsr

import (
	"context"
	"strconv"
	"time"
)

// MockPropose generates a predictable set of new skeletons for deterministic testing.
func MockPropose(ctx context.Context, req ProposeRequest) ProposeResult {
	time.Sleep(1 * time.Millisecond)

	parentSkeleton := req.Parents[0].Skeleton
	val, err := strconv.Atoi(parentSkeleton)
	if err != nil {
		return ProposeResult{Err: err}
	}

	newSkeletons := []ProgramSkeleton{
		strconv.Itoa(val - 1),
		strconv.Itoa(val + 10),
	}

	return ProposeResult{
		Skeletons: newSkeletons,
		Metadata:  Metadata{IslandID: req.IslandID},
	}
}
