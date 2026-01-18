# Leo - Go Concurrency in 4 easy pieces
[![Go Report Card](https://goreportcard.com/badge/github.com/nis057489/leo)](https://goreportcard.com/report/github.com/nis057489/leo)

API inspired by the C++ library [Taskflow](https://github.com/taskflow/taskflow)

## Install
Run `go get github.com/nis057489/leo`

Or just import it and run `go mod tidy`

## Example usage
See ./examples for more.
```go
import "github.com/nis057489/leo"

// Step 1: Initialise the graph to put your tasks/functions in
tasks := leo.TaskGraph()

// Helpers for our example that simulates work by sleeping for a random duration
// but this can be any interface.
taskFunc := func(name string) leo.TaskFunc {
    return func() error {
        fmt.Printf("Executing task %s\n", name)
        // Simulate work with a random sleep
        time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
        fmt.Printf("Completed task %s\n", name)
        return nil
    }
}

// Step 2: Add tasks to the graph. You need to give it a name because I don't know a good way in Go to get __func__
// Each task is just a function that prints its name and sleeps.
tasks.MustAdd("Task A", taskFunc("Task A"))
tasks.MustAdd("Task B", taskFunc("Task B"))
tasks.MustAdd("Task C", taskFunc("Task C"))
tasks.MustAdd("Task D", taskFunc("Task D"))

// Step 3: Establish dependencies: Task A must precede Task B and Task C. Task D succeeds task C. Cycles are an error.
// This means that Task B and Task C will run concurrently after Task A completes,
// and Task D will run after both Task B and Task C complete.
tasks.Precede("Task A", "Task B") // A runs before B
tasks.Precede("Task A", "Task C") // A also runs before C
tasks.Succeed("Task D", "Task B") // D runs after B
tasks.Succeed("Task D", "Task C") // D also runs after C

// Step 4: Create an executor to run the tasks
executor := leo.NewExecutorWithOptions(tasks, leo.ExecutorOptions{
    MaxConcurrency: 4,
    ErrorPolicy:    leo.FailFast,
})

fmt.Println("Executing graph in a loop...")
for i := 0; i < 3; i++ {
    // Execute the graph.
    // This will run Task A first, then Task B and Task C concurrently, then Task D after both B and C complete.
    if err := executor.Execute(); err != nil {
        fmt.Printf("Execution failed: %v\n", err)
    } else {
        fmt.Println("All tasks executed successfully.")
    }
}
```

## Behavior tree example
Leo can be used to create a behaviour tree. See `examples/behaviour_tree`

```go
graph := leo.TaskGraph()

graph.MustAdd("Sense", func() error {
    fmt.Println("Sense: looking for target")
    return nil
})

graph.MustAddCondition("Decide", func() (int, error) {
    // 0 = Combat sequence, 1 = Patrol
    if rand.Intn(2) == 0 {
        fmt.Println("Decide: engage")
        return 0, nil
    }
    fmt.Println("Decide: patrol")
    return 1, nil
})

graph.MustAdd("Chase", func() error { fmt.Println("Chase"); return nil })
graph.MustAdd("Attack", func() error { fmt.Println("Attack"); return nil })
graph.MustAdd("Patrol", func() error { fmt.Println("Patrol"); return nil })

graph.Precede("Sense", "Decide")
graph.Branch("Decide", "Chase", "Patrol")
graph.Precede("Chase", "Attack")

executor := leo.NewExecutor(graph)
_ = executor.Execute()
```

## Execution semantics
- Tasks run when all their parents complete successfully.
- With `FailFast`, no new tasks are scheduled after the first error (in-flight tasks still finish).
- With `ContinueOnError`, the graph continues scheduling regardless of errors, but the first error is still returned.
- `NewExecutor` snapshots the graph at creation time, so later graph mutations won’t affect the executor.

## API notes
- `Add` returns an error (duplicate name or nil task). Use `MustAdd` for a panic-on-error helper.
- `Execute` returns `ErrEmptyGraph` when no tasks were added.

## Conditional tasks
Condition tasks choose exactly one branch to run.
```go
tasks.MustAdd("A", func() error { return nil })
tasks.MustAdd("B", func() error { return nil })
tasks.MustAdd("C", func() error { return nil })

tasks.MustAddCondition("Decide", func() (int, error) {
    // return 0 to run B, 1 to run C
    return 0, nil
})

tasks.Precede("A", "Decide")
tasks.Branch("Decide", "B", "C")
```

## Compose graphs
Compose a subgraph into a larger graph with a prefix and connect via entry/exit nodes.
```go
sub := leo.TaskGraph()
sub.MustAdd("S1", func() error { return nil })
sub.MustAdd("S2", func() error { return nil })
sub.Precede("S1", "S2")

module, _ := tasks.Compose("sub", sub)
for _, entry := range module.Entry {
    tasks.Precede("Task A", entry)
}
for _, exit := range module.Exit {
    tasks.Precede(exit, "Task D")
}
```

### Output
You'll see something like this 3 times. Each time the order of B and C could be different because you left it up to the CPU.
```
Executing task Task A
Completed task Task A
Executing task Task C
Executing task Task B
Completed task Task B
Completed task Task C
Executing task Task D
Completed task Task D
All tasks executed successfully.
```
