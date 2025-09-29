package mock

import (
	"context"
	"fmt"
	"log"
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

func (s *State) Update(res ObserveResult) bool {
	s.EvaluationsCount++
	newProgram := Program{
		Skeleton: res.Query,
		Score:    res.Evidence,
	}
	s.Programs = append(s.Programs, newProgram)

	const maxPopulation = 10
	if len(s.Programs) > maxPopulation {
		sort.Slice(s.Programs, func(i, j int) bool {
			return s.Programs[i].Score < s.Programs[j].Score
		})
		s.Programs = s.Programs[:maxPopulation]
	}

	if res.Evidence < s.BestScore {
		s.BestScore = res.Evidence
		fmt.Printf("New best score: %f (Evaluation #%d)\n", s.BestScore, s.EvaluationsCount)
	}

	log.Printf("[Update] Received result for skeleton starting with: %q", res.Query[:20])
	log.Printf("[Update] Metadata parents: %v", res.Metadata.ParentSkeletons)
	log.Printf("[Update] PendingParents BEFORE delete: %v", s.PendingParents)
	for _, p := range res.Metadata.ParentSkeletons {
		delete(s.PendingParents, p)
	}
	log.Printf("[Update] PendingParents AFTER delete: %v", s.PendingParents)

	return s.EvaluationsCount >= s.MaxEvaluations
}

func (s *State) NewRequest() ([]Program, bool) {
	// Special handling for the very first task
	if s.EvaluationsCount == 0 && len(s.Programs) == 1 {
		initialProgram := s.Programs[0]
		parents := []Program{initialProgram, initialProgram}
		for _, p := range parents {
			s.PendingParents[p.Skeleton] = true
		}
		log.Printf("[Update] GENERATED new task. New PendingParents: %v", s.PendingParents)
		return parents, true
	}

	if len(s.PendingParents) > 0 {
		log.Printf("[Update] SKIPPING new task generation. PendingParents is not empty: %v", s.PendingParents)
		return nil, false
	}

	log.Println("[Update] PendingParents is empty. Proceeding to generate new task.")
	availablePrograms := make([]Program, 0, len(s.Programs))
	for _, p := range s.Programs {
		if !s.PendingParents[p.Skeleton] {
			availablePrograms = append(availablePrograms, p)
		}
	}

	if len(availablePrograms) < 2 {
		log.Println("[Update] Not enough available programs to create a new task. Terminating.")
		return nil, false
	}

	rand.Shuffle(len(availablePrograms), func(i, j int) {
		availablePrograms[i], availablePrograms[j] = availablePrograms[j], availablePrograms[i]
	})
	parent1 := availablePrograms[0]
	parent2 := availablePrograms[1]

	parents := []Program{parent1, parent2}
	for _, p := range parents {
		s.PendingParents[p.Skeleton] = true
	}
	log.Printf("[Update] GENERATED new task. New PendingParents: %v", s.PendingParents)
	return parents, true
}

// --- Types for bilevel Runner ---

type ProposeResult struct {
	Skeletons []ProgramSkeleton
	Metadata  Metadata
}

type ObserveRequest struct {
	Query    ProgramSkeleton
	Metadata Metadata
}

type ObserveResult struct {
	Query    ProgramSkeleton
	Evidence Score
	Metadata Metadata
}

// --- Pipeline Functions ---

func Propose(ctx context.Context, parents []Program) ProposeResult {
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

	return ProposeResult{
		Skeletons: newSkeletons,
		Metadata: Metadata{
			ParentSkeletons: parentSkeletons,
		},
	}
}

func Observe(ctx context.Context, req ObserveRequest) ObserveResult {
	score := rand.Float64()
	return ObserveResult{
		Query:    req.Query,
		Evidence: score,
		Metadata: req.Metadata,
	}
}

type Adapter struct {
	queue []ObserveRequest
}

func NewAdapter() *Adapter {
	return &Adapter{
		queue: make([]ObserveRequest, 0),
	}
}

func (s *Adapter) Recv(res ProposeResult) bool {
	for _, skeleton := range res.Skeletons {
		req := ObserveRequest{
			Query:    skeleton,
			Metadata: res.Metadata,
		}
		s.queue = append(s.queue, req)
	}
	return false
}

func (s *Adapter) Next() (ObserveRequest, bool) {
	if len(s.queue) == 0 {
		return ObserveRequest{}, false
	}
	req := s.queue[0]
	s.queue = s.queue[1:]
	return req, true
}
