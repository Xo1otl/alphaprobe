package rastrigin

import (
	"context"
	"math"
	"math/rand"
	"sort"
)

// --- Type Aliases for Clarity ---
type Gene = []float64
type Fitness = float64
type Population = []Individual

// --- Concrete Data Structures ---

// Individual holds the genetic information and fitness of a single entity.
type Individual struct {
	Gene    Gene
	Fitness Fitness
}

// Island represents a subpopulation.
type Island struct {
	ID         int
	Population Population
}

// Metadata is the context object passed through the pipeline.
type Metadata struct {
	IslandID int
}

// State holds the entire state and logic for the genetic algorithm.
type State struct {
	Islands            []*Island
	PendingIslands     map[int]bool
	AvailableIslandIDs []int
	EvaluationsCount   int
	TotalEvaluations   int
	MigrationInterval  int
	MigrationSize      int
}

// NewState initializes the state for the GA.
func NewState(
	islandPopulation int,
	numIslands int,
	totalEvaluations int,
	migrationInterval int,
	migrationSize int,
) *State {
	islands := make([]*Island, numIslands)
	for i := range numIslands {
		islands[i] = &Island{ID: i, Population: newInitialPopulation(islandPopulation)}
	}

	availableIDs := make([]int, len(islands))
	for i, island := range islands {
		availableIDs[i] = island.ID
	}

	return &State{
		Islands:            islands,
		PendingIslands:     make(map[int]bool),
		AvailableIslandIDs: availableIDs,
		EvaluationsCount:   0,
		TotalEvaluations:   totalEvaluations,
		MigrationInterval:  migrationInterval,
		MigrationSize:      migrationSize,
	}
}

// Update is the core logic function. It's decoupled from the runner's internal types.
func (s *State) Update(ctx context.Context, gene Gene, fitness Fitness, metadata Metadata) ([]*Island, bool) {
	// --- 1. Incorporate the result from the last completed task (Propagate logic) ---
	// On the first call, gene will be nil.
	if gene != nil {
		islandID := metadata.IslandID
		evaluatedChild := Individual{Gene: gene, Fitness: fitness}

		delete(s.PendingIslands, islandID)
		s.EvaluationsCount++
		s.AvailableIslandIDs = append(s.AvailableIslandIDs, islandID)

		incorporate(s.Islands[islandID], []Individual{evaluatedChild})

		if s.EvaluationsCount > 0 && s.EvaluationsCount%s.MigrationInterval == 0 {
			migrate(s.Islands, s.MigrationSize)
		}
	}

	// --- 2. Check for termination condition (ShouldTerminate logic) ---
	if s.EvaluationsCount >= s.TotalEvaluations {
		return nil, true // No new tasks, and terminate.
	}

	// --- 3. Prepare the next task(s) to be dispatched (Dispatch logic) ---
	if len(s.AvailableIslandIDs) == 0 {
		return nil, false // No tasks to dispatch right now, but don't terminate.
	}

	randIndex := rand.Intn(len(s.AvailableIslandIDs))
	islandID := s.AvailableIslandIDs[randIndex]

	s.AvailableIslandIDs = append(s.AvailableIslandIDs[:randIndex], s.AvailableIslandIDs[randIndex+1:]...)
	s.PendingIslands[islandID] = true

	nextTask := s.Islands[islandID]
	return []*Island{nextTask}, false // Dispatch one task, and continue.
}

// --- GA Logic (Propose/Observe) ---

// Propose generates a new gene from an island. It's a pure function.
func Propose(ctx context.Context, island *Island) (Gene, Metadata) {
	pop := island.Population
	tournament := func() Individual {
		best := pop[rand.Intn(len(pop))]
		for i := 1; i < 5; i++ { // TournamentSize
			competitor := pop[rand.Intn(len(pop))]
			if competitor.Fitness < best.Fitness {
				best = competitor
			}
		}
		return best
	}
	parent1, parent2 := tournament(), tournament()

	var childGene Gene
	if rand.Float64() < 0.9 { // CrossoverRate
		childGene = crossoverBLXAlpha(parent1.Gene, parent2.Gene, 0.5)
	} else {
		childGene = make(Gene, 30) // NumDimensions
		copy(childGene, parent1.Gene)
	}

	for i := range childGene {
		if rand.Float64() < 1.0/30.0 { // MutationRate
			childGene[i] += rand.NormFloat64() * ((5.12 - (-5.12)) * 0.05) // StdDev
			childGene[i] = math.Max(-5.12, math.Min(5.12, childGene[i]))
		}
	}

	return childGene, Metadata{IslandID: island.ID}
}

// Observe evaluates a gene's fitness. It's a pure function.
func Observe(ctx context.Context, gene Gene) Fitness {
	a := 10.0
	sum := a * float64(len(gene))
	for _, x := range gene {
		sum += x*x - a*math.Cos(2*math.Pi*x)
	}
	return Fitness(sum)
}

// --- Helper Functions ---

func newInitialPopulation(size int) Population {
	pop := make(Population, size)
	for i := range pop {
		gene := make(Gene, 30)
		for j := range gene {
			gene[j] = -5.12 + rand.Float64()*(5.12-(-5.12))
		}
		pop[i] = Individual{Gene: gene, Fitness: Observe(context.Background(), gene)}
	}
	return pop
}

func crossoverBLXAlpha(p1, p2 Gene, alpha float64) Gene {
	child := make(Gene, len(p1))
	for i := range p1 {
		d := math.Abs(p1[i] - p2[i])
		minGene := math.Min(p1[i], p2[i]) - alpha*d
		maxGene := math.Max(p1[i], p2[i]) + alpha*d
		child[i] = minGene + rand.Float64()*(maxGene-minGene)
	}
	return child
}

func incorporate(island *Island, individuals []Individual) {
	sort.Slice(island.Population, func(a, b int) bool {
		return island.Population[a].Fitness > island.Population[b].Fitness // Worst first
	})
	for j := 0; j < len(individuals) && j < len(island.Population); j++ {
		if individuals[j].Fitness < island.Population[j].Fitness {
			island.Population[j] = individuals[j]
		}
	}
}

func migrate(islands []*Island, migrationSize int) {
	if len(islands) <= 1 {
		return
	}
	allMigrants := make([][]Individual, len(islands))
	for i, island := range islands {
		sort.Slice(island.Population, func(a, b int) bool {
			return island.Population[a].Fitness < island.Population[b].Fitness // Best first
		})
		count := min(migrationSize, len(island.Population))
		allMigrants[i] = island.Population[:count]
	}
	for i, sourceIslandMigrants := range allMigrants {
		targetIslandIndex := (i + 1) % len(islands)
		incorporate(islands[targetIslandIndex], sourceIslandMigrants)
	}
}