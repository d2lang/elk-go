// Copyright (c) 2008, 2020 Kiel University and others.
// Copyright (c) 2011, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package force

import (
	"fmt"
	"math"
	"time"

	"github.com/d2lang/elk-go/internal/common/nodespacing"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
	"github.com/d2lang/elk-go/internal/selfloops"
)

// Importer is the Go counterpart of ELK 0.8.1's ElkGraphImporter. It retains
// arena IDs rather than Java origin-object properties.
type Importer struct {
	model  *graph.Model
	parent graph.NodeID
	nodes  map[graph.NodeID]*FNode
}

func NewImporter(model *graph.Model, parent graph.NodeID) *Importer {
	return &Importer{model: model, parent: parent, nodes: make(map[graph.NodeID]*FNode)}
}

func (importer *Importer) ImportGraph() (*FGraph, error) {
	parent := importer.model.Node(importer.parent)
	fgraph := &FGraph{
		Origin:                      importer.parent,
		Model:                       graph.Get(&parent.Properties, Model),
		Iterations:                  graph.Get(&parent.Properties, Iterations),
		Temperature:                 graph.Get(&parent.Properties, Temperature),
		Repulsion:                   graph.Get(&parent.Properties, Repulsion),
		SpacingNodeNode:             graph.Get(&parent.Properties, SpacingNodeNode),
		AspectRatio:                 graph.Get(&parent.Properties, AspectRatio),
		SeparateConnectedComponents: graph.Get(&parent.Properties, SeparateConnectedComponents),
		Interactive:                 graph.Get(&parent.Properties, Interactive),
		Padding:                     graph.Get(&parent.Properties, Padding),
	}
	seed := graph.Get(&parent.Properties, RandomSeed)
	if seed == 0 {
		// Java's new Random() is intentionally nondeterministic for seed zero.
		// The precise seed-uniquifier value is not stable across JVMs either.
		fgraph.Random = javacompat.NewRandom(time.Now().UnixNano())
	} else {
		fgraph.Random = javacompat.NewRandom(int64(seed))
	}

	loopPlans := selfloops.Collect(importer.model, importer.parent)
	for index, nodeID := range parent.Children {
		node := importer.model.Node(nodeID)
		label := ""
		if len(node.Labels) > 0 {
			label = importer.model.Label(node.Labels[0]).Text
		}
		_ = label // Retained in Java for toString/debug output only.
		fnode := &FNode{
			Particle: Particle{
				Position: node.Shape.Position().Add(geometry.Point{X: node.Shape.Width / 2, Y: node.Shape.Height / 2}),
				Size:     geometry.Point{X: math.Max(node.Shape.Width, 1), Y: math.Max(node.Shape.Height, 1)},
				Priority: graph.Get(&node.Properties, Priority),
			},
			ID:       index,
			Origin:   nodeID,
			LoopPlan: loopPlans[nodeID],
		}
		fnode.Particle.node = fnode
		fgraph.Nodes = append(fgraph.Nodes, fnode)
		importer.nodes[nodeID] = fnode
	}

	// ElkGraphUtil.allOutgoingEdges includes edges connected through ports.
	// Iterating the model edge arena for each child preserves JSON/EMF order.
	for _, sourceID := range parent.Children {
		for edgeIndex := range importer.model.Edges {
			edgeID := graph.EdgeID(edgeIndex)
			edge := importer.model.Edge(edgeID)
			if len(edge.Sources) == 0 || importer.model.ShapeNode(edge.Sources[0]) != sourceID {
				continue
			}
			if len(edge.Sources) != 1 || len(edge.Targets) != 1 {
				return nil, fmt.Errorf("graph must not contain hyperedges")
			}
			targetID := importer.model.ShapeNode(edge.Targets[0])
			sourceNode, sourceFound := importer.nodes[sourceID]
			targetNode, targetFound := importer.nodes[targetID]
			// As in ELK, only edges between distinct local nodes become springs.
			// An opted-in loop is stored on its owner’s LoopPlan.
			if !sourceFound || !targetFound || sourceID == targetID {
				continue
			}
			fedge := &FEdge{
				Origin:         edgeID,
				Source:         sourceNode,
				Target:         targetNode,
				Priority:       graph.Get(&edge.Properties, Priority),
				RepulsivePower: graph.Get(&edge.Properties, RepulsivePower),
			}
			fgraph.Edges = append(fgraph.Edges, fedge)
			for _, labelID := range edge.Labels {
				label := importer.model.Label(labelID)
				flabel := &FLabel{
					Particle: Particle{
						Size:     geometry.Point{X: math.Max(label.Shape.Width, 1), Y: math.Max(label.Shape.Height, 1)},
						Priority: graph.Get(&label.Properties, Priority),
					},
					Origin:  labelID,
					Edge:    fedge,
					Text:    label.Text,
					Inline:  graph.Get(&label.Properties, EdgeLabelsInline),
					Spacing: graph.Get(&edge.Properties, SpacingEdgeLabel),
				}
				flabel.RefreshPosition()
				fedge.Labels = append(fedge.Labels, flabel)
				fgraph.Labels = append(fgraph.Labels, flabel)
			}
		}
	}
	return fgraph, nil
}

// ApplyLayout transfers FGraph coordinates back to the core graph, preserving
// ELK's mutation order (nodes are offset before edge endpoints are clipped).
func (importer *Importer) ApplyLayout(fgraph *FGraph) error {
	return importer.applyLayout(fgraph, true)
}

func (importer *Importer) applyLayout(fgraph *FGraph, writeLoops bool) error {
	minX, minY := float64(math.MaxInt32), float64(math.MaxInt32)
	maxX, maxY := float64(math.MinInt32), float64(math.MinInt32)
	hasLoops := false
	for _, node := range fgraph.Nodes {
		hasLoops = hasLoops || node.LoopPlan != nil
		topLeft, bottomRight := node.OccupiedBounds()
		minX = math.Min(minX, topLeft.X)
		minY = math.Min(minY, topLeft.Y)
		maxX = math.Max(maxX, bottomRight.X)
		maxY = math.Max(maxY, bottomRight.Y)
	}
	for _, bendpoint := range fgraph.Bendpoints {
		minX = math.Min(minX, bendpoint.Position.X-bendpoint.Size.X/2)
		minY = math.Min(minY, bendpoint.Position.Y-bendpoint.Size.Y/2)
		maxX = math.Max(maxX, bendpoint.Position.X+bendpoint.Size.X/2)
		maxY = math.Max(maxY, bendpoint.Position.Y+bendpoint.Size.Y/2)
	}
	width := maxX - minX + fgraph.Padding.Horizontal()
	height := maxY - minY + fgraph.Padding.Vertical()
	parentNode := importer.model.Node(importer.parent)
	fixedSize := graph.Get(&parentNode.Properties, NodeSizeFixedGraphSize)
	// Stress's initial Force pass supplies provisional positions. Validate the
	// final layout only, before transferring its node positions or edge sections.
	if writeLoops && hasLoops && fixedSize && (width > parentNode.Shape.Width+1e-8 || height > parentNode.Shape.Height+1e-8) {
		return fmt.Errorf("self-loop layout with padding requires %.2f x %.2f, exceeding fixed parent %q (%.2f x %.2f)", width, height, parentNode.Identifier, parentNode.Shape.Width, parentNode.Shape.Height)
	}
	offset := geometry.Point{X: fgraph.Padding.Left - minX, Y: fgraph.Padding.Top - minY}

	for _, fnode := range fgraph.Nodes {
		node := importer.model.Node(fnode.Origin)
		position := fnode.Position.Add(offset)
		node.Shape.SetLocation(position.X-node.Shape.Width/2, position.Y-node.Shape.Height/2)
		if plan := fnode.LoopPlan; plan != nil {
			if writeLoops {
				plan.Write(importer.model, node.Shape.X, node.Shape.Y)
			} else {
				plan.TranslateExisting(importer.model, node.Shape.X, node.Shape.Y)
			}
		}
	}
	for _, fedge := range fgraph.Edges {
		edge := importer.model.Edge(fedge.Origin)
		var sectionID graph.SectionID
		if len(edge.Sections) == 0 {
			sectionID = importer.model.AddSection(fedge.Origin)
		} else {
			sectionID = edge.Sections[0]
		}
		section := importer.model.Section(sectionID)
		section.Start = fedge.SourcePoint().Add(offset)
		section.BendPoints = section.BendPoints[:0]
		for _, bendpoint := range fedge.Bendpoints {
			section.BendPoints = append(section.BendPoints, bendpoint.Position.Add(offset))
		}
		section.End = fedge.TargetPoint().Add(offset)
		section.StartPresent, section.EndPresent = true, true
	}
	for _, flabel := range fgraph.Labels {
		label := importer.model.Label(flabel.Origin)
		position := flabel.Position.Add(offset)
		label.Shape.SetLocation(position.X, position.Y)
	}

	if !fixedSize {
		resizeNode(importer.model, importer.parent, width, height)
	}
	graph.Set(&parentNode.Properties, core.ChildAreaWidth, width-fgraph.Padding.Horizontal())
	graph.Set(&parentNode.Properties, core.ChildAreaHeight, height-fgraph.Padding.Vertical())
	return nil
}

// resizeNode mirrors ElkUtil.resizeNode(node, width, height, false, true).
func resizeNode(model *graph.Model, nodeID graph.NodeID, width, height float64) {
	node := model.Node(nodeID)
	oldWidth, oldHeight := node.Shape.Width, node.Shape.Height
	constraints := graph.Get(&node.Properties, core.NodeSizeConstraints)
	if hasString(constraints, "MINIMUM_SIZE") {
		minimum := graph.Get(&node.Properties, core.NodeSizeMinimum)
		if hasString(graph.Get(&node.Properties, nodespacing.NodeSizeOptions), "DEFAULT_MINIMUM_SIZE") {
			if minimum.X <= 0 {
				minimum.X = 20
			}
			if minimum.Y <= 0 {
				minimum.Y = 20
			}
		}
		width = math.Max(width, minimum.X)
		height = math.Max(height, minimum.Y)
	}
	widthDiff, heightDiff := width-oldWidth, height-oldHeight
	node.Shape.SetDimensions(width, height)
	for _, labelID := range node.Labels {
		label := model.Label(labelID)
		middleX := label.Shape.X + label.Shape.Width/2
		middleY := label.Shape.Y + label.Shape.Height/2
		widthPercent := middleX / oldWidth
		heightPercent := middleY / oldHeight
		if widthPercent+heightPercent >= 1 {
			if widthPercent-heightPercent > 0 && middleY >= 0 {
				label.Shape.X += widthDiff
				label.Shape.Y += heightDiff * heightPercent
				label.Shape.Presence |= graph.HasX | graph.HasY
			} else if widthPercent-heightPercent < 0 && middleX >= 0 {
				label.Shape.X += widthDiff * widthPercent
				label.Shape.Y += heightDiff
				label.Shape.Presence |= graph.HasX | graph.HasY
			}
		}
	}
	graph.Set(&node.Properties, core.NodeSizeConstraints, []string(nil))
}

func hasString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
