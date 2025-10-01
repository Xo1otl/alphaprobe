# `state.go`
```go
type State struct {
	Islands               map[int]*Island
	MaxEvaluations        int
	EvaluationsCount      int
	MigrationInterval     int
	NextMigration         int
	InitialSkeleton       Skeleton
	NumIslandsToEliminate int
	ScoreQuantization     int
	Fatal                 func(err error)
}

func NewState(initialSkeleton Skeleton, initialScore ProgramScore, maxEvaluations, numIslands, migrationInterval, scoreQuantization int, eliminationRate float64, fatal func(err error)) (*State, error) {
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
		Fatal:                 fatal,
	}

	initialClusterScore := quantize(initialScore, s.ScoreQuantization)

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
	island.addProgram(program, s.ScoreQuantization)

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
```

# **NOTE**
* The `Update`/`Next` functions will be called unpredictably within a single goroutine, so no locks are needed.
* The code uses the latest Go syntax and compiles successfully.

# MyConcern
* Since the State Machine is called in a goroutine as part of a pipeline, a simple `return` does not stop the program itself.
* I also feel that it's not appropriate for a goroutine to call `panic`.
* It's a synchronous loop that doesn't include any I/O or CPU-bound processing, so it feels strange for the state to hold a `ctx` (context).
* Therefore, I made the `fatal` function injectable via Dependency Injection (DI). As you can see in `llmsr_test.go`, a function that internally calls `cancel` is injected, enabling a proper shutdown.
* I want to perform more detailed debugging, and I'm thinking it would be a good idea to accept a logger.
* A standard logger usually comes with a `Fatal` function that also handles termination, but `slog` does not have this feature.

# Your Task
What are your thoughts on modifying the design to introduce a Logger Interface equipped with functions like `Debug`, `Print`, and `Fatal`, and then injecting it via DI?
