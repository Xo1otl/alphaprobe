package llmsr

import (
	"log"
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

// scoreToKey converts a float64 score into a consistent string representation suitable for map keys.
func scoreToKey(score Score) string {
	return strconv.FormatFloat(score, 'f', -1, 64)
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
	Logger                *log.Logger
}

// NewState creates a new initial state for the GA.
func NewState(initialSkeleton ProgramSkeleton, maxEvaluations, numIslands, migrationInterval int, logger *log.Logger) *State {
	initialScoreVal, err := strconv.ParseFloat(string(initialSkeleton), 64)
	if err != nil {
		logger.Fatalf("Could not parse initial skeleton '%s' into a float score: %v", initialSkeleton, err)
	}
	initialScore := Score(initialScoreVal)

	islands := make(map[int]*Island, numIslands)
	for i := range numIslands {
		program := &Program{Skeleton: initialSkeleton, Score: initialScore}
		cluster := &Cluster{Score: initialScore, Programs: []*Program{program}}
		initialKey := scoreToKey(initialScore)
		islands[i] = &Island{
			ID:               i,
			Clusters:         map[string]*Cluster{initialKey: cluster},
			EvaluationsCount: 0,
			CullingCount:     0,
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

	island, ok := s.Islands[res.Metadata.IslandID]
	if !ok {
		s.Logger.Printf("error: island with ID %d not found", res.Metadata.IslandID)
		return false
	}

	island.EvaluationsCount++ // Increment island-specific evaluation count
	program := &Program{Skeleton: res.Query, Score: res.Evidence}
	key := scoreToKey(program.Score)

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
			s.Logger.Printf("FATAL: Selected an empty island (%d) while other islands are populated. This is an invalid state that resets evolutionary progress.", island.ID)
			return ProposeRequest{}, false // Stop the process
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
		s.Logger.Fatalf("FATAL: selectParent called on an empty island (ID: %d). This should not happen.", island.ID)
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

	// Find max score for numerical stability
	maxScore := clusters[0].Score
	for _, c := range clusters[1:] {
		if c.Score > maxScore {
			maxScore = c.Score
		}
	}

	probabilities := make([]float64, len(clusters))
	sumExp := 0.0
	for i, cluster := range clusters {
		expVal := math.Exp((cluster.Score - maxScore) / tc)
		probabilities[i] = expVal
		sumExp += expVal
	}

	if sumExp == 0 {
		s.Logger.Fatalf("FATAL: sumExp is zero during cluster selection in island %d. All selection probabilities are zero, possibly due to score underflow.", island.ID)
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
		s.Logger.Fatalf("FATAL: Failed to select a cluster in island %d. This indicates a flaw in the probability calculation.", island.ID)
	}

	// 2. Skeleton Selection (Length-based)
	if len(selectedCluster.Programs) == 0 {
		s.Logger.Fatalf("FATAL: Selected cluster is empty (Island ID: %d, Score: %f). This indicates a logic flaw.", island.ID, selectedCluster.Score)
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
		s.Logger.Fatalf("FATAL: sumExpSkel is zero during skeleton selection in island %d, cluster score %f.", island.ID, selectedCluster.Score)
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
		s.Logger.Fatalf("FATAL: Failed to select a program from cluster in island %d, cluster score %f.", island.ID, selectedCluster.Score)
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
	for i := 0; i < numSurvivors; i++ {
		bestProgram := sortedIslands[i].getBestProgram()
		if bestProgram == nil {
			s.Logger.Printf("FATAL: Surviving island %d is empty during migration. This indicates a fundamental logic error.", sortedIslands[i].ID)
			return
		}
		elites = append(elites, bestProgram)
	}

	if len(elites) == 0 {
		s.Logger.Print("FATAL: No elites found from surviving islands. This should be impossible.")
		return
	}

	// Replace the worst-performing islands
	for i := numSurvivors; i < len(sortedIslands); i++ {
		islandToReplace := sortedIslands[i]
		elite := elites[rand.Intn(len(elites))]
		key := scoreToKey(elite.Score)
		// Preserve and increment the culling count from the old island instance
		newCullingCount := s.Islands[islandToReplace.ID].CullingCount + 1
		s.Islands[islandToReplace.ID] = &Island{
			ID:               islandToReplace.ID,
			Clusters:         map[string]*Cluster{key: {Score: elite.Score, Programs: []*Program{elite}}},
			CullingCount:     newCullingCount,
			EvaluationsCount: 0,
		}
	}
}

func (island *Island) getBestScore() Score {
	bestScore := Score(-1e9)
	for _, cluster := range island.Clusters {
		if cluster.Score > bestScore {
			bestScore = cluster.Score
		}
	}
	return bestScore
}

func (island *Island) getBestProgram() *Program {
	bestScore := Score(-1e9)
	var bestProgram *Program
	for _, cluster := range island.Clusters {
		if cluster.Score > bestScore && len(cluster.Programs) > 0 {
			bestScore = cluster.Score
			// To be deterministic, we should have a canonical way to select.
			// For now, just picking the first one is fine.
			bestProgram = cluster.Programs[0]
		}
	}
	return bestProgram
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
