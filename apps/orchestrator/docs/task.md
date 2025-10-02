# `state.go`
```go
package llmsr

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"time"
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
	CallSequence          []string
	rng                   *rand.Rand
}

func NewState(initialSkeleton Skeleton, initialScore ProgramScore, maxEvaluations, numIslands, migrationInterval, scoreQuantization int, eliminationRate float64, rng *rand.Rand) (*State, error) {
	if eliminationRate < 0 || eliminationRate >= 1 {
		return nil, fmt.Errorf("%w", ErrInvalidEliminationRate)
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	s := &State{
		Islands:               make(map[int]*Island, numIslands),
		MaxEvaluations:        maxEvaluations,
		MigrationInterval:     migrationInterval,
		NextMigration:         migrationInterval,
		InitialSkeleton:       initialSkeleton,
		NumIslandsToEliminate: int(float64(numIslands) * eliminationRate),
		ScoreQuantization:     scoreQuantization,
		rng:                   rng,
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
	s.CallSequence = append(s.CallSequence, "Update")
	if res.Err != nil {
		return true, res.Err
	}
	s.EvaluationsCount++

	island, ok := s.Islands[res.Metadata.IslandID]
	if !ok {
		return true, fmt.Errorf("%w: island with ID %d", ErrIslandNotFound, res.Metadata.IslandID)
	}

	program := &Program{Skeleton: res.Query, Score: res.Evidence}
	if err := island.addProgram(program, s.ScoreQuantization); err != nil {
		return true, err
	}

	if s.EvaluationsCount >= s.NextMigration {
		if err := s.manageIslands(); err != nil {
			return true, err
		}
		s.NextMigration += s.MigrationInterval
	}
	return s.EvaluationsCount >= s.MaxEvaluations, nil
}

func (s *State) Issue() (ProposeRequest, bool, error) {
	s.CallSequence = append(s.CallSequence, "Issue")
	islandIDs := make([]int, 0, len(s.Islands))
	for id := range s.Islands {
		islandIDs = append(islandIDs, id)
	}
	if len(islandIDs) == 0 {
		return ProposeRequest{}, false, nil
	}
	randomID := islandIDs[s.rng.Intn(len(islandIDs))]
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
		return ProposeRequest{}, false, err
	}
	parent2, err := s.selectParent(island)
	if err != nil {
		return ProposeRequest{}, false, err
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
	selectedCluster, err := weightedChoice(clusters, clusterWeightFunc, s.rng)
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
	selectedProgram, err := weightedChoice(programs, skeletonWeightFunc, s.rng)
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
		randomSurvivor := survivors[s.rng.Intn(len(survivors))]
		elite := randomSurvivor.BestProgram
		if err := islandToReplace.resetWithElite(elite, s.ScoreQuantization); err != nil {
			return err
		}
	}
	return nil
}

func weightedChoice[T any](items []T, getWeight func(T) float64, rng *rand.Rand) (T, error) {
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

	randVal := rng.Float64() * sumWeights
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
```
# `llmsr_test.go`
```go
package llmsr

import (
	"alphaprobe/orchestrator/internal/bilevel"
	"alphaprobe/orchestrator/internal/pb"
	"bufio"
	"context"
	"math/rand"
	"os/exec"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	maxEvaluations     = 3000
	numIslands         = 4
	eliminationRate    = 0.5
	migrationInterval  = 25
	proposeConcurrency = 2
	observeConcurrency = 4
	testTimeout        = 5 * time.Second
	scoreQuantization  = 2
)

func testRng() *rand.Rand {
	return rand.New(rand.NewSource(42))
}

func TestLLMSR_WithMock(t *testing.T) {
	state, initialScore := runLLMSR(t, MockPropose, MockObserve)
	grpcCallSequence := state.CallSequence

	logStateSummary(t, state, initialScore)

	assert.True(t, state.EvaluationsCount >= maxEvaluations, "Should have completed at least the specified number of evaluations")
	assert.Greater(t, getBestScore(state), initialScore, "The final best score should be better (greater) than the initial score")

	t.Log("--- Running Simulation with sequence and Mock workers ---")

	simulatedState := runSimulation(t, grpcCallSequence, MockPropose, MockObserve)
	logStateSummary(t, simulatedState, initialScore)
}

func TestLLMSR_WithGRPCServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		"/workspaces/alphaprobe/.venv/bin/python", "-u",
		"-c", "import llmsr_worker; llmsr_worker.main()",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout pipe: %v", err)
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start gRPC server: %v", err)
	}
	defer func() {
		if err := cmd.Process.Kill(); err != nil {
			t.Logf("Failed to kill process: %v", err)
		}
		cmd.Wait()
	}()

	serverReady := make(chan bool)
	expectedOutput := "gRPC server started"
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			t.Logf("[gRPC Server]: %s", line)
			if strings.Contains(line, expectedOutput) {
				t.Log("gRPC server is ready.")
				close(serverReady)
				return
			}
		}
	}()
	select {
	case <-serverReady:
	case <-ctx.Done():
		t.Fatal("Timeout waiting for gRPC server to start.")
	}

	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to connect to gRPC server: %v", err)
	}
	defer conn.Close()

	client := pb.NewLLMSRClient(conn)
	proposeFn := NewGRPCPropose(client)
	observeFn := NewGRPCObserve(client)

	state, initialScore := runLLMSR(t, proposeFn, observeFn)
	callSequence := state.CallSequence

	logStateSummary(t, state, initialScore)

	assert.True(t, state.EvaluationsCount >= maxEvaluations, "Should have completed at least the specified number of evaluations")
	assert.Greater(t, getBestScore(state), initialScore, "The final best score should be better (greater) than the initial score")

	t.Log("--- Running Simulation with gRPC sequence and Mock workers ---")

	simulatedState := runSimulation(t, callSequence, MockPropose, MockObserve)
	logStateSummary(t, simulatedState, initialScore)
}

func runLLMSR(t *testing.T, proposeFn bilevel.ProposeFunc[ProposeRequest, ProposeResult], observeFn bilevel.ObserveFunc[ObserveRequest, ObserveResult]) (*State, float64) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	initialSkeleton := "-100"
	initialScore := observeFn(ctx, ObserveRequest{Query: Skeleton(initialSkeleton)}).Evidence
	state, err := NewState(initialSkeleton, initialScore, maxEvaluations, numIslands, migrationInterval, scoreQuantization, eliminationRate, testRng())
	if err != nil {
		t.Fatalf("Failed to create initial state: %v", err)
	}

	adapter := NewAdapter()

	orchestrator := bilevel.NewOrchestrator(
		proposeFn,
		observeFn,
		proposeConcurrency,
		observeConcurrency,
	)

	errCh := make(chan error, 1)
	go func() {
		err, ok := <-errCh
		if ok {
			t.Logf("Test context canceled by error: %v", err)
			cancel()
		}
	}()

	bilevel.RunWithAdapter(orchestrator, ctx, state, adapter, errCh)

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("Test timed out, indicating a potential deadlock or server issue.")
	}
	return state, initialScore
}

func logStateSummary(t *testing.T, state *State, initialScore float64) {
	t.Helper()
	t.Log("--- State Summary ---")
	t.Logf("Total Islands: %d", len(state.Islands))

	// Sort islands by ID for consistent logging
	sortedIslands := make([]*Island, 0, len(state.Islands))
	for _, island := range state.Islands {
		sortedIslands = append(sortedIslands, island)
	}
	sort.Slice(sortedIslands, func(i, j int) bool {
		return sortedIslands[i].ID < sortedIslands[j].ID
	})

	totalProposeWeightedSum := 0.0

	for _, island := range sortedIslands {
		totalPrograms := 0
		totalScore := 0.0
		for _, cluster := range island.Clusters {
			numPrograms := len(cluster.Programs)
			totalPrograms += numPrograms
			totalScore += cluster.Score * float64(numPrograms)
			totalProposeWeightedSum += float64(numPrograms) * (cluster.Score - initialScore)
		}

		avgScore := 0.0
		if totalPrograms > 0 {
			avgScore = totalScore / float64(totalPrograms)
		}

		bestProgram := island.BestProgram
		bestSkeleton := "N/A"
		if bestProgram != nil {
			bestSkeleton = bestProgram.Skeleton
		}

		t.Logf("  Island %d: %d clusters, %d programs, Evals: %d, Culls: %d, Avg Score: %.2f, Best Score: %.2f, Best Skeleton: '%s'",
			island.ID, len(island.Clusters), totalPrograms, island.EvaluationsCount, island.CullingCount, avgScore, island.BestProgram.Score, bestSkeleton)
	}
	t.Logf("Total Propose-Weighted Sum: %.2f", totalProposeWeightedSum)

	var sequenceBuilder strings.Builder
	issueCount := 0
	updateCount := 0
	for _, call := range state.CallSequence {
		if len(call) > 0 {
			sequenceBuilder.WriteByte(call[0])
		}
		switch call {
		case "Issue":
			issueCount++
		case "Update":
			updateCount++
		}
	}
	sequence := sequenceBuilder.String()
	if len(sequence) > 100 {
		sequence = sequence[:100] + "..."
	}
	t.Logf("Call Sequence: %s", sequence)
	t.Logf("Total Calls: Issue=%d, Update=%d", issueCount, updateCount)
	t.Logf("Initial score: %f, Best score found: %f", initialScore, getBestScore(state))
	t.Log("---------------------")
}

func getBestScore(s *State) ProgramScore {
	bestScore := ProgramScore(-1e9)
	for _, island := range s.Islands {
		islandBest := island.BestProgram.Score
		if islandBest > bestScore {
			bestScore = islandBest
		}
	}
	return bestScore
}

func runSimulation(t *testing.T, callSequence []string, proposeFn bilevel.ProposeFunc[ProposeRequest, ProposeResult], observeFn bilevel.ObserveFunc[ObserveRequest, ObserveResult]) *State {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	initialSkeleton := "-100"
	initialScore := observeFn(ctx, ObserveRequest{Query: Skeleton(initialSkeleton)}).Evidence
	state, err := NewState(initialSkeleton, initialScore, maxEvaluations, numIslands, migrationInterval, scoreQuantization, eliminationRate, testRng())
	if err != nil {
		t.Fatalf("Failed to create initial state: %v", err)
	}

	skeletonsToObserve := make([]ObserveRequest, 0)

	for _, call := range callSequence {
		if call == "Issue" {
			proposeReq, ok, err := state.Issue()
			if err != nil {
				t.Fatalf("Error issuing propose request: %v", err)
			}
			if ok {
				proposeResult := proposeFn(ctx, proposeReq)
				for _, skeleton := range proposeResult.Skeletons {
					skeletonsToObserve = append(skeletonsToObserve, ObserveRequest{
						Query:    skeleton,
						Metadata: Metadata{IslandID: proposeReq.IslandID},
					})
				}
			}
		} else if call == "Update" {
			if len(skeletonsToObserve) > 0 {
				observeReq := skeletonsToObserve[0]
				skeletonsToObserve = skeletonsToObserve[1:]

				observeResult := observeFn(ctx, observeReq)
				done, err := state.Update(observeResult)
				if err != nil {
					t.Fatalf("Error updating state: %v", err)
				}
				if done {
					break
				}
			}
		}
	}

	return state
}
```
# **NOTE**
* The `Update`/`Next` functions will be called unpredictably within a single goroutine, so no locks are needed.
* The code uses the latest Go syntax and compiles successfully.

# MyConcern
rngを固定してもRunとsimulationの結果が異なります
callSequenceを用いても、UpdateがどのIssueから得られたresultによって行われたのかが不明であり、単純なFIFO Queueでは実際のデータを再現できず、異なる結果になったと思われる
単純なcallSequenceではなく、stateに変化をもたらす入力をすべてイベントとして使い、イベントソーシングすべきでは？
Updateの入力だけ保持しとけば再現性とれるはずだよね。
あ、callSequenceとUpdateの入力を全部保持しとけば再現できるのかな。

# Your Task
リファクタリング案を考えてください
