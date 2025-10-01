package llmsr

// --- Types for bilevel Runner ---

type ProposeRequest struct {
	Parents  []*Program
	IslandID int
}

type ProposeResult struct {
	Skeletons []ProgramSkeleton
	Metadata  Metadata
	Err       error
}

type ObserveRequest struct {
	Query    ProgramSkeleton
	Metadata Metadata
	Err      error
}

type ObserveResult struct {
	Query    ProgramSkeleton
	Evidence Score
	Metadata Metadata
	Err      error
}

type Metadata struct {
	IslandID int
}

type ProgramSkeleton = string

type Score = float64

type Program struct {
	Skeleton ProgramSkeleton
	Score    Score
}

// isBetterThan compares program p with other, returning true if p is strictly better.
// Tie-breaking is done by shorter length, then lexicographically.
func (p *Program) isBetterThan(other *Program) bool {
	if p.Score != other.Score {
		return p.Score > other.Score
	}
	if len(p.Skeleton) != len(other.Skeleton) {
		return len(p.Skeleton) < len(other.Skeleton)
	}
	return p.Skeleton < other.Skeleton
}
