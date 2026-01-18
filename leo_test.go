package leo

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestAddNode(t *testing.T) {
	graph := TaskGraph()

	if err := graph.Add("A", func() error { return nil }); err != nil {
		t.Fatalf("Add failed to add node 'A': %v", err)
	}

	if _, exists := graph.nodes["A"]; !exists {
		t.Errorf("AddNode failed to add node 'A'")
	}
}

func TestPrecede(t *testing.T) {
	graph := TaskGraph()

	graph.MustAdd("A", func() error { return nil })
	graph.MustAdd("B", func() error { return nil })
	graph.MustAdd("C", func() error { return nil })

	if err := graph.Precede("A", "B"); err != nil {
		t.Errorf("Precede failed to add edge from 'A' to 'B': %v", err)
	}

	if err := graph.Precede("B", "C"); err != nil {
		t.Errorf("Precede failed to add edge from 'B' to 'C': %v", err)
	}

	// This should create a cycle and hence should fail
	if err := graph.Precede("C", "A"); err == nil {
		t.Errorf("%v, Precede should have detected a cycle when adding edge from 'C' to 'A'", err)
	}
}

// TestSucceed checks if edges are added correctly for the Succeed function.
func TestSucceed(t *testing.T) {
	graph := TaskGraph()

	graph.MustAdd("A", func() error { return nil })
	graph.MustAdd("B", func() error { return nil })
	graph.MustAdd("C", func() error { return nil })

	if err := graph.Succeed("B", "A"); err != nil {
		t.Errorf("Succeed failed to add edge from 'B' to 'A': %v", err)
	}

	if err := graph.Succeed("C", "B"); err != nil {
		t.Errorf("Succeed failed to add edge from 'C' to 'B': %v", err)
	}

	// This should create a cycle because it closes the cycle A -> B -> C -> A
	if err := graph.Succeed("A", "C"); err == nil {
		t.Errorf("Succeed should have detected a cycle when adding edge from 'A' to 'C' to close the cycle")
	}

	// This should not create a cycle and should be allowed
	if err := graph.Succeed("C", "A"); err != nil {
		t.Errorf("Succeed should not have detected a cycle when adding edge from 'C' to 'A': %v", err)
	}
}

func TestExecutorExecute(t *testing.T) {
	graph := TaskGraph()

	executedNodes := make(map[string]bool)

	graph.MustAdd("A", func() error {
		executedNodes["A"] = true
		return nil
	})
	graph.MustAdd("B", func() error {
		if !executedNodes["A"] {
			return errors.New("node 'A' should have executed before 'B'")
		}
		executedNodes["B"] = true
		return nil
	})
	graph.MustAdd("C", func() error {
		if !executedNodes["B"] {
			return errors.New("node 'B' should have executed before 'C'")
		}
		executedNodes["C"] = true
		return nil
	})

	graph.Precede("A", "B")
	graph.Precede("B", "C")

	executor := NewExecutor(graph)

	if err := executor.Execute(); err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	for _, node := range []string{"A", "B", "C"} {
		if !executedNodes[node] {
			t.Errorf("node '%s' did not execute", node)
		}
	}
}

func TestDAGExecution(t *testing.T) {
	graph := TaskGraph()

	var executionOrder []string
	var orderLock sync.Mutex

	// Helper function to record execution order
	recordExecution := func(name string) {
		orderLock.Lock()
		defer orderLock.Unlock()
		executionOrder = append(executionOrder, name)
	}

	// Define tasks
	graph.MustAdd("A", func() error {
		recordExecution("A")
		time.Sleep(100 * time.Millisecond)
		return nil
	})
	graph.MustAdd("B", func() error {
		recordExecution("B")
		time.Sleep(100 * time.Millisecond)
		return nil
	})
	graph.MustAdd("C", func() error {
		recordExecution("C")
		time.Sleep(100 * time.Millisecond)
		return nil
	})
	graph.MustAdd("D", func() error {
		recordExecution("D")
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	// Set up dependencies
	err := graph.Precede("A", "B")
	if err != nil {
		t.Fatalf("Failed to add edge: %s", err)
	}
	err = graph.Precede("A", "C")
	if err != nil {
		t.Fatalf("Failed to add edge: %s", err)
	}
	err = graph.Succeed("D", "B")
	if err != nil {
		t.Fatalf("Failed to add edge: %s", err)
	}
	err = graph.Succeed("D", "C")
	if err != nil {
		t.Fatalf("Failed to add edge: %s", err)
	}

	// Execute graph
	executor := NewExecutor(graph)

	if err := executor.Execute(); err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	fmt.Println("Execution Order:", executionOrder)

	// A must come before B and C, and D must come after B and C
	if !(indexOf(executionOrder, "A") < indexOf(executionOrder, "B") &&
		indexOf(executionOrder, "A") < indexOf(executionOrder, "C") &&
		indexOf(executionOrder, "B") < indexOf(executionOrder, "D") &&
		indexOf(executionOrder, "C") < indexOf(executionOrder, "D")) {
		t.Errorf("Execution order does not match expected dependencies")
	}
}

func indexOf(slice []string, val string) int {
	for i, item := range slice {
		if item == val {
			return i
		}
	}
	return -1
}

func TestAddDuplicateTask(t *testing.T) {
	graph := TaskGraph()

	if err := graph.Add("A", func() error { return nil }); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := graph.Add("A", func() error { return nil }); !errors.Is(err, ErrDuplicateTask) {
		t.Fatalf("expected ErrDuplicateTask, got %v", err)
	}
}

func TestExecuteEmptyGraph(t *testing.T) {
	graph := TaskGraph()
	executor := NewExecutor(graph)

	if err := executor.Execute(); !errors.Is(err, ErrEmptyGraph) {
		t.Fatalf("expected ErrEmptyGraph, got %v", err)
	}
}

func TestExecuteFailFast(t *testing.T) {
	graph := TaskGraph()
	var ranB bool

	graph.MustAdd("A", func() error { return errors.New("boom") })
	graph.MustAdd("B", func() error { ranB = true; return nil })

	if err := graph.Precede("A", "B"); err != nil {
		t.Fatalf("unexpected edge error: %v", err)
	}

	executor := NewExecutorWithOptions(graph, ExecutorOptions{ErrorPolicy: FailFast})
	if err := executor.Execute(); err == nil {
		t.Fatalf("expected error")
	}
	if ranB {
		t.Fatalf("expected B to be skipped in fail-fast mode")
	}
}

func TestExecuteContinueOnError(t *testing.T) {
	graph := TaskGraph()
	var ranB bool

	graph.MustAdd("A", func() error { return errors.New("boom") })
	graph.MustAdd("B", func() error { ranB = true; return nil })

	if err := graph.Precede("A", "B"); err != nil {
		t.Fatalf("unexpected edge error: %v", err)
	}

	executor := NewExecutorWithOptions(graph, ExecutorOptions{ErrorPolicy: ContinueOnError})
	if err := executor.Execute(); err == nil {
		t.Fatalf("expected error")
	}
	if !ranB {
		t.Fatalf("expected B to run in continue-on-error mode")
	}
}

func TestMaxConcurrency(t *testing.T) {
	graph := TaskGraph()
	var maxSeen int
	var current int
	var lock sync.Mutex

	makeTask := func() TaskFunc {
		return func() error {
			lock.Lock()
			current++
			if current > maxSeen {
				maxSeen = current
			}
			lock.Unlock()

			time.Sleep(50 * time.Millisecond)

			lock.Lock()
			current--
			lock.Unlock()
			return nil
		}
	}

	graph.MustAdd("A", makeTask())
	graph.MustAdd("B", makeTask())
	graph.MustAdd("C", makeTask())
	graph.MustAdd("D", makeTask())

	executor := NewExecutorWithOptions(graph, ExecutorOptions{MaxConcurrency: 2, ErrorPolicy: ContinueOnError})
	if err := executor.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if maxSeen > 2 {
		t.Fatalf("expected max concurrency 2, saw %d", maxSeen)
	}
}

func TestConditionalBranch(t *testing.T) {
	graph := TaskGraph()
	var ranB, ranC bool

	graph.MustAdd("A", func() error { return nil })
	graph.MustAddCondition("Cond", func() (int, error) { return 0, nil })
	graph.MustAdd("B", func() error { ranB = true; return nil })
	graph.MustAdd("C", func() error { ranC = true; return nil })

	if err := graph.Precede("A", "Cond"); err != nil {
		t.Fatalf("unexpected edge error: %v", err)
	}
	if err := graph.Branch("Cond", "B", "C"); err != nil {
		t.Fatalf("unexpected branch error: %v", err)
	}

	executor := NewExecutor(graph)
	if err := executor.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ranB || ranC {
		t.Fatalf("expected B to run and C to be skipped")
	}
}

func TestConditionalInvalidIndex(t *testing.T) {
	graph := TaskGraph()

	graph.MustAdd("A", func() error { return nil })
	graph.MustAddCondition("Cond", func() (int, error) { return 2, nil })
	graph.MustAdd("B", func() error { return nil })
	graph.MustAdd("C", func() error { return nil })

	if err := graph.Precede("A", "Cond"); err != nil {
		t.Fatalf("unexpected edge error: %v", err)
	}
	if err := graph.Branch("Cond", "B", "C"); err != nil {
		t.Fatalf("unexpected branch error: %v", err)
	}

	executor := NewExecutor(graph)
	if err := executor.Execute(); !errors.Is(err, ErrInvalidConditionIndex) {
		t.Fatalf("expected ErrInvalidConditionIndex, got %v", err)
	}
}

func TestComposeGraph(t *testing.T) {
	root := TaskGraph()
	var order []string
	var lock sync.Mutex

	record := func(name string) {
		lock.Lock()
		defer lock.Unlock()
		order = append(order, name)
	}

	root.MustAdd("A", func() error { record("A"); return nil })
	root.MustAdd("B", func() error { record("B"); return nil })

	sub := TaskGraph()
	sub.MustAdd("S1", func() error { record("S1"); return nil })
	sub.MustAdd("S2", func() error { record("S2"); return nil })
	if err := sub.Precede("S1", "S2"); err != nil {
		t.Fatalf("unexpected subgraph edge error: %v", err)
	}

	module, err := root.Compose("sub", sub)
	if err != nil {
		t.Fatalf("unexpected compose error: %v", err)
	}

	for _, entry := range module.Entry {
		if err := root.Precede("A", entry); err != nil {
			t.Fatalf("unexpected edge error: %v", err)
		}
	}
	for _, exit := range module.Exit {
		if err := root.Precede(exit, "B"); err != nil {
			t.Fatalf("unexpected edge error: %v", err)
		}
	}

	executor := NewExecutor(root)
	if err := executor.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !(indexOf(order, "A") < indexOf(order, "S1") &&
		indexOf(order, "S2") < indexOf(order, "B")) {
		t.Fatalf("unexpected execution order: %v", order)
	}
}
