package llmsr

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
)

type ProgramSkeleton = string
type Score = float64
type Program struct {
	Skeleton ProgramSkeleton
	Score    Score
}

type State struct {
	Programs         []Program
	EvaluationsCount int
	MaxEvaluations   int
	BestScore        Score
	PendingParents   map[string]bool
}

type Metadata struct {
	ParentSkeletons []ProgramSkeleton
}

func NewState(initialSkeleton ProgramSkeleton, maxEvaluations int) *State {
	initialProgram := Program{
		Skeleton: initialSkeleton,
		Score:    1e9, // A very large number representing an unevaluated score.
	}
	return &State{
		Programs:         []Program{initialProgram},
		EvaluationsCount: 0,
		MaxEvaluations:   maxEvaluations,
		BestScore:        1e9,
		PendingParents:   make(map[string]bool),
	}
}

func (s *State) GetInitialTask() [][]Program {
	if len(s.Programs) != 1 || s.EvaluationsCount != 0 {
		return nil // Should only be called at the start.
	}

	initialProgram := s.Programs[0]
	s.PendingParents[initialProgram.Skeleton] = true
	nextTask := []Program{initialProgram, initialProgram}
	return [][]Program{nextTask}
}

func (s *State) Update(ctx context.Context, skeleton ProgramSkeleton, score Score, metadata Metadata) ([][]Program, bool) {
	s.EvaluationsCount++
	newProgram := Program{
		Skeleton: skeleton,
		Score:    score,
	}
	s.Programs = append(s.Programs, newProgram)

	const maxPopulation = 10
	if len(s.Programs) > maxPopulation {
		sort.Slice(s.Programs, func(i, j int) bool {
			return s.Programs[i].Score < s.Programs[j].Score
		})
		s.Programs = s.Programs[:maxPopulation]
	}

	if score < s.BestScore {
		s.BestScore = score
		fmt.Printf("New best score: %f (Evaluation #%d)\n", s.BestScore, s.EvaluationsCount)
	}

	for _, p := range metadata.ParentSkeletons {
		delete(s.PendingParents, p)
	}

	if s.EvaluationsCount >= s.MaxEvaluations {
		return nil, true
	}

	if len(s.PendingParents) > 0 {
		return nil, false
	}

	availablePrograms := make([]Program, 0, len(s.Programs))
	for _, p := range s.Programs {
		if !s.PendingParents[p.Skeleton] {
			availablePrograms = append(availablePrograms, p)
		}
	}

	if len(availablePrograms) < 2 {
		return nil, true
	}

	rand.Shuffle(len(availablePrograms), func(i, j int) {
		availablePrograms[i], availablePrograms[j] = availablePrograms[j], availablePrograms[i]
	})
	parent1 := availablePrograms[0]
	parent2 := availablePrograms[1]
	s.PendingParents[parent1.Skeleton] = true
	s.PendingParents[parent2.Skeleton] = true

	nextTask := []Program{parent1, parent2}
	return [][]Program{nextTask}, false
}

func Propose(ctx context.Context, parents []Program) ([]ProgramSkeleton, Metadata) {
	batchSize := rand.Intn(4) + 1
	newSkeletons := make([]ProgramSkeleton, 0, batchSize)
	for range batchSize {
		newSkeleton := fmt.Sprintf("%s\n# Mutated %d", parents[0].Skeleton, rand.Intn(100))
		newSkeletons = append(newSkeletons, newSkeleton)
	}

	parentSkeletons := make([]ProgramSkeleton, len(parents))
	for i, p := range parents {
		parentSkeletons[i] = p.Skeleton
	}

	metadata := Metadata{
		ParentSkeletons: parentSkeletons,
	}
	return newSkeletons, metadata
}

func FanOut(pout []ProgramSkeleton, data Metadata) []ProgramSkeleton {
	return pout
}

func Observe(ctx context.Context, skeleton ProgramSkeleton) Score {
	return rand.Float64()
}
