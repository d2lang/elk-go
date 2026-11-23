// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package force

import (
	"math"
	"strings"
	"testing"

	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/selfloops"
)

func TestForceLoopFootprintInfluencesOnlyItsOccupiedSide(t *testing.T) {
	owner := FNode{Particle: Particle{Size: geometry.Point{X: 30, Y: 40}}, LoopPlan: &selfloops.Plan{Left: 60}}
	owner.Particle.node = &owner
	neighbor := FNode{Particle: Particle{Size: geometry.Point{X: 30, Y: 40}}}
	model := abstractForceModel{hasSelfLoops: true}
	for _, tc := range []struct{ dx, want float64 }{{-120, 10}, {120, 70}} {
		neighbor.Position.X = tc.dx
		delta := neighbor.Position.Sub(owner.Position)
		if got := model.surfaceDistance(&owner.Particle, &neighbor.Particle, delta, math.Abs(tc.dx)); got != tc.want {
			t.Errorf("surface distance in direction %v = %v, want %v", delta, got, tc.want)
		}
		ordinary := FEdge{Source: &owner, Target: &neighbor}
		if got := math.Abs(ordinary.SourcePoint().X - owner.Position.X); got != 15 {
			t.Errorf("ordinary edge clipped %v from center, want the 15-unit body half-width", got)
		}
	}
}

func TestForceModelsRouteAndPackLoopOwners(t *testing.T) {
	for _, modelName := range []string{ModelEades, ModelFruchtermanReingold} {
		t.Run(modelName, func(t *testing.T) {
			m := graph.NewModel()
			root := m.Node(m.Root)
			root.Properties.SetRaw(selfloops.RouteOption, true)
			graph.Set(&root.Properties, Model, modelName)
			graph.Set(&root.Properties, OmitNodeMicroLayout, true)
			// Two components, each with its own loop and a nearby connected node.
			var owners []graph.NodeID
			var loops []graph.EdgeID
			for i := 0; i < 2; i++ {
				owner, neighbor := m.AddNode(&m.Root), m.AddNode(&m.Root)
				owners = append(owners, owner)
				m.Node(owner).Shape.SetDimensions(40, 30)
				m.Node(neighbor).Shape.SetDimensions(40, 30)
				edge := m.AddEdge(&m.Root)
				m.AddSource(edge, graph.NodeRef(owner))
				m.AddTarget(edge, graph.NodeRef(neighbor))
				loop := m.AddEdge(&m.Root)
				m.AddSource(loop, graph.NodeRef(owner))
				m.AddTarget(loop, graph.NodeRef(owner))
				label := m.AddLabel(graph.EdgeElementRef(loop), "retry after a timeout")
				m.Label(label).Shape.SetDimensions(180, 15)
				loops = append(loops, loop)
			}
			if err := InitialLayout(m, m.Root, core.NullProgressMonitor); err != nil {
				t.Fatal(err)
			}
			for _, id := range loops {
				if got := len(m.Edge(id).Sections); got != 0 {
					t.Fatalf("Stress initialization published %d sections for loop %d", got, id)
				}
			}
			if err := (&Provider{}).Layout(m, m.Root, core.NullProgressMonitor); err != nil {
				t.Fatal(err)
			}
			bounds := m.Node(m.Root).Shape.Rectangle()
			var labels []geometry.Rectangle
			for i, id := range loops {
				edge := m.Edge(id)
				if len(edge.Sections) != 1 {
					t.Fatalf("loop %d has %d sections", id, len(edge.Sections))
				}
				section := m.Section(edge.Sections[0])
				if len(section.BendPoints) < 2 {
					t.Fatalf("loop %d has no detour: %+v", id, section)
				}
				body := m.Node(owners[i]).Shape.Rectangle()
				if body.Width != 40 || body.Height != 30 {
					t.Fatalf("loop changed visible body dimensions: %+v", body)
				}
				label := m.Label(edge.Labels[0]).Shape.Rectangle()
				if label.X < bounds.X || label.Y < bounds.Y || label.MaxX() > bounds.MaxX() || label.MaxY() > bounds.MaxY() {
					t.Errorf("label %+v outside root %+v", label, bounds)
				}
				for _, nodeID := range m.Node(m.Root).Children {
					if label.Intersects(m.Node(nodeID).Shape.Rectangle()) {
						t.Errorf("loop label %+v overlaps body of node %d", label, nodeID)
					}
				}
				for _, other := range labels {
					if label.Intersects(other) {
						t.Errorf("packed components' loop labels overlap: %+v and %+v", label, other)
					}
				}
				labels = append(labels, label)
			}
		})
	}
}

func TestForceLoopMustFitFixedParent(t *testing.T) {
	for _, width := range []float64{130, 300} {
		m := graph.NewModel()
		parent := m.Node(m.Root)
		parent.Identifier = "fixed canvas"
		parent.Properties.SetRaw(selfloops.RouteOption, true)
		graph.Set(&parent.Properties, NodeSizeFixedGraphSize, true)
		graph.Set(&parent.Properties, OmitNodeMicroLayout, true)
		// The 30-unit body and the default 50 units of padding per side
		// exactly fill the smaller parent before adding its loop.
		parent.Shape.SetDimensions(width, width)
		node := m.AddNode(&m.Root)
		m.Node(node).Shape.SetDimensions(30, 30)
		edge := m.AddEdge(&m.Root)
		m.AddSource(edge, graph.NodeRef(node))
		m.AddTarget(edge, graph.NodeRef(node))
		err := (&Provider{}).Layout(m, m.Root, core.NullProgressMonitor)
		if width == 130 {
			if err == nil || !strings.Contains(err.Error(), "fixed parent") {
				t.Fatalf("undersized parent: got %v", err)
			}
			if len(m.Edge(edge).Sections) != 0 {
				t.Fatal("failed layout published a loop outside the fixed parent")
			}
		} else if err != nil || len(m.Edge(edge).Sections) != 1 {
			t.Fatalf("sufficient fixed parent: sections = %v, err = %v", m.Edge(edge).Sections, err)
		}
		if parent.Shape.Width != width || parent.Shape.Height != width {
			t.Errorf("fixed parent resized to %+v", parent.Shape.Rectangle())
		}
	}
}
