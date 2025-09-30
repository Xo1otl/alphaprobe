package llmsr

import (
	"log"
	"math"
	"math/rand"
	"sort"
)

const (
	// T0 is the initial temperature for Boltzmann selection.
	T0 = 1.0
	// N is the total number of individuals to be evaluated before the island is considered for replacement.
	N = 100
	// Tp is a temperature parameter for skeleton selection, typically fixed to 1.
	Tp = 1.0
	// Epsilon is a small constant to prevent division by zero.
	Epsilon = 1e-6
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
	ID               int
	Clusters         map[Score]*Cluster
	EvaluationsCount int
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
			ID:               i,
			Clusters:         make(map[Score]*Cluster),
			EvaluationsCount: 0,
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
	island.EvaluationsCount++ // Increment island-specific evaluation count
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

	// Select one island to perform the evolutionary step, as per the README.
	island := s.Islands[rand.Intn(len(s.Islands))]

	parentA := s.selectParent(island)
	parentB := s.selectParent(island)

	return ProposeRequest{
		Parents:  []*Program{parentA, parentB},
		IslandID: island.ID,
	}, true
}

func (s *State) selectParent(island *Island) *Program {
	if len(island.Clusters) == 0 {
		return &Program{Skeleton: s.InitialSkeleton}
	}

	// 1. Cluster Selection (Score-based)
	clusters := make([]*Cluster, 0, len(island.Clusters))
	for _, cluster := range island.Clusters {
		clusters = append(clusters, cluster)
	}

	tc := T0 * (1 - float64(island.EvaluationsCount%N)/float64(N))
	if tc < Epsilon {
		tc = Epsilon
	}

	probabilities := make([]float64, len(clusters))
	sumExp := 0.0
	for i, cluster := range clusters {
		expVal := math.Exp(cluster.Score / tc)
		probabilities[i] = expVal
		sumExp += expVal
	}

	if sumExp == 0 {
		sumExp = Epsilon
	}

	for i := range probabilities {
		probabilities[i] /= sumExp
	}

	randVal := rand.Float64()
	cumulativeProb := 0.0
	var selectedCluster *Cluster
	for i, prob := range probabilities {
		cumulativeProb += prob
		if randVal <= cumulativeProb {
			selectedCluster = clusters[i]
			break
		}
	}
	if selectedCluster == nil {
		selectedCluster = clusters[len(clusters)-1]
	}

	// 2. Skeleton Selection (Length-based)
	if len(selectedCluster.Programs) == 0 {
		return &Program{Skeleton: s.InitialSkeleton}
	}

	minLength := len(selectedCluster.Programs[0].Skeleton)
	maxLength := len(selectedCluster.Programs[0].Skeleton)
	for _, program := range selectedCluster.Programs {
		length := len(program.Skeleton)
		if length < minLength {
			minLength = length
		}
		if length > maxLength {
			maxLength = length
		}
	}

	skelProbs := make([]float64, len(selectedCluster.Programs))
	sumExpSkel := 0.0
	for i, program := range selectedCluster.Programs {
		normalizedLength := float64(len(program.Skeleton)-minLength) / (float64(maxLength-minLength) + Epsilon)
		expVal := math.Exp(-normalizedLength / Tp)
		skelProbs[i] = expVal
		sumExpSkel += expVal
	}

	if sumExpSkel == 0 {
		sumExpSkel = Epsilon
	}

	for i := range skelProbs {
		skelProbs[i] /= sumExpSkel
	}

	randValSkel := rand.Float64()
	cumulativeProbSkel := 0.0
	var selectedProgram *Program
	for i, prob := range skelProbs {
		cumulativeProbSkel += prob
		if randValSkel <= cumulativeProbSkel {
			selectedProgram = selectedCluster.Programs[i]
			break
		}
	}
	if selectedProgram == nil {
		selectedProgram = selectedCluster.Programs[len(selectedCluster.Programs)-1]
	}

	return selectedProgram
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
