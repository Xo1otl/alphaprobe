package llmsr

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
)

const (
	T0      = 1.0
	N       = 100
	Tp      = 1.0
	Epsilon = 1e-6
)

func (s *State) scoreToKey(score Score) string {
	return strconv.FormatFloat(score, 'f', s.ScoreQuantization, 64)
}

type Cluster struct {
	Score    Score
	Programs []*Program
}

type Island struct {
	ID               int
	Clusters         map[string]*Cluster
	EvaluationsCount int
	CullingCount     int
	BestProgram      *Program
}

func (i *Island) addProgram(p *Program, scoreToKey func(Score) string) {
	i.EvaluationsCount++
	if p.isBetterThan(i.BestProgram) {
		i.BestProgram = p
	}
	key := scoreToKey(p.Score)
	if cluster, ok := i.Clusters[key]; ok {
		cluster.Programs = append(cluster.Programs, p)
	} else {
		i.Clusters[key] = &Cluster{Score: p.Score, Programs: []*Program{p}}
	}
}

func (i *Island) resetWithElite(elite *Program, scoreToKey func(Score) string) {
	key := scoreToKey(elite.Score)
	i.Clusters = map[string]*Cluster{key: {Score: elite.Score, Programs: []*Program{elite}}}
	i.EvaluationsCount = 0
	i.CullingCount++
	i.BestProgram = elite
}

func (island *Island) getBestScore() Score      { return island.BestProgram.Score }
func (island *Island) getBestProgram() *Program { return island.BestProgram }

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

func NewState(initialSkeleton ProgramSkeleton, maxEvaluations, numIslands, migrationInterval, scoreQuantization int, fatal func(err error)) (*State, error) {
	initialScoreVal, err := strconv.ParseFloat(string(initialSkeleton), 64)
	if err != nil {
		return nil, err
	}
	initialScore := Score(initialScoreVal)

	s := &State{
		Islands:               make(map[int]*Island, numIslands),
		MaxEvaluations:        maxEvaluations,
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
			ID:          i,
			Clusters:    map[string]*Cluster{initialKey: cluster},
			BestProgram: program,
		}
	}
	return s, nil
}

func (s *State) Update(res ObserveResult) (done bool) {
	if res.Err != nil {
		s.Fatal(fmt.Errorf("error in observation: %v", res.Err))
	}
	s.EvaluationsCount++

	island, ok := s.Islands[res.Metadata.IslandID]
	if !ok {
		s.Fatal(fmt.Errorf("%w: island with ID %d", ErrIslandNotFound, res.Metadata.IslandID))
	}

	program := &Program{Skeleton: res.Query, Score: res.Evidence}
	island.addProgram(program, s.scoreToKey)

	if s.EvaluationsCount >= s.NextMigration {
		s.manageIslands()
		s.NextMigration += s.MigrationInterval
	}
	return s.EvaluationsCount >= s.MaxEvaluations
}

func (s *State) NewRequest() (ProposeRequest, bool) {
	islandIDs := make([]int, 0, len(s.Islands))
	for id := range s.Islands {
		islandIDs = append(islandIDs, id)
	}
	if len(islandIDs) == 0 {
		return ProposeRequest{}, false
	}
	randomID := islandIDs[rand.Intn(len(islandIDs))]
	island := s.Islands[randomID]

	// FATAL CHECK: Logic flaw if an empty island is chosen while others are not.
	if len(island.Clusters) == 0 {
		for _, otherIsland := range s.Islands {
			if len(otherIsland.Clusters) > 0 {
				s.Fatal(fmt.Errorf("%w: island %d", ErrEmptyIslandSelected, island.ID))
				break
			}
		}
	}

	return ProposeRequest{
		Parents:  []*Program{s.selectParent(island), s.selectParent(island)},
		IslandID: island.ID,
	}, true
}

func (s *State) selectParent(island *Island) *Program {
	selectedCluster := s.selectCluster(island)
	return s.selectProgramFromCluster(selectedCluster, island.ID)
}

func (s *State) selectCluster(island *Island) *Cluster {
	if len(island.Clusters) == 0 {
		s.Fatal(fmt.Errorf("%w: island %d", ErrSelectionFromEmptyIsland, island.ID))
	}

	clusters := make([]*Cluster, 0, len(island.Clusters))
	for _, cluster := range island.Clusters {
		clusters = append(clusters, cluster)
	}

	tc := T0*(1-float64(island.EvaluationsCount%N)/float64(N)) + Epsilon
	maxScore := island.getBestScore()

	clusterWeightFunc := func(c *Cluster) float64 {
		return math.Exp((c.Score - maxScore) / tc)
	}
	selectedCluster, err := weightedChoice(clusters, clusterWeightFunc)
	if err != nil {
		s.Fatal(fmt.Errorf("cluster selection failed in island %d: %w", island.ID, err))
	}
	return selectedCluster
}

func (s *State) selectProgramFromCluster(cluster *Cluster, islandID int) *Program {
	programs := cluster.Programs
	if len(programs) == 0 {
		s.Fatal(fmt.Errorf("%w in island %d", ErrInvalidCluster, islandID))
	}
	if len(programs) == 1 {
		return programs[0]
	}

	minLength, maxLength := math.MaxInt32, 0
	for _, p := range programs {
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
		s.Fatal(fmt.Errorf("program selection failed from cluster with score %f in island %d: %w", cluster.Score, islandID, err))
	}

	return selectedProgram
}

func (s *State) manageIslands() {
	if len(s.Islands) <= s.NumIslandsToEliminate {
		return
	}

	sortedIslands := make([]*Island, 0, len(s.Islands))
	for _, island := range s.Islands {
		sortedIslands = append(sortedIslands, island)
	}
	sort.Slice(sortedIslands, func(i, j int) bool {
		return sortedIslands[i].getBestScore() > sortedIslands[j].getBestScore()
	})

	numSurvivors := len(sortedIslands) - s.NumIslandsToEliminate
	elites := make([]*Program, 0, numSurvivors)
	for _, island := range sortedIslands[:numSurvivors] {
		bestProgram := island.getBestProgram()
		if bestProgram == nil {
			s.Fatal(fmt.Errorf("%w: island %d", ErrEmptySurvivorIsland, island.ID))
		}
		elites = append(elites, bestProgram)
	}

	if len(elites) == 0 {
		s.Fatal(fmt.Errorf("%w", ErrNoElitesFound))
	}

	for _, islandToReplace := range sortedIslands[numSurvivors:] {
		elite := elites[rand.Intn(len(elites))]
		islandToReplace.resetWithElite(elite, s.scoreToKey)
	}
}

// getBestScore finds the highest score among the best programs of all islands.
func (s *State) getBestScore() Score {
	bestScore := Score(-1e9) // Start with a very low score
	for _, island := range s.Islands {
		islandBest := island.getBestScore()
		if islandBest > bestScore {
			bestScore = islandBest
		}
	}
	return bestScore
}

func weightedChoice[T any](items []T, getWeight func(T) float64) (T, error) {
	var zero T
	if len(items) == 0 {
		return zero, ErrSelectionFromEmptySlice
	}

	weights := make([]float64, len(items))
	sumWeights := 0.0
	for i, item := range items {
		w := getWeight(item)
		if w < 0 {
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
	return items[len(items)-1], nil
}
