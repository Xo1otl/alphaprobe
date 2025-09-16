package rastrigin

import (
	"math"
	"math/rand"
	"sort"

	"alphaprobe/orchestrator/internal/islandga"
)

// Gene represents the chromosome for the Rastrigin problem.
type Gene []float64

// Fitness represents the evaluation result for the Rastrigin problem.
type Fitness float64

type InternalState = []islandga.Individual[Gene, Fitness]

// --- GA/Rastrigin Constants ---
const (
	// -- GA Parameters --
	NumDimensions  = 30
	CrossoverRate  = 0.9
	BLXAlpha       = 0.5
	TournamentSize = 5

	// -- Execution Control --
	MigrationInterval = 25
	MigrationSize     = 5
)

var (
	SearchMin      = -5.12
	SearchMax      = 5.12
	MutationRate   = 1.0 / float64(NumDimensions)
	MutationStdDev = (SearchMax - SearchMin) * 0.05
)

// --- Island Implementation ---

// Island implements the islandga.Island interface for the Rastrigin problem.
type Island struct {
	id         int
	population []islandga.Individual[Gene, Fitness]
}

func NewIsland(id int, population []islandga.Individual[Gene, Fitness]) islandga.Island[Gene, Fitness, InternalState] {
	return &Island{id: id, population: population}
}

func (i *Island) ID() int { return i.id }

func (i *Island) InternalState() InternalState {
	// Return a copy to prevent external modification of the slice header, though individuals are pointers.
	popCopy := make([]islandga.Individual[Gene, Fitness], len(i.population))
	copy(popCopy, i.population)
	return popCopy
}

func (i *Island) Incorporate(individuals []islandga.Individual[Gene, Fitness]) {
	sort.Slice(i.population, func(a, b int) bool {
		return i.population[a].Fitness > i.population[b].Fitness // Sort worst first
	})

	for j := 0; j < len(individuals) && j < len(i.population); j++ {
		if individuals[j].Fitness < i.population[j].Fitness {
			i.population[j] = individuals[j]
		}
	}
}

func (i *Island) SelectMigrants(n int, cloneFn islandga.CloneFunc[Gene]) []islandga.Individual[Gene, Fitness] {
	sort.Slice(i.population, func(a, b int) bool {
		return i.population[a].Fitness < i.population[b].Fitness // Sort best first
	})

	count := min(n, len(i.population))
	migrants := make([]islandga.Individual[Gene, Fitness], count)
	for j := range count {
		original := i.population[j]
		migrants[j] = islandga.Individual[Gene, Fitness]{
			Gene:    cloneFn(original.Gene),
			Fitness: original.Fitness,
		}
	}
	return migrants
}

// --- GA Logic ---

// Propose implements the variation part of the GA for Rastrigin.
func Propose(population InternalState) Gene {
	tournament := func() islandga.Individual[Gene, Fitness] {
		best := population[rand.Intn(len(population))]
		for i := 1; i < TournamentSize; i++ {
			competitor := population[rand.Intn(len(population))]
			if competitor.Fitness < best.Fitness {
				best = competitor
			}
		}
		return best
	}
	parent1, parent2 := tournament(), tournament()

	var childChromosome Gene
	if rand.Float64() < CrossoverRate {
		childChromosome = crossoverBLXAlpha(parent1.Gene, parent2.Gene, BLXAlpha)
	} else {
		childChromosome = make(Gene, NumDimensions)
		copy(childChromosome, parent1.Gene)
	}

	for i := range childChromosome {
		if rand.Float64() < MutationRate {
			childChromosome[i] += rand.NormFloat64() * MutationStdDev
			childChromosome[i] = math.Max(SearchMin, math.Min(SearchMax, childChromosome[i]))
		}
	}
	return childChromosome
}

// Observe implements the evaluation part of the GA for Rastrigin.
func Observe(gene Gene) Fitness {
	return rastrigin(gene)
}

// NewInitialPopulation creates and evaluates the first generation of individuals.
func NewInitialPopulation(populationSize int) []islandga.Individual[Gene, Fitness] {
	population := make([]islandga.Individual[Gene, Fitness], populationSize)
	for i := range population {
		gene := make(Gene, NumDimensions)
		for j := range gene {
			gene[j] = SearchMin + rand.Float64()*(SearchMax-SearchMin)
		}
		population[i] = islandga.Individual[Gene, Fitness]{
			Gene:    gene,
			Fitness: Observe(gene),
		}
	}
	return population
}

func rastrigin(chromosome Gene) Fitness {
	a := 10.0
	sum := a * float64(len(chromosome))
	for _, x := range chromosome {
		sum += x*x - a*math.Cos(2*math.Pi*x)
	}
	return Fitness(sum)
}

func crossoverBLXAlpha(p1, p2 Gene, alpha float64) Gene {
	child := make(Gene, len(p1))
	for i := range p1 {
		d := math.Abs(p1[i] - p2[i])
		minGene := math.Min(p1[i], p2[i]) - alpha*d
		maxGene := math.Max(p1[i], p2[i]) + alpha*d
		minGene = math.Max(SearchMin, minGene)
		maxGene = math.Min(SearchMax, maxGene)
		if minGene > maxGene {
			minGene, maxGene = maxGene, minGene
		}
		child[i] = minGene + rand.Float64()*(maxGene-minGene)
	}
	return child
}
