package llmsr

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
)

type State struct {
	Islands               map[int]*Island
	MaxEvaluations        int
	EvaluationsCount      int
	MigrationInterval     int
	NextMigration         int
	InitialSkeleton       Skeleton
	NumIslandsToEliminate int
	ScoreQuantization     int
}

func NewState(initialSkeleton Skeleton, initialScore ProgramScore, maxEvaluations, numIslands, migrationInterval, scoreQuantization int, eliminationRate float64) (*State, error) {
	if eliminationRate < 0 || eliminationRate >= 1 {
		return nil, fmt.Errorf("%w", ErrInvalidEliminationRate)
	}
	s := &State{
		Islands:               make(map[int]*Island, numIslands),
		MaxEvaluations:        maxEvaluations,
		MigrationInterval:     migrationInterval,
		NextMigration:         migrationInterval,
		InitialSkeleton:       initialSkeleton,
		NumIslandsToEliminate: int(float64(numIslands) * eliminationRate),
		ScoreQuantization:     scoreQuantization,
	}

	initialClusterScore, err := quantize(initialScore, s.ScoreQuantization)
	if err != nil {
		return nil, err
	}

	for i := range numIslands {
		program := &Program{Skeleton: initialSkeleton, Score: initialScore}
		cluster := &Cluster{Score: initialClusterScore, Programs: []*Program{program}}
		s.Islands[i] = &Island{
			ID:             i,
			Clusters:       map[ClusterScore]*Cluster{initialClusterScore: cluster},
			PopulationSize: 1,
			BestProgram:    program,
		}
	}
	return s, nil
}

func (s *State) Update(res ObserveResult) (done bool, err error) {
	if res.Err != nil {
		return true, fmt.Errorf("error in observation: %v", res.Err)
	}
	s.EvaluationsCount++

	island, ok := s.Islands[res.Metadata.IslandID]
	if !ok {
		return true, fmt.Errorf("%w: island with ID %d", ErrIslandNotFound, res.Metadata.IslandID)
	}

	program := &Program{Skeleton: res.Query, Score: res.Evidence}
	if err := island.addProgram(program, s.ScoreQuantization); err != nil {
		return true, fmt.Errorf("failed to add program: %w", err)
	}

	if s.EvaluationsCount >= s.NextMigration {
		if err := s.manageIslands(); err != nil {
			return true, fmt.Errorf("failed to manage islands: %w", err)
		}
		s.NextMigration += s.MigrationInterval
	}
	return s.EvaluationsCount >= s.MaxEvaluations, nil
}

func (s *State) NewRequest() (ProposeRequest, bool, error) {
	islandIDs := make([]int, 0, len(s.Islands))
	for id := range s.Islands {
		islandIDs = append(islandIDs, id)
	}
	if len(islandIDs) == 0 {
		return ProposeRequest{}, false, nil
	}
	randomID := islandIDs[rand.Intn(len(islandIDs))]
	island := s.Islands[randomID]

	if len(island.Clusters) == 0 {
		for _, otherIsland := range s.Islands {
			if len(otherIsland.Clusters) > 0 {
				return ProposeRequest{}, false, fmt.Errorf("%w: island %d", ErrEmptyIslandSelected, island.ID)
			}
		}
	}

	parent1, err := s.selectParent(island)
	if err != nil {
		return ProposeRequest{}, false, fmt.Errorf("failed to select parent: %w", err)
	}
	parent2, err := s.selectParent(island)
	if err != nil {
		return ProposeRequest{}, false, fmt.Errorf("failed to select parent: %w", err)
	}

	return ProposeRequest{
		Parents:  []*Program{parent1, parent2},
		IslandID: island.ID,
	}, true, nil
}

func (s *State) selectParent(island *Island) (*Program, error) {
	selectedCluster, err := s.selectCluster(island)
	if err != nil {
		return nil, err
	}
	return s.selectProgramFromCluster(selectedCluster, island.ID)
}

func (s *State) selectCluster(island *Island) (*Cluster, error) {
	if len(island.Clusters) == 0 {
		return nil, fmt.Errorf("%w: island %d", ErrSelectionFromEmptyIsland, island.ID)
	}

	clusters := make([]*Cluster, 0, len(island.Clusters))
	maxClusterScore := ClusterScore(math.Inf(-1))
	for _, cluster := range island.Clusters {
		clusters = append(clusters, cluster)
		if cluster.Score > maxClusterScore {
			maxClusterScore = cluster.Score
		}
	}

	tc := T0*(1-float64(island.PopulationSize%N)/float64(N)) + Epsilon

	clusterWeightFunc := func(c *Cluster) float64 {
		return math.Exp((c.Score - maxClusterScore) / tc)
	}
	selectedCluster, err := weightedChoice(clusters, clusterWeightFunc)
	if err != nil {
		return nil, fmt.Errorf("cluster selection failed in island %d: %w", island.ID, err)
	}
	return selectedCluster, nil
}

func (s *State) selectProgramFromCluster(cluster *Cluster, islandID int) (*Program, error) {
	programs := cluster.Programs
	if len(programs) == 0 {
		return nil, fmt.Errorf("%w in island %d", ErrInvalidCluster, islandID)
	}
	if len(programs) == 1 {
		return programs[0], nil
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
		return nil, fmt.Errorf("program selection failed from cluster with score %f in island %d: %w", cluster.Score, islandID, err)
	}

	return selectedProgram, nil
}

func (s *State) manageIslands() error {
	if len(s.Islands) <= s.NumIslandsToEliminate {
		return nil
	}

	allIslands := make([]*Island, 0, len(s.Islands))
	for _, island := range s.Islands {
		allIslands = append(allIslands, island)
	}

	sort.Slice(allIslands, func(i, j int) bool {
		return allIslands[i].BestProgram.Score > allIslands[j].BestProgram.Score
	})

	numSurvivors := len(allIslands) - s.NumIslandsToEliminate
	survivors := allIslands[:numSurvivors]
	culled := allIslands[numSurvivors:]

	if len(survivors) == 0 || survivors[len(survivors)-1].BestProgram == nil {
		return fmt.Errorf("%w", ErrNoElitesFound)
	}

	for _, islandToReplace := range culled {
		randomSurvivor := survivors[rand.Intn(len(survivors))]
		elite := randomSurvivor.BestProgram
		if err := islandToReplace.resetWithElite(elite, s.ScoreQuantization); err != nil {
			return err
		}
	}
	return nil
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

type Island struct {
	ID               int
	Clusters         map[ClusterScore]*Cluster
	PopulationSize   int
	EvaluationsCount int
	CullingCount     int
	BestProgram      *Program
}

func (i *Island) addProgram(p *Program, quantization int) error {
	i.EvaluationsCount++
	if p.isBetterThan(i.BestProgram) {
		i.BestProgram = p
		clusterScore, err := quantize(p.Score, quantization)
		if err != nil {
			return err
		}

		if cluster, ok := i.Clusters[clusterScore]; ok {
			cluster.Programs = append(cluster.Programs, p)
		} else {
			i.Clusters[clusterScore] = &Cluster{Score: clusterScore, Programs: []*Program{p}}
		}
		i.PopulationSize++
	}
	return nil
}

func (i *Island) resetWithElite(elite *Program, quantization int) error {
	clusterScore, err := quantize(elite.Score, quantization)
	if err != nil {
		return err
	}
	i.Clusters = map[ClusterScore]*Cluster{clusterScore: {Score: clusterScore, Programs: []*Program{elite}}}
	i.PopulationSize = 1
	i.EvaluationsCount = 0
	i.CullingCount++
	i.BestProgram = elite
	return nil
}

type Cluster struct {
	Score    ClusterScore
	Programs []*Program
}

type ProposeRequest struct {
	Parents  []*Program
	IslandID int
}

type ObserveResult struct {
	Query    Skeleton
	Evidence ProgramScore
	Metadata Metadata
	Err      error
}

type Metadata struct {
	IslandID int
}

func quantize(score ProgramScore, precision int) (ClusterScore, error) {
	key := strconv.FormatFloat(score, 'f', precision, 64)
	f, err := strconv.ParseFloat(key, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse quantized score string '%s': %w", key, err)
	}
	return f, nil
}

type Program struct {
	Skeleton Skeleton
	Score    ProgramScore
}

func (p *Program) isBetterThan(other *Program) bool {
	if p.Score != other.Score {
		return p.Score > other.Score
	}
	if len(p.Skeleton) != len(other.Skeleton) {
		return len(p.Skeleton) < len(other.Skeleton)
	}
	return false
}

type Skeleton = string

type ProgramScore = float64
type ClusterScore = float64

const (
	T0      = 1.0
	N       = 100
	Tp      = 1.0
	Epsilon = 1e-6
)
