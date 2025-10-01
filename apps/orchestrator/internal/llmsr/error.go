package llmsr

import "errors"

var (
	ErrInObserve = errors.New("error occurred in observe phase")
	ErrInPropose = errors.New("error occurred in propose phase")

	// --- State Management Errors ---

	ErrInvalidEliminationRate = errors.New("invalid elimination rate, must be in [0, 1)")
	ErrIslandNotFound = errors.New("island not found")
	ErrEmptyIslandSelected = errors.New("selected an empty island while others are populated")
	ErrSelectionFromEmptyIsland = errors.New("parent selection from an empty island")
	ErrEmptySurvivorIsland = errors.New("surviving island is empty during migration")
	ErrNoElitesFound = errors.New("no elites found from surviving islands")

	// --- Selection Process Errors ---

	ErrClusterSelectionFailed = errors.New("cluster selection failed")
	ErrProgramSelectionFailed = errors.New("program selection failed")
	ErrInvalidCluster = errors.New("selected cluster is invalid (nil or empty)")
	ErrSelectionFromEmptySlice = errors.New("selection from empty slice")
	ErrNegativeWeight = errors.New("negative weight provided for selection")

	// --- Numerical Stability Errors ---

	ErrNumericalInstability = errors.New("numerical instability detected")
)