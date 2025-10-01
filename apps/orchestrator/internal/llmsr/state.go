package llmsr

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
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

// scoreToKey converts a float64 score into a consistent string representation suitable for map keys,
// applying quantization based on the state's settings.
func (s *State) scoreToKey(score Score) string {
	return strconv.FormatFloat(score, 'f', s.ScoreQuantization, 64)
}

// Cluster groups programs with identical scores.
type Cluster struct {
	Score    Score
	Programs []*Program
}

// Island holds a population of programs organized into clusters.
type Island struct {
	ID               int
	Clusters         map[string]*Cluster
	EvaluationsCount int
	CullingCount     int
	BestProgram      *Program // Cache the best program
}

// State manages the entire population across all islands.
type State struct {
	Islands               map[int]*Island
	MaxEvaluations        int
	EvaluationsCount      int
	MigrationInterval     int
	NextMigration         int
	InitialSkeleton       ProgramSkeleton
	NumIslandsToEliminate int
	ScoreQuantization     int
	Fatal                 func(err error)
}

// NewState creates a new initial state for the GA.
func NewState(initialSkeleton ProgramSkeleton, maxEvaluations, numIslands, migrationInterval, scoreQuantization int, fatal func(err error)) (*State, error) {
	initialScoreVal, err := strconv.ParseFloat(string(initialSkeleton), 64)
	if err != nil {
		return nil, err
	}
	initialScore := Score(initialScoreVal)

	s := &State{
		Islands:               make(map[int]*Island, numIslands),
		MaxEvaluations:        maxEvaluations,
		EvaluationsCount:      0,
		MigrationInterval:     migrationInterval,
		NextMigration:         migrationInterval,
		InitialSkeleton:       initialSkeleton,
		NumIslandsToEliminate: numIslands / 2,
		ScoreQuantization:     scoreQuantization,
		Fatal:                 fatal,
	}

	for i := range numIslands {
		program := &Program{Skeleton: initialSkeleton, Score: initialScore}
		cluster := &Cluster{Score: initialScore, Programs: []*Program{program}}
		initialKey := s.scoreToKey(initialScore)
		s.Islands[i] = &Island{
			ID:               i,
			Clusters:         map[string]*Cluster{initialKey: cluster},
			EvaluationsCount: 0,
			CullingCount:     0,
			BestProgram:      program, // Initialize the cache
		}
	}

	return s, nil
}

// Update incorporates an observation result into the state.
func (s *State) Update(res ObserveResult) (done bool) {
	if res.Err != nil {
		s.Fatal(fmt.Errorf("error in observation: %v", res.Err))
	}
	s.EvaluationsCount++

	island, ok := s.Islands[res.Metadata.IslandID]
	if !ok {
		s.Fatal(fmt.Errorf("%w: island with ID %d", ErrIslandNotFound, res.Metadata.IslandID))
	}

	island.EvaluationsCount++ // Increment island-specific evaluation count
	program := &Program{Skeleton: res.Query, Score: res.Evidence}

	// Update the cache if the new program is better
	if program.isBetterThan(island.BestProgram) {
		island.BestProgram = program
	}

	key := s.scoreToKey(program.Score)

	if cluster, ok := island.Clusters[key]; ok {
		cluster.Programs = append(cluster.Programs, program)
	} else {
		island.Clusters[key] = &Cluster{
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
	// Get island IDs to select one randomly
	islandIDs := make([]int, 0, len(s.Islands))
	for id := range s.Islands {
		islandIDs = append(islandIDs, id)
	}
	if len(islandIDs) == 0 {
		return ProposeRequest{}, false // No islands to select from
	}
	randomID := islandIDs[rand.Intn(len(islandIDs))]
	island := s.Islands[randomID]

	// FATAL CHECK: If we select an empty island while others are populated,
	// it's a critical logic flaw that resets progress.
	if len(island.Clusters) == 0 {
		isAnyIslandPopulated := false
		for _, otherIsland := range s.Islands {
			if len(otherIsland.Clusters) > 0 {
				isAnyIslandPopulated = true
				break
			}
		}
		if isAnyIslandPopulated {
			s.Fatal(fmt.Errorf("%w: island %d", ErrEmptyIslandSelected, island.ID))
		}
	}

	parentA := s.selectParent(island)
	parentB := s.selectParent(island)

	return ProposeRequest{
		Parents:  []*Program{parentA, parentB},
		IslandID: island.ID,
	}, true
}

func (s *State) selectParent(island *Island) *Program {
	if len(island.Clusters) == 0 {
		s.Fatal(fmt.Errorf("%w: island %d", ErrSelectionFromEmptyIsland, island.ID))
	}

	// 1. Cluster Selection (Score-based)
	clusters := make([]*Cluster, 0, len(island.Clusters))
	for _, cluster := range island.Clusters {
		clusters = append(clusters, cluster)
	}

	tc := T0*(1-float64(island.EvaluationsCount%N)/float64(N)) + Epsilon
	maxScore := island.getBestScore() // More efficient way to get max score

	clusterWeightFunc := func(c *Cluster) float64 {
		return math.Exp((c.Score - maxScore) / tc)
	}
	selectedCluster, err := weightedChoice(clusters, clusterWeightFunc)
	if err != nil {
		s.Fatal(fmt.Errorf("cluster selection failed in island %d: %w", island.ID, err))
	}

	// 2. Skeleton Selection (Length-based)
	programs := selectedCluster.Programs
	if len(programs) == 0 {
		s.Fatal(fmt.Errorf("%w in island %d", ErrInvalidCluster, island.ID))
	}

	minLength := len(programs[0].Skeleton)
	maxLength := len(programs[0].Skeleton)
	for _, p := range programs[1:] {
		l := len(p.Skeleton)
		if l < minLength {
			minLength = l
		}
		if l > maxLength {
			maxLength = l
		}
	}

	lengthRange := float64(maxLength-minLength) + Epsilon
	skeletonWeightFunc := func(p *Program) float64 {
		normalizedLength := float64(len(p.Skeleton)-minLength) / lengthRange
		return math.Exp(-normalizedLength / Tp)
	}
	selectedProgram, err := weightedChoice(programs, skeletonWeightFunc)
	if err != nil {
		s.Fatal(fmt.Errorf("program selection failed from cluster with score %f in island %d: %w", selectedCluster.Score, island.ID, err))
	}

	return selectedProgram
}

func (s *State) manageIslands() {
	if len(s.Islands) <= 1 {
		return
	}

	// Convert map to slice for sorting
	sortedIslands := make([]*Island, 0, len(s.Islands))
	for _, island := range s.Islands {
		sortedIslands = append(sortedIslands, island)
	}

	sort.Slice(sortedIslands, func(i, j int) bool {
		return sortedIslands[i].getBestScore() > sortedIslands[j].getBestScore()
	})

	// Identify elites from surviving islands
	elites := make([]*Program, 0)
	numSurvivors := len(sortedIslands) - s.NumIslandsToEliminate
	for i := range numSurvivors {
		bestProgram := sortedIslands[i].getBestProgram()
		if bestProgram == nil {
			s.Fatal(fmt.Errorf("%w: island %d", ErrEmptySurvivorIsland, sortedIslands[i].ID))
		}
		elites = append(elites, bestProgram)
	}

	if len(elites) == 0 {
		s.Fatal(fmt.Errorf("%w: this should be impossible", ErrNoElitesFound))
	}

	// Replace the worst-performing islands
	for i := numSurvivors; i < len(sortedIslands); i++ {
		islandToReplace := sortedIslands[i]
		elite := elites[rand.Intn(len(elites))]
		key := s.scoreToKey(elite.Score)
		// Preserve and increment the culling count from the old island instance
		newCullingCount := s.Islands[islandToReplace.ID].CullingCount + 1
		s.Islands[islandToReplace.ID] = &Island{
			ID:               islandToReplace.ID,
			Clusters:         map[string]*Cluster{key: {Score: elite.Score, Programs: []*Program{elite}}},
			CullingCount:     newCullingCount,
			EvaluationsCount: 0,
			BestProgram:      elite, // Seed the cache with the elite
		}
	}
}

func (island *Island) getBestScore() Score {
	return island.BestProgram.Score
}

func (island *Island) getBestProgram() *Program {
	return island.BestProgram
}

func (s *State) getBestScore() Score {
	bestScore := Score(-1e9)
	for _, island := range s.Islands {
		islandBest := island.getBestScore()
		if islandBest > bestScore {
			bestScore = islandBest
		}
	}
	return bestScore
}

// weightedChoice performs a weighted random selection from a slice of items.
// It takes a slice and a function that returns the weight for each item.
func weightedChoice[T any](items []T, getWeight func(T) float64) (T, error) {
	var zero T
	if len(items) == 0 {
		return zero, ErrSelectionFromEmptySlice
	}

	weights := make([]float64, len(items))
	sumWeights := 0.0
	for i, item := range items {
		w := getWeight(item)
		if w < 0 { // Weights must be non-negative
			return zero, ErrNegativeWeight
		}
		weights[i] = w
		sumWeights += w
	}

	if sumWeights <= Epsilon {
		return zero, ErrNumericalInstability
	}

	randVal := rand.Float64() * sumWeights
	cumulativeWeight := 0.0
	for i, w := range weights {
		cumulativeWeight += w
		if randVal <= cumulativeWeight {
			return items[i], nil
		}
	}

	return items[len(items)-1], nil // Fallback for floating-point inaccuracies
}