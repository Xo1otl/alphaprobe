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
	Err      error // Proposeで発生したエラーを伝搬するためのフィールド
}

type ObserveResult struct {
	Query    ProgramSkeleton
	Evidence Score
	Metadata Metadata
	Err      error // Proposeで発生したエラーか、Observeで発生したエラーが入る
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
