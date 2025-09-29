package llmsr

import (
	"context"
	"strconv"
)

// MockObserve provides a deterministic, predictable score based on the skeleton's content.
func MockObserve(ctx context.Context, req ObserveRequest) ObserveResult {
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
