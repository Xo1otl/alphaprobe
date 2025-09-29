package llmsr

import (
	"log"
	"math/rand"
	"sort"
)

// ProgramSkeleton represents the symbolic structure of an equation.
type ProgramSkeleton = string

// Score represents the quantitative evaluation of a program.
type Score = float64

// Program is an equation skeleton with an assigned score.
type Program struct {
	Skeleton ProgramSkeleton
	Score    Score
}

// Cluster groups programs with identical scores.
type Cluster struct {
	Score    Score
	Programs []*Program
}

// Island holds a population of programs organized into clusters.
type Island struct {
	ID       int
	Clusters map[Score]*Cluster
}

// State manages the entire population across all islands.
type State struct {
	Islands               []*Island
	MaxEvaluations        int
	EvaluationsCount      int
	MigrationInterval     int
	NextMigration         int
	InitialSkeleton       ProgramSkeleton
	NumIslandsToEliminate int
	Logger                *log.Logger
}

// NewState creates a new initial state for the GA.
func NewState(initialSkeleton ProgramSkeleton, maxEvaluations, numIslands, migrationInterval int, logger *log.Logger) *State {
	islands := make([]*Island, numIslands)
	for i := range numIslands {
		islands[i] = &Island{
			ID:       i,
			Clusters: make(map[Score]*Cluster),
		}
	}

	return &State{
		Islands:               islands,
		MaxEvaluations:        maxEvaluations,
		EvaluationsCount:      0,
		MigrationInterval:     migrationInterval,
		NextMigration:         migrationInterval,
		InitialSkeleton:       initialSkeleton,
		NumIslandsToEliminate: numIslands / 2,
		Logger:                logger,
	}
}

// Update incorporates an observation result into the state.
func (s *State) Update(res ObserveResult) (done bool) {
	if res.Err != nil {
		s.Logger.Printf("error in observation: %v", res.Err)
		return false
	}
	s.EvaluationsCount++

	island := s.Islands[res.Metadata.IslandID]
	program := &Program{Skeleton: res.Query, Score: res.Evidence}

	if cluster, ok := island.Clusters[program.Score]; ok {
		cluster.Programs = append(cluster.Programs, program)
	} else {
		island.Clusters[program.Score] = &Cluster{
			Score:    program.Score,
			Programs: []*Program{program},
		}
	}

	if s.EvaluationsCount >= s.NextMigration {
		s.manageIslands()
		s.NextMigration += s.MigrationInterval
	}
	return s.EvaluationsCount >= s.MaxEvaluations
}

// NewRequest generates a new ProposeRequest.
func (s *State) NewRequest() (ProposeRequest, bool) {
	// Special handling for the very first request to ensure the process starts correctly.
	if s.EvaluationsCount == 0 {
		return ProposeRequest{
			Parents:  []*Program{{Skeleton: s.InitialSkeleton}, {Skeleton: s.InitialSkeleton}},
			IslandID: rand.Intn(len(s.Islands)),
		}, true
	}

	// Select a random island to contribute to genetic diversity.
	island := s.Islands[rand.Intn(len(s.Islands))]

	// Collect a pool of potential parent programs from the island's clusters.
	parentsPool := make([]*Program, 0, len(island.Clusters))
	for _, cluster := range island.Clusters {
		if len(cluster.Programs) > 0 {
			// Add a random program from each cluster to the pool.
			parentsPool = append(parentsPool, cluster.Programs[rand.Intn(len(cluster.Programs))])
		}
	}

	// If the pool has fewer than two parents, fall back to the initial skeleton to prevent stalling.
	// This is crucial for islands that may have been reset or are sparsely populated.
	if len(parentsPool) < 2 {
		return ProposeRequest{
			Parents:  []*Program{{Skeleton: s.InitialSkeleton}, {Skeleton: s.InitialSkeleton}},
			IslandID: island.ID,
		}, true
	}

	// Shuffle the pool and select two distinct parents for crossover.
	rand.Shuffle(len(parentsPool), func(i, j int) { parentsPool[i], parentsPool[j] = parentsPool[j], parentsPool[i] })
	return ProposeRequest{
		Parents:  []*Program{parentsPool[0], parentsPool[1]},
		IslandID: island.ID,
	}, true
}

func (s *State) manageIslands() {
	if len(s.Islands) <= 1 {
		return
	}
	sort.Slice(s.Islands, func(i, j int) bool {
		return s.Islands[i].getBestScore() < s.Islands[j].getBestScore()
	})
	elites := make([]*Program, 0)
	for i := 0; i < len(s.Islands)-s.NumIslandsToEliminate; i++ {
		elites = append(elites, s.Islands[i].getBestProgram())
	}
	if len(elites) == 0 {
		return
	}
	for i := 0; i < s.NumIslandsToEliminate; i++ {
		newIsland := &Island{
			ID:       s.Islands[len(s.Islands)-1-i].ID,
			Clusters: make(map[Score]*Cluster),
		}
		elite := elites[rand.Intn(len(elites))]
		newIsland.Clusters[elite.Score] = &Cluster{Score: elite.Score, Programs: []*Program{elite}}
		s.Islands[len(s.Islands)-1-i] = newIsland
	}
}

func (island *Island) getBestScore() Score {
	bestScore := 1e9
	for score := range island.Clusters {
		if score < bestScore {
			bestScore = score
		}
	}
	return bestScore
}

func (island *Island) getBestProgram() *Program {
	bestScore := 1e9
	var bestProgram *Program
	for score, cluster := range island.Clusters {
		if score < bestScore && len(cluster.Programs) > 0 {
			bestScore = score
			bestProgram = cluster.Programs[0]
		}
	}
	return bestProgram
}

func (s *State) getBestScore() Score {
	bestScore := 1e9
	for _, island := range s.Islands {
		islandBest := island.getBestScore()
		if islandBest < bestScore {
			bestScore = islandBest
		}
	}
	return bestScore
}

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
