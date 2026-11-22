// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package radial

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

// recursiveRouteEdges is routeEdges as it was before it was made
// iterative (its frame grew with the property map inside the section
// composite, so deep trees overflowed a 1 MiB stack sooner than in the
// original).
func recursiveRouteEdges(model *graph.Model, node graph.NodeID) {
	for _, edgeID := range model.Node(node).OutgoingEdges {
		edge := model.Edge(edgeID)
		if len(edge.Sources) == 0 || edge.Sources[0].Kind == graph.PortShape || len(edge.Targets) == 0 {
			continue
		}
		target := model.ShapeNode(edge.Targets[0])
		if edge.ContainingNode == nil || model.Node(node).Parent == nil || *edge.ContainingNode != *model.Node(node).Parent {
			continue
		}
		sourceShape, targetShape := model.Node(node).Shape, model.Node(target).Shape
		source := geometry.Point{X: sourceShape.X + sourceShape.Width/2, Y: sourceShape.Y + sourceShape.Height/2}
		end := geometry.Point{X: targetShape.X + targetShape.Width/2, Y: targetShape.Y + targetShape.Height/2}
		source, end = clippedEndpoints(source, end, geometry.Point{X: sourceShape.Width, Y: sourceShape.Height}, geometry.Point{X: targetShape.Width, Y: targetShape.Height})
		var sectionID graph.SectionID
		if len(edge.Sections) == 0 {
			sectionID = model.AddSection(edgeID)
		} else {
			sectionID = edge.Sections[0]
		}
		section := model.Section(sectionID)
		section.Start, section.End = source, end
		section.StartPresent, section.EndPresent = true, true
		section.BendPoints = nil
		section.BendPointsPresent = false
		recursiveRouteEdges(model, target)
	}
}

// randomTreeModel builds, from seed, a random tree (plus edges routeEdges
// skips: from ports, without targets, contained elsewhere; and DAG joins).
func randomTreeModel(seed int64) (*graph.Model, graph.NodeID) {
	random := rand.New(rand.NewSource(seed))
	model := graph.NewModel()
	parent := model.AddNode(&model.Root)
	var nodes []graph.NodeID
	for count := 1 + random.Intn(12); count > 0; count-- {
		node := model.AddNode(&parent)
		model.Node(node).Shape.SetDimensions(float64(1+random.Intn(30)), float64(1+random.Intn(30)))
		model.Node(node).Shape.SetLocation(float64(random.Intn(200)), float64(random.Intn(200)))
		nodes = append(nodes, node)
	}
	for index := 1; index < len(nodes); index++ {
		from := nodes[random.Intn(index)]
		edge := model.AddEdge(&parent)
		model.AddSource(edge, graph.NodeRef(from))
		model.AddTarget(edge, graph.NodeRef(nodes[index]))
		if random.Intn(4) == 0 {
			model.AddSection(edge) // an edge that has a section already
		}
		switch random.Intn(12) {
		case 0: // a second edge between the same nodes
			again := model.AddEdge(&parent)
			model.AddSource(again, graph.NodeRef(from))
			model.AddTarget(again, graph.NodeRef(nodes[index]))
		case 1: // an edge contained elsewhere
			other := model.AddEdge(&model.Root)
			model.AddSource(other, graph.NodeRef(from))
			model.AddTarget(other, graph.NodeRef(nodes[index]))
		case 2: // an edge without target
			loose := model.AddEdge(&parent)
			model.AddSource(loose, graph.NodeRef(from))
		}
	}
	return model, nodes[0]
}

func sectionsText(model *graph.Model) string {
	out := ""
	for index := range model.Edges {
		out += fmt.Sprint(index, model.Edges[index].Sections, ";")
	}
	for index := range model.Sections {
		section := &model.Sections[index]
		out += fmt.Sprint(section.Parent, section.Start, section.End, section.StartPresent, section.EndPresent,
			section.BendPoints == nil, section.BendPointsPresent, "|")
	}
	return out
}

// TestIterativeRouteEdgesMatchesRecursion compares the
// iterative routeEdges with the recursive original on random trees.
func TestIterativeRouteEdgesMatchesRecursion(t *testing.T) {
	for seed := int64(1); seed <= 3000; seed++ {
		want, wantRoot := randomTreeModel(seed)
		got, gotRoot := randomTreeModel(seed)
		recursiveRouteEdges(want, wantRoot)
		routeEdges(got, gotRoot)
		if sectionsText(want) != sectionsText(got) {
			t.Fatalf("seed %d:\nrecursive %s\niterative %s", seed, sectionsText(want), sectionsText(got))
		}
	}
}
