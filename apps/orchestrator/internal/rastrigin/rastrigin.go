package rastrigin

import (
	"math"
	"math/rand"
	"sort"

	"alphaprobe/orchestrator/internal/bilevel"
)

// --- Type Aliases for Clarity ---
// These types map the Rastrigin problem to the generic types of the island_v2.Runner.
type Gene = []float64
type Fitness = float64
type Population = []Individual

type State = GaState            // S
type ProposeIn = *Island        // PIn
type Query = Gene               // Q
type Context = RastriginContext // C
type Evidence = Fitness         // E

// --- Concrete Data Structures ---

// Individual holds the genetic information and fitness of a single entity.
type Individual struct {
	Gene    Gene
	Fitness Fitness
}

// Island represents a subpopulation. It is no longer an interface implementation,
// but a concrete struct used by the Rastrigin logic.
type Island struct {
	ID         int
	Population Population
}

// GaState holds the overall state of the genetic algorithm.
type GaState struct {
	Islands            []*Island
	PendingIslands     map[int]bool
	AvailableIslandIDs []int
	EvaluationsCount   int
	TotalEvaluations   int
	MigrationInterval  int
	MigrationSize      int
}

// RastriginContext is the context object passed through the pipeline.
// It carries the necessary information for the propagate function.
type RastriginContext struct {
	IslandID int
	Gene     Gene // The original gene, to be re-associated with the fitness.
}

// --- State Initialization ---

func NewInitialState(
	islandPopulation int,
	numIslands int,
	totalEvaluations int,
	migrationInterval int,
	migrationSize int,
) *GaState {
	islands := make([]*Island, numIslands)
	for i := range numIslands {
		islands[i] = &Island{ID: i, Population: newInitialPopulation(islandPopulation)}
	}

	availableIDs := make([]int, len(islands))
	for i, island := range islands {
		availableIDs[i] = island.ID
	}

	return &GaState{
		Islands:            islands,
		PendingIslands:     make(map[int]bool),
		AvailableIslandIDs: availableIDs,
		EvaluationsCount:   0,
		TotalEvaluations:   totalEvaluations,
		MigrationInterval:  migrationInterval,
		MigrationSize:      migrationSize,
	}
}

// --- GA Logic (Propose/Observe) ---

// Propose generates a new gene and a context object from a given island.
func Propose(island ProposeIn) (Query, Context) {
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

	ctx := Context{
		IslandID: island.ID,
		Gene:     childGene,
	}
	return childGene, ctx
}

// Observe evaluates a gene and returns its fitness.
func Observe(gene Query) Evidence {
	a := 10.0
	sum := a * float64(len(gene))
	for _, x := range gene {
		sum += x*x - a*math.Cos(2*math.Pi*x)
	}
	return Fitness(sum)
}

// --- State Manipulation Functions for Runner ---

// Dispatch selects an available island and sends it to the proposal channel.
func Dispatch(state *GaState, proposeCh chan<- ProposeIn) {
	if len(state.AvailableIslandIDs) == 0 {
		return
	}
	randIndex := rand.Intn(len(state.AvailableIslandIDs))
	islandID := state.AvailableIslandIDs[randIndex]

	state.AvailableIslandIDs = append(state.AvailableIslandIDs[:randIndex], state.AvailableIslandIDs[randIndex+1:]...)
	state.PendingIslands[islandID] = true

	proposeCh <- state.Islands[islandID]
}

// Propagate incorporates the evaluation result and handles migration.
func Propagate(state *GaState, result bilevel.Result[Evidence, Context]) {
	ctx := result.Ctx
	islandID := ctx.IslandID
	evaluatedChild := Individual{Gene: ctx.Gene, Fitness: result.Evidence}

	// Update state
	delete(state.PendingIslands, islandID)
	state.EvaluationsCount++
	state.AvailableIslandIDs = append(state.AvailableIslandIDs, islandID)

	// Incorporate result
	incorporate(state.Islands[islandID], []Individual{evaluatedChild})

	// Handle migration
	if state.EvaluationsCount%state.MigrationInterval == 0 && state.EvaluationsCount > 0 {
		migrate(state.Islands, state.MigrationSize)
	}
}

// ShouldTerminate checks if the evaluation limit has been reached.
func ShouldTerminate(state *GaState) bool {
	isEvaluationLimitReached := state.EvaluationsCount >= state.TotalEvaluations
	return isEvaluationLimitReached
}

// --- Helper Functions ---

func newInitialPopulation(size int) Population {
	pop := make(Population, size)
	for i := range pop {
		gene := make(Gene, 30)
		for j := range gene {
			gene[j] = -5.12 + rand.Float64()*(5.12-(-5.12))
		}
		pop[i] = Individual{Gene: gene, Fitness: Observe(gene)}
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
