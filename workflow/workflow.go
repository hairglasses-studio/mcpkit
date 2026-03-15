package workflow

import (
	"context"
	"fmt"
	"time"
)

// State flows through the graph. Value type — cloned between steps.
type State struct {
	Data      map[string]any
	Metadata  map[string]string
	Step      int
	NodeName  string
	UpdatedAt time.Time
}

func NewState() State {
	return State{
		Data:      make(map[string]any),
		Metadata:  make(map[string]string),
		UpdatedAt: time.Now(),
	}
}

func (s State) Clone() State {
	c := State{
		Data:      make(map[string]any, len(s.Data)),
		Metadata:  make(map[string]string, len(s.Metadata)),
		Step:      s.Step,
		NodeName:  s.NodeName,
		UpdatedAt: s.UpdatedAt,
	}
	for k, v := range s.Data {
		c.Data[k] = v
	}
	for k, v := range s.Metadata {
		c.Metadata[k] = v
	}
	return c
}

// Get retrieves a typed value from State.Data.
func Get[T any](s State, key string) (T, bool) {
	v, ok := s.Data[key]
	if !ok {
		var zero T
		return zero, false
	}
	t, ok := v.(T)
	if !ok {
		var zero T
		return zero, false
	}
	return t, true
}

// Set returns a new State with the given key-value pair set.
func Set(s State, key string, value any) State {
	c := s.Clone()
	c.Data[key] = value
	return c
}

// NodeFunc executes a graph node. Takes and returns State.
type NodeFunc func(ctx context.Context, state State) (State, error)

// EdgeCondition evaluates state and returns the next node name (or EndNode).
type EdgeCondition func(state State) string

// EndNode sentinel — conditional edges return this to signal completion.
const EndNode = "__END__"

// NodeOption configures a node.
type NodeOption func(*nodeConfig)

type nodeConfig struct {
	timeout time.Duration
}

// WithNodeTimeout sets a per-node timeout.
func WithNodeTimeout(d time.Duration) NodeOption {
	return func(c *nodeConfig) {
		c.timeout = d
	}
}

type edge struct {
	to        string
	condition EdgeCondition // nil for unconditional
}

type node struct {
	name   string
	fn     NodeFunc
	config nodeConfig
	edges  []edge
}

// Graph is the workflow graph definition.
type Graph struct {
	nodes map[string]*node
	start string
}

// NewGraph creates a new empty graph.
func NewGraph() *Graph {
	return &Graph{
		nodes: make(map[string]*node),
	}
}

// AddNode adds a named node to the graph.
func (g *Graph) AddNode(name string, fn NodeFunc, opts ...NodeOption) error {
	if name == "" {
		return fmt.Errorf("workflow: node name cannot be empty")
	}
	if name == EndNode {
		return fmt.Errorf("workflow: %q is reserved", EndNode)
	}
	if fn == nil {
		return fmt.Errorf("workflow: node %q handler cannot be nil", name)
	}
	if _, exists := g.nodes[name]; exists {
		return fmt.Errorf("workflow: duplicate node %q", name)
	}
	var cfg nodeConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	g.nodes[name] = &node{name: name, fn: fn, config: cfg}
	return nil
}

// AddEdge adds an unconditional edge from one node to another (or EndNode).
func (g *Graph) AddEdge(from, to string) error {
	if _, ok := g.nodes[from]; !ok {
		return fmt.Errorf("workflow: unknown source node %q", from)
	}
	if to != EndNode {
		if _, ok := g.nodes[to]; !ok {
			return fmt.Errorf("workflow: unknown target node %q", to)
		}
	}
	n := g.nodes[from]
	n.edges = append(n.edges, edge{to: to})
	return nil
}

// AddConditionalEdge adds a conditional edge from a node. The condition function
// evaluates state and returns the next node name (or EndNode).
func (g *Graph) AddConditionalEdge(from string, condition EdgeCondition) error {
	if _, ok := g.nodes[from]; !ok {
		return fmt.Errorf("workflow: unknown source node %q", from)
	}
	if condition == nil {
		return fmt.Errorf("workflow: condition cannot be nil for node %q", from)
	}
	n := g.nodes[from]
	n.edges = append(n.edges, edge{condition: condition})
	return nil
}

// SetStart sets the entry-point node.
func (g *Graph) SetStart(name string) error {
	if _, ok := g.nodes[name]; !ok {
		return fmt.Errorf("workflow: unknown start node %q", name)
	}
	g.start = name
	return nil
}

// Validate checks the graph for structural issues.
func (g *Graph) Validate() error {
	if g.start == "" {
		return fmt.Errorf("workflow: no start node set")
	}
	if len(g.nodes) == 0 {
		return fmt.Errorf("workflow: graph has no nodes")
	}
	// Check all unconditional edge targets exist
	for name, n := range g.nodes {
		for _, e := range n.edges {
			if e.condition != nil {
				continue // conditional edges are evaluated at runtime
			}
			if e.to != EndNode {
				if _, ok := g.nodes[e.to]; !ok {
					return fmt.Errorf("workflow: node %q has edge to unknown node %q", name, e.to)
				}
			}
		}
		// Each node must have at least one edge (or it's a dead-end)
		if len(n.edges) == 0 {
			return fmt.Errorf("workflow: node %q has no outgoing edges", name)
		}
	}
	return nil
}
