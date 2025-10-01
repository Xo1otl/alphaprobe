package llmsr

import "errors"

var (
	// ErrInObserve is returned when an error occurs during the observe phase.
	ErrInObserve = errors.New("error occurred in observe phase")
	// ErrInPropose is returned when an error occurs during the propose phase.
	ErrInPropose = errors.New("error occurred in propose phase")

	// --- State Management Errors ---

	// ErrIslandNotFound indicates that an operation was attempted on a non-existent island.
	ErrIslandNotFound = errors.New("island not found")
	// ErrEmptyIslandSelected is a critical error indicating a logic flaw where an empty island
	// was chosen for parent selection while other populated islands exist.
	ErrEmptyIslandSelected = errors.New("selected an empty island while others are populated")
	// ErrSelectionFromEmptyIsland occurs when trying to select a parent from an island with no programs.
	ErrSelectionFromEmptyIsland = errors.New("parent selection from an empty island")
	// ErrEmptySurvivorIsland occurs during migration if a surviving island is found to be empty.
	ErrEmptySurvivorIsland = errors.New("surviving island is empty during migration")
	// ErrNoElitesFound indicates that no elite individuals could be identified from surviving islands,
	// which should be an impossible state.
	ErrNoElitesFound = errors.New("no elites found from surviving islands")

	// --- Selection Process Errors ---

	// ErrClusterSelectionFailed indicates a failure to select a cluster, often due to a flaw
	// in the probability calculation logic.
	ErrClusterSelectionFailed = errors.New("cluster selection failed")
	// ErrProgramSelectionFailed indicates a failure to select a program from a chosen cluster.
	ErrProgramSelectionFailed = errors.New("program selection failed")
	// ErrInvalidCluster is raised when a selected cluster is found to be nil or empty.
	ErrInvalidCluster = errors.New("selected cluster is invalid (nil or empty)")

	// --- Numerical Stability Errors ---

	// ErrNumericalInstability is returned when a numerical issue, such as underflow leading to
	// zero probabilities, is detected during calculations.
	ErrNumericalInstability = errors.New("numerical instability detected")
)