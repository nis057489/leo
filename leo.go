package leo

import (
	"errors"
	"fmt"
)

type TaskFunc func() error
type CondTaskFunc func() (int, error)

var (
	ErrEmptyGraph            = errors.New("graph has no tasks")
	ErrDuplicateTask         = errors.New("task with the same name already exists")
	ErrNodeNotFound          = errors.New("one or both nodes do not exist")
	ErrCycle                 = errors.New("adding this edge would create a cycle")
	ErrNilTask               = errors.New("task function cannot be nil")
	ErrEdgeExists            = errors.New("edge already exists")
	ErrNoStartNodes          = errors.New("graph has no start nodes (cycle?)")
	ErrInvalidConditionIndex = errors.New("condition returned an invalid index")
	ErrConditionNoBranches   = errors.New("condition task has no branches")
	ErrInvalidModulePrefix   = errors.New("module prefix cannot be empty")
	ErrEmptySubgraph         = errors.New("subgraph has no tasks")
	ErrNotConditionTask      = errors.New("node is not a condition task")
)

type Node struct {
	task         TaskFunc
	cond         CondTaskFunc
	children     []*Node
	condChildren []*Node
	parents      []*Node
	name         string
}

type Graph struct {
	nodes map[string]*Node
}

func TaskGraph() *Graph {
	return &Graph{
		nodes: make(map[string]*Node),
	}
}

func (g *Graph) Add(name string, task TaskFunc) error {
	if task == nil {
		return ErrNilTask
	}
	if _, exists := g.nodes[name]; exists {
		return ErrDuplicateTask
	}
	g.nodes[name] = &Node{
		task:         task,
		children:     make([]*Node, 0),
		condChildren: make([]*Node, 0),
		parents:      make([]*Node, 0),
		name:         name,
	}
	return nil
}

func (g *Graph) MustAdd(name string, task TaskFunc) {
	if err := g.Add(name, task); err != nil {
		panic(err)
	}
}

func (g *Graph) AddCondition(name string, task CondTaskFunc) error {
	if task == nil {
		return ErrNilTask
	}
	if _, exists := g.nodes[name]; exists {
		return ErrDuplicateTask
	}
	g.nodes[name] = &Node{
		cond:         task,
		children:     make([]*Node, 0),
		condChildren: make([]*Node, 0),
		parents:      make([]*Node, 0),
		name:         name,
	}
	return nil
}

func (g *Graph) MustAddCondition(name string, task CondTaskFunc) {
	if err := g.AddCondition(name, task); err != nil {
		panic(err)
	}
}

// Precede adds a directed edge from node `from` to node `to`
func (g *Graph) Precede(from, to string) error {
	fromNode, fromExists := g.nodes[from]
	toNode, toExists := g.nodes[to]

	if !fromExists || !toExists {
		return ErrNodeNotFound
	}

	if fromNode == toNode {
		return ErrCycle
	}

	for _, child := range fromNode.children {
		if child == toNode {
			return ErrEdgeExists
		}
	}

	if g.pathExists(toNode, fromNode) {
		return ErrCycle
	}

	fromNode.children = append(fromNode.children, toNode)
	toNode.parents = append(toNode.parents, fromNode)

	return nil
}

// Succeed sets up a "succeeds" relationship, indicating that `to` should succeed `from`.
func (g *Graph) Succeed(from, to string) error {
	return g.Precede(to, from)
}

// Branch adds ordered conditional edges from a condition task to its possible successors.
// The condition function should return the index of the next task to run in the `tos` list.
func (g *Graph) Branch(from string, tos ...string) error {
	fromNode, fromExists := g.nodes[from]
	if !fromExists {
		return ErrNodeNotFound
	}
	if fromNode.cond == nil {
		return ErrNotConditionTask
	}
	if len(tos) == 0 {
		return ErrConditionNoBranches
	}

	for _, to := range tos {
		toNode, toExists := g.nodes[to]
		if !toExists {
			return ErrNodeNotFound
		}
		if fromNode == toNode {
			return ErrCycle
		}
		for _, child := range fromNode.condChildren {
			if child == toNode {
				return ErrEdgeExists
			}
		}
		if g.pathExists(toNode, fromNode) {
			return ErrCycle
		}
		fromNode.condChildren = append(fromNode.condChildren, toNode)
		toNode.parents = append(toNode.parents, fromNode)
	}

	return nil
}

type Module struct {
	Prefix string
	Entry  []string
	Exit   []string
}

func (g *Graph) Compose(prefix string, sub *Graph) (*Module, error) {
	if prefix == "" {
		return nil, ErrInvalidModulePrefix
	}
	if sub == nil || len(sub.nodes) == 0 {
		return nil, ErrEmptySubgraph
	}

	nameFor := func(n string) string {
		return prefix + "::" + n
	}

	for name := range sub.nodes {
		if _, exists := g.nodes[nameFor(name)]; exists {
			return nil, ErrDuplicateTask
		}
	}

	cloneMap := make(map[*Node]*Node, len(sub.nodes))
	for name, node := range sub.nodes {
		newNode := &Node{
			task:         node.task,
			cond:         node.cond,
			children:     make([]*Node, 0, len(node.children)),
			condChildren: make([]*Node, 0, len(node.condChildren)),
			parents:      make([]*Node, 0, len(node.parents)),
			name:         nameFor(name),
		}
		g.nodes[newNode.name] = newNode
		cloneMap[node] = newNode
	}

	for node, newNode := range cloneMap {
		for _, child := range node.children {
			newChild := cloneMap[child]
			newNode.children = append(newNode.children, newChild)
		}
		for _, child := range node.condChildren {
			newChild := cloneMap[child]
			newNode.condChildren = append(newNode.condChildren, newChild)
		}
	}

	for _, newNode := range cloneMap {
		for _, child := range newNode.children {
			child.parents = append(child.parents, newNode)
		}
		for _, child := range newNode.condChildren {
			child.parents = append(child.parents, newNode)
		}
	}

	entry := make([]string, 0)
	exit := make([]string, 0)
	for node, newNode := range cloneMap {
		if len(node.parents) == 0 {
			entry = append(entry, newNode.name)
		}
		if len(node.children) == 0 && len(node.condChildren) == 0 {
			exit = append(exit, newNode.name)
		}
	}

	return &Module{Prefix: prefix, Entry: entry, Exit: exit}, nil
}

type Executor struct {
	graph   *Graph
	options ExecutorOptions
}

type ErrorPolicy int

const (
	FailFast ErrorPolicy = iota
	ContinueOnError
)

type ExecutorOptions struct {
	MaxConcurrency int
	ErrorPolicy    ErrorPolicy
}

func NewExecutor(graph *Graph) *Executor {
	return NewExecutorWithOptions(graph, ExecutorOptions{ErrorPolicy: FailFast})
}

func NewExecutorWithOptions(graph *Graph, options ExecutorOptions) *Executor {
	if options.ErrorPolicy != ContinueOnError {
		options.ErrorPolicy = FailFast
	}

	return &Executor{
		graph:   graph.clone(),
		options: options,
	}
}

func (e *Executor) Execute() error {
	if len(e.graph.nodes) == 0 {
		return ErrEmptyGraph
	}

	inDegree := make(map[*Node]int, len(e.graph.nodes))
	ready := make([]*Node, 0, len(e.graph.nodes))

	for _, node := range e.graph.nodes {
		inDegree[node] = len(node.parents)
		if inDegree[node] == 0 {
			ready = append(ready, node)
		}
	}

	if len(ready) == 0 {
		return ErrNoStartNodes
	}

	type taskResult struct {
		node      *Node
		err       error
		condIndex int
		hasCond   bool
	}

	taskDone := make(chan taskResult)
	running := 0
	var firstErr error
	maxConc := e.options.MaxConcurrency
	policy := e.options.ErrorPolicy

	for len(ready) > 0 || running > 0 {
		if firstErr != nil && policy == FailFast && running == 0 {
			break
		}

		for len(ready) > 0 && (maxConc <= 0 || running < maxConc) && (firstErr == nil || policy == ContinueOnError) {
			idx := len(ready) - 1
			node := ready[idx]
			ready = ready[:idx]
			running++
			go func(n *Node) {
				var err error
				result := taskResult{node: n}
				if n.cond != nil {
					idx, condErr := n.cond()
					result.condIndex = idx
					result.hasCond = true
					err = condErr
				} else if n.task != nil {
					err = n.task()
				}
				if err != nil {
					result.err = fmt.Errorf("error executing node %s: %w", n.name, err)
				}
				taskDone <- result
			}(node)
		}

		if running == 0 {
			break
		}

		result := <-taskDone
		running--
		if result.err != nil && firstErr == nil {
			firstErr = result.err
		}

		if result.err == nil || policy == ContinueOnError {
			if result.node.cond != nil && result.err == nil {
				if len(result.node.condChildren) == 0 {
					if firstErr == nil {
						firstErr = ErrConditionNoBranches
					}
					continue
				}
				if !result.hasCond {
					if firstErr == nil {
						firstErr = ErrInvalidConditionIndex
					}
					continue
				}
				idx := result.condIndex
				if idx < 0 || idx >= len(result.node.condChildren) {
					if firstErr == nil {
						firstErr = ErrInvalidConditionIndex
					}
					continue
				}
				selected := result.node.condChildren[idx]
				inDegree[selected]--
				if inDegree[selected] == 0 {
					ready = append(ready, selected)
				}
			}

			for _, child := range result.node.children {
				inDegree[child]--
				if inDegree[child] == 0 {
					ready = append(ready, child)
				}
			}
		}
	}

	if firstErr != nil {
		return firstErr
	}

	return nil
}

func (g Graph) Print() {
	for _, node := range g.nodes {
		fmt.Printf("%s -> ", node.name)
		for _, child := range node.children {
			fmt.Printf("%s, ", child.name)
		}
		for _, child := range node.condChildren {
			fmt.Printf("%s?, ", child.name)
		}
		fmt.Println()
	}
}

func (g *Graph) pathExists(from, target *Node) bool {
	stack := []*Node{from}
	visited := make(map[*Node]bool)

	for len(stack) > 0 {
		idx := len(stack) - 1
		node := stack[idx]
		stack = stack[:idx]

		if node == target {
			return true
		}
		if visited[node] {
			continue
		}
		visited[node] = true

		for _, child := range node.children {
			if !visited[child] {
				stack = append(stack, child)
			}
		}
		for _, child := range node.condChildren {
			if !visited[child] {
				stack = append(stack, child)
			}
		}
	}

	return false
}

func (g *Graph) clone() *Graph {
	clone := &Graph{
		nodes: make(map[string]*Node, len(g.nodes)),
	}

	for name, node := range g.nodes {
		clone.nodes[name] = &Node{
			task:         node.task,
			cond:         node.cond,
			children:     make([]*Node, 0, len(node.children)),
			condChildren: make([]*Node, 0, len(node.condChildren)),
			parents:      make([]*Node, 0, len(node.parents)),
			name:         name,
		}
	}

	for name, node := range g.nodes {
		cloneNode := clone.nodes[name]
		for _, child := range node.children {
			cloneChild := clone.nodes[child.name]
			cloneNode.children = append(cloneNode.children, cloneChild)
		}
		for _, child := range node.condChildren {
			cloneChild := clone.nodes[child.name]
			cloneNode.condChildren = append(cloneNode.condChildren, cloneChild)
		}
	}

	for _, node := range clone.nodes {
		for _, child := range node.children {
			child.parents = append(child.parents, node)
		}
		for _, child := range node.condChildren {
			child.parents = append(child.parents, node)
		}
	}

	return clone
}
