// Copyright (c) 2011, 2015 Kiel University and others.
// Copyright (c) 2011, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package force

import (
	"math"

	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
)

// Particle is ELK's FParticle. Position is a center for nodes and the
// top-left corner for labels, matching the Java implementation.
type Particle struct {
	Position     geometry.Point
	Size         geometry.Point
	Displacement geometry.Point
	Priority     int
	node         *FNode
	bendEdge     *FEdge
}

func (particle *Particle) Radius() float64 { return forceVectorLength(particle.Size) / 2 }

type FNode struct {
	Particle
	ID     int
	Origin graph.NodeID
}

type FLabel struct {
	Particle
	Origin  graph.LabelID
	Edge    *FEdge
	Text    string
	Inline  bool
	Spacing float64
}

type FBendpoint struct {
	Particle
	Edge *FEdge
}

type FEdge struct {
	Origin         graph.EdgeID
	Source, Target *FNode
	Bendpoints     []*FBendpoint
	Labels         []*FLabel
	Priority       int
	RepulsivePower int
}

type FGraph struct {
	Nodes      []*FNode
	Edges      []*FEdge
	Labels     []*FLabel
	Bendpoints []*FBendpoint
	Adjacency  [][]int

	Origin                      graph.NodeID
	Model                       string
	Iterations                  int
	Temperature                 float64
	Repulsion                   float64
	SpacingNodeNode             float64
	AspectRatio                 float64
	SeparateConnectedComponents bool
	Interactive                 bool
	Padding                     geometry.Spacing
	Random                      *javacompat.Random

	Priority   int
	BBUpLeft   geometry.Point
	BBLowRight geometry.Point
}

func (fgraph *FGraph) Particles() []*Particle {
	particles := make([]*Particle, 0, len(fgraph.Nodes)+len(fgraph.Labels)+len(fgraph.Bendpoints))
	for _, node := range fgraph.Nodes {
		particles = append(particles, &node.Particle)
	}
	for _, label := range fgraph.Labels {
		particles = append(particles, &label.Particle)
	}
	for _, bendpoint := range fgraph.Bendpoints {
		particles = append(particles, &bendpoint.Particle)
	}
	return particles
}

func (fgraph *FGraph) CalcAdjacency() {
	fgraph.Adjacency = make([][]int, len(fgraph.Nodes))
	for index := range fgraph.Adjacency {
		fgraph.Adjacency[index] = make([]int, len(fgraph.Nodes))
	}
	for _, edge := range fgraph.Edges {
		fgraph.Adjacency[edge.Source.ID][edge.Target.ID] += edge.Priority
	}
}

func (fgraph *FGraph) Connection(forcer *Particle, forcee *Particle) int {
	if forcer.node != nil && forcee.node != nil {
		return fgraph.Adjacency[forcer.node.ID][forcee.node.ID] +
			fgraph.Adjacency[forcee.node.ID][forcer.node.ID]
	}
	if forcer.bendEdge != nil && forcee.bendEdge != nil && forcer.bendEdge == forcee.bendEdge {
		return forcee.bendEdge.Priority
	}
	return 0
}

func (edge *FEdge) SourcePoint() geometry.Point {
	vector := edge.Target.Position.Sub(edge.Source.Position)
	return edge.Source.Position.Add(clipVector(vector, edge.Source.Size.X, edge.Source.Size.Y))
}

func (edge *FEdge) TargetPoint() geometry.Point {
	vector := edge.Source.Position.Sub(edge.Target.Position)
	return edge.Target.Position.Add(clipVector(vector, edge.Target.Size.X, edge.Target.Size.Y))
}

func clipVector(vector geometry.Point, width, height float64) geometry.Point {
	halfWidth, halfHeight := width/2, height/2
	xscale, yscale := 1.0, 1.0
	if math.Abs(vector.X) > halfWidth {
		xscale = halfWidth / math.Abs(vector.X)
	}
	if math.Abs(vector.Y) > halfHeight {
		yscale = halfHeight / math.Abs(vector.Y)
	}
	return vector.Scale(math.Min(xscale, yscale))
}

func (edge *FEdge) DistributeBendpoints() {
	count := len(edge.Bendpoints)
	if count == 0 {
		return
	}
	factor := float64(1 / float64(count+1))
	difference := edge.Target.Position.Sub(edge.Source.Position)
	increment := geometry.Point{
		X: float64(difference.X * factor),
		Y: float64(difference.Y * factor),
	}
	position := edge.Source.Position
	for _, bendpoint := range edge.Bendpoints {
		position = position.Add(increment)
		bendpoint.Position = position
	}
}

func (label *FLabel) RefreshPosition() {
	source := label.Edge.Source.Position
	target := label.Edge.Target.Position
	if label.Inline {
		label.Position = source.Add(target.Sub(source).Scale(0.5)).Sub(label.Size.Scale(0.5))
		return
	}
	spacing := label.Spacing
	position := label.Position
	if source.X >= target.X {
		if source.Y >= target.Y {
			position.X = target.X + (source.X-target.X)/2 + spacing
			position.Y = target.Y + (source.Y-target.Y)/2 - spacing - label.Size.Y
		} else {
			position.X = target.X + (source.X-target.X)/2 + spacing
			position.Y = source.Y + (target.Y-source.Y)/2 + spacing
		}
	} else if source.Y >= target.Y {
		position.X = source.X + (target.X-source.X)/2 + spacing
		position.Y = target.Y + (source.Y-target.Y)/2 + spacing
	} else {
		position.X = source.X + (target.X-source.X)/2 + spacing
		position.Y = source.Y + (target.Y-source.Y)/2 - spacing - label.Size.Y
	}
	label.Position = position
}
