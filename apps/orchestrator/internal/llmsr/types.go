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
