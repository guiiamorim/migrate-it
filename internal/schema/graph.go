package schema

import "sort"

// Graph is a directed graph of table dependencies built from foreign keys.
//
// An edge from -> to means "from must be created before to": the referenced
// (parent) table points to the referencing (child) table. Reading edges in
// this direction makes a topological sort yield a valid creation order
// (parents first); reversing it yields a valid drop order (children first).
type Graph struct {
	nodes map[string]bool
	edges map[string]map[string]bool // from -> set of to
}

// NewGraph returns an empty graph.
func NewGraph() *Graph {
	return &Graph{nodes: map[string]bool{}, edges: map[string]map[string]bool{}}
}

// BuildGraph constructs the dependency graph for every table in the schema.
// Foreign keys to tables outside the schema, and self-references, are ignored:
// neither constrains ordering within this schema.
func BuildGraph(s *Schema) *Graph {
	g := NewGraph()
	for _, name := range s.TableNames() {
		g.AddNode(name)
	}
	for _, name := range s.TableNames() {
		t := s.Tables[name]
		for _, parent := range t.DependsOn() {
			if _, ok := s.Tables[parent]; !ok {
				continue
			}
			g.AddEdge(parent, t.Name) // parent before child
		}
	}
	return g
}

// AddNode registers a table in the graph.
func (g *Graph) AddNode(name string) {
	g.nodes[name] = true
	if g.edges[name] == nil {
		g.edges[name] = map[string]bool{}
	}
}

// AddEdge records that `from` must precede `to`.
func (g *Graph) AddEdge(from, to string) {
	g.AddNode(from)
	g.AddNode(to)
	g.edges[from][to] = true
}

// Nodes returns all nodes in sorted order.
func (g *Graph) Nodes() []string {
	names := make([]string, 0, len(g.nodes))
	for n := range g.nodes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// TopologicalSort orders nodes so that every node appears after all of its
// prerequisites (parents before children). It uses Kahn's algorithm with a
// deterministic tie-break on node name.
//
// The second return value lists nodes that could not be ordered because they
// participate in a dependency cycle (e.g. two tables with mutual foreign keys).
// Callers handle these out of band — typically by creating the tables first and
// adding the cyclic foreign keys afterwards.
func (g *Graph) TopologicalSort() (ordered []string, cyclic []string) {
	inDegree := map[string]int{}
	for n := range g.nodes {
		inDegree[n] = 0
	}
	for from := range g.edges {
		for to := range g.edges[from] {
			inDegree[to]++
		}
	}

	var ready []string
	for n, d := range inDegree {
		if d == 0 {
			ready = append(ready, n)
		}
	}
	sort.Strings(ready)

	for len(ready) > 0 {
		n := ready[0]
		ready = ready[1:]
		ordered = append(ordered, n)

		var unlocked []string
		for to := range g.edges[n] {
			inDegree[to]--
			if inDegree[to] == 0 {
				unlocked = append(unlocked, to)
			}
		}
		sort.Strings(unlocked)
		ready = append(ready, unlocked...)
		sort.Strings(ready)
	}

	if len(ordered) != len(g.nodes) {
		for n, d := range inDegree {
			if d > 0 {
				cyclic = append(cyclic, n)
			}
		}
		sort.Strings(cyclic)
	}
	return ordered, cyclic
}

// CreationOrder returns tables ordered so parents precede children, followed by
// any tables tangled in cycles (in sorted order).
func (g *Graph) CreationOrder() []string {
	ordered, cyclic := g.TopologicalSort()
	return append(ordered, cyclic...)
}

// DropOrder returns the reverse of CreationOrder, so children precede parents.
func (g *Graph) DropOrder() []string {
	order := g.CreationOrder()
	reversed := make([]string, len(order))
	for i, n := range order {
		reversed[len(order)-1-i] = n
	}
	return reversed
}
