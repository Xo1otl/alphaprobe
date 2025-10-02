package llmsr

import (
	"alphaprobe/orchestrator/internal/bilevel"
	"alphaprobe/orchestrator/internal/pb"
	"bufio"
	"context"
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
	state, err := NewState(initialSkeleton, initialScore, maxEvaluations, numIslands, migrationInterval, scoreQuantization, eliminationRate)
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
	state, err := NewState(initialSkeleton, initialScore, maxEvaluations, numIslands, migrationInterval, scoreQuantization, eliminationRate)
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
