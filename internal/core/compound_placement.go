// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package core

import (
	"slices"

	"github.com/d2lang/elk-go/internal/graph"
)

// RouteCrossContainerEdges enables native routes for binary connections across
// separately laid-out hierarchies. It is an elk-go extension on the root.
var RouteCrossContainerEdges = graph.NewProperty[bool]("elkgo.routeCrossContainerEdges")

// CrossContainerRadialTrees is temporary placement data for an opted-in
// Radial owner whose projected sibling graph has disconnected components.
var CrossContainerRadialTrees = graph.NewProperty[[][]graph.NodeID]("elkgo.internal.crossContainerRadialTrees")

type compoundEdge struct {
	id                 graph.EdgeID
	source, target     []graph.ShapeID
	container          *graph.NodeID
	srcChild, dstChild graph.NodeID
	parent             graph.NodeID
	algorithm          string
	projected          bool
}

// Flat providers place sibling nodes. For the placement pass represent a
// cross-container connection by its two sibling subtrees. A connection from
// a container into itself has no such pair, so hide it until final routing.
// This also keeps Radial's traversal from treating a grandchild as a sibling.
func (engine *Engine) layoutCompoundEdges(model *graph.Model) error {
	var edges []compoundEdge
	for i := range model.Edges {
		id := graph.EdgeID(i)
		e := model.Edge(id)
		parent, ok := compoundOwner(model, e)
		if !ok {
			continue
		}
		algorithm, _ := graph.TryGet(&model.Node(parent).Properties, ResolvedAlgorithm)
		if algorithm == nil || !supportsCompoundProjection(algorithm.ID) {
			continue
		}
		src, dst := model.ShapeNode(e.Sources[0]), model.ShapeNode(e.Targets[0])
		edges = append(edges, compoundEdge{
			id: id, source: e.Sources, target: e.Targets, container: e.ContainingNode, parent: parent,
			srcChild: childInLayout(model, src, parent), dstChild: childInLayout(model, dst, parent),
			algorithm: algorithm.ID,
		})
	}
	if len(edges) == 0 {
		_, err := engine.layoutRecursively(model, model.Root, engine.Monitor)
		return err
	}

	restore := projectCompoundEdges(model, edges)
	// Restore endpoint identity even if a custom provider or monitor panics.
	err := func() error {
		defer restore()
		_, err := engine.layoutRecursively(model, model.Root, engine.Monitor)
		return err
	}()
	if err != nil || engine.Monitor.Canceled() {
		return err
	}
	return routeCompoundEdges(model, edges)
}

// compoundOwner is the provider placing the two endpoint subtrees. For
// a container-to-descendant edge it is the provider that places the container.
func compoundOwner(model *graph.Model, e *graph.Edge) (graph.NodeID, bool) {
	if len(e.Sources) != 1 || len(e.Targets) != 1 || e.ContainingNode == nil || graph.Get(&e.Properties, NoLayout) {
		return 0, false
	}
	src, dst := model.ShapeNode(e.Sources[0]), model.ShapeNode(e.Targets[0])
	srcParent, dstParent := model.Node(src).Parent, model.Node(dst).Parent
	if src == dst || srcParent != nil && dstParent != nil && *srcParent == *dstParent {
		return 0, false
	}
	if !layoutNodeEnabled(model, src) || !layoutNodeEnabled(model, dst) {
		return 0, false
	}
	parent := *e.ContainingNode
	if src == parent || dst == parent {
		if p := model.Node(parent).Parent; p != nil {
			parent = *p
		}
	}
	return parent, true
}

func supportsCompoundProjection(algorithm string) bool {
	switch algorithm {
	case "org.eclipse.elk.force", "org.eclipse.elk.stress", "org.eclipse.elk.mrtree", "org.eclipse.elk.radial":
		return true
	}
	return false
}

func layoutNodeEnabled(model *graph.Model, id graph.NodeID) bool {
	for p := &id; p != nil; p = model.Node(*p).Parent {
		if graph.Get(&model.Node(*p).Properties, NoLayout) {
			return false
		}
	}
	return true
}

func childInLayout(model *graph.Model, id, owner graph.NodeID) graph.NodeID {
	for id != owner {
		p := *model.Node(id).Parent
		if p == owner {
			break
		}
		id = p
	}
	return id
}

func compoundIncidence(model *graph.Model, shape graph.ShapeID, outgoing bool) *[]graph.EdgeID {
	if shape.Kind == graph.PortShape {
		port := model.Port(shape.Port)
		if outgoing {
			return &port.OutgoingEdges
		}
		return &port.IncomingEdges
	}
	node := model.Node(shape.Node)
	if outgoing {
		return &node.OutgoingEdges
	}
	return &node.IncomingEdges
}

func projectCompoundEdges(model *graph.Model, edges []compoundEdge) func() {
	// Snapshot only the incidence lists that actually change. Edges and
	// ports keep their arena IDs throughout, including on the JSON and typed
	// paths. Filtering each list once preserves the original edge order.
	hidden := make(map[graph.EdgeID]bool, len(edges))
	for _, c := range edges {
		hidden[c.id] = true
	}
	// A tree provider can traverse one placement link for each pair of
	// components even if several directed edges cross the same boundary.
	trees := map[graph.NodeID]treeProjection{}
	saved := make(map[*[]graph.EdgeID][]graph.EdgeID)
	save := func(list *[]graph.EdgeID) { saved[list] = *list }
	for i := range edges {
		c := &edges[i]
		save(compoundIncidence(model, c.source[0], true))
		save(compoundIncidence(model, c.target[0], false))
		save(&model.Node(*c.container).ContainedEdges)
		c.projected = c.srcChild != c.dstChild && c.srcChild != c.parent && c.dstChild != c.parent
		if c.algorithm == "org.eclipse.elk.radial" || c.projected && c.algorithm == "org.eclipse.elk.mrtree" {
			tree := trees[c.parent]
			if tree == nil {
				tree = newTreeProjection(model, c.parent, hidden)
				trees[c.parent] = tree
			}
			if c.projected {
				c.srcChild, c.dstChild, c.projected = tree.connect(c.srcChild, c.dstChild)
			}
		}
		if c.projected {
			save(&model.Node(c.srcChild).OutgoingEdges)
			save(&model.Node(c.dstChild).IncomingEdges)
			save(&model.Node(c.parent).ContainedEdges)
		}
	}
	for list, original := range saved {
		*list = slices.DeleteFunc(slices.Clone(original), func(id graph.EdgeID) bool { return hidden[id] })
	}
	for i := range edges {
		c := &edges[i]
		e := model.Edge(c.id)
		e.Sources, e.Targets, e.ContainingNode = nil, nil, nil
		if !c.projected {
			continue
		}
		parent := model.Node(c.parent)
		e.PlacementProjection = true
		e.ContainingNode = &c.parent
		parent.ContainedEdges = append(parent.ContainedEdges, c.id)
		model.AddSource(c.id, graph.NodeRef(c.srcChild))
		model.AddTarget(c.id, graph.NodeRef(c.dstChild))
	}
	var forests []graph.NodeID
	for parent, tree := range trees {
		node := model.Node(parent)
		algorithm, _ := graph.TryGet(&node.Properties, ResolvedAlgorithm)
		if algorithm.ID != "org.eclipse.elk.radial" {
			continue
		}
		components := tree.components(node.Children)
		if len(components) > 1 {
			graph.Set(&node.Properties, CrossContainerRadialTrees, components)
			forests = append(forests, parent)
		}
	}
	return func() {
		for _, parent := range forests {
			graph.Unset(&model.Node(parent).Properties, CrossContainerRadialTrees)
		}
		for list, original := range saved {
			*list = original
		}
		for _, c := range edges {
			e := model.Edge(c.id)
			e.PlacementProjection = false
			e.Sources, e.Targets, e.ContainingNode = c.source, c.target, c.container
		}
	}
}

// Keep existing sibling edges first. A projected relation may join two
// components but must not introduce a cycle or a second tree parent.
type treeProjection map[graph.NodeID]graph.NodeID

func newTreeProjection(model *graph.Model, owner graph.NodeID, hidden map[graph.EdgeID]bool) treeProjection {
	tree := make(treeProjection)
	for _, id := range model.Node(owner).ContainedEdges {
		e := model.Edge(id)
		if hidden[id] || len(e.Sources) != 1 || len(e.Targets) != 1 {
			continue
		}
		source, target := model.ShapeNode(e.Sources[0]), model.ShapeNode(e.Targets[0])
		src, dst := tree.root(source), tree.root(target)
		if src != dst {
			tree[dst] = src
		}
	}
	return tree
}

func (tree treeProjection) root(id graph.NodeID) graph.NodeID {
	root := id
	for {
		parent, found := tree[root]
		if !found {
			break
		}
		root = parent
	}
	for id != root {
		next := tree[id]
		tree[id] = root
		id = next
	}
	return root
}

func (tree treeProjection) connect(source, target graph.NodeID) (graph.NodeID, graph.NodeID, bool) {
	srcRoot, dstRoot := tree.root(source), tree.root(target)
	if srcRoot == dstRoot {
		return source, target, false
	}
	if target != dstRoot && source == srcRoot {
		// Give the new outside subtree a place in the existing tree
		// without introducing a second parent at the inner endpoint.
		tree[srcRoot] = dstRoot
		return target, source, true
	}
	tree[dstRoot] = srcRoot
	return source, dstRoot, true
}

func (tree treeProjection) components(children []graph.NodeID) [][]graph.NodeID {
	var groups [][]graph.NodeID
	index := make(map[graph.NodeID]int)
	for _, id := range children {
		root := tree.root(id)
		i, found := index[root]
		if !found {
			i = len(groups)
			index[root] = i
			groups = append(groups, nil)
		}
		groups[i] = append(groups[i], id)
	}
	return groups
}
