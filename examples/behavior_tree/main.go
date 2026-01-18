package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/nis057489/leo"
)

func main() {
	rand.Seed(time.Now().UnixNano())

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
	if err := executor.Execute(); err != nil {
		fmt.Printf("Execution failed: %v\n", err)
	}

}
