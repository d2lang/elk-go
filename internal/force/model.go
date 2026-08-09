// Copyright (c) 2011, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package force

import (
	"math"

	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/javacompat"
)

const displacementBoundFactor = 16

type forceModel interface {
	layout(*FGraph, core.ProgressMonitor)
}

type abstractForceModel struct {
	graph             *FGraph
	random            *javacompat.Random
	displacementBound float64
}

func (model *abstractForceModel) initialize(fgraph *FGraph) {
	model.graph = fgraph
	model.random = fgraph.Random
	fgraph.CalcAdjacency()
	model.displacementBound = math.Max(
		float64(len(fgraph.Nodes)*displacementBoundFactor+len(fgraph.Edges)),
		displacementBoundFactor*displacementBoundFactor,
	)
	if !fgraph.Interactive {
		positionScale := float64(len(fgraph.Nodes))
		for _, node := range fgraph.Nodes {
			node.Position.X = model.random.NextDouble() * positionScale
			node.Position.Y = model.random.NextDouble() * positionScale
		}
	}
	for _, edge := range fgraph.Edges {
		for index := 0; index < edge.RepulsivePower; index++ {
			bendpoint := &FBendpoint{Particle: Particle{Priority: 1}, Edge: edge}
			bendpoint.Particle.bendEdge = edge
			edge.Bendpoints = append(edge.Bendpoints, bendpoint)
			fgraph.Bendpoints = append(fgraph.Bendpoints, bendpoint)
		}
		edge.DistributeBendpoints()
	}
}

func (model *abstractForceModel) iterationDone() {
	for _, edge := range model.graph.Edges {
		for _, label := range edge.Labels {
			label.RefreshPosition()
		}
		edge.DistributeBendpoints()
	}
}

func avoidSamePosition(random *javacompat.Random, forcer, forcee *Particle) {
	for forcer.Position.X-forcee.Position.X == 0 && forcer.Position.Y-forcee.Position.Y == 0 {
		forcer.Position.X += random.NextDouble() - 0.5
		forcer.Position.Y += random.NextDouble() - 0.5
		forcee.Position.X += random.NextDouble() - 0.5
		forcee.Position.Y += random.NextDouble() - 0.5
	}
}

type eadesModel struct {
	abstractForceModel
	maxIterations   int
	springLength    float64
	repulsionFactor float64
}

func (model *eadesModel) layout(fgraph *FGraph, monitor core.ProgressMonitor) {
	model.initialize(fgraph)
	model.maxIterations = fgraph.Iterations
	model.springLength = fgraph.SpacingNodeNode
	model.repulsionFactor = fgraph.Repulsion
	model.run(monitor, func(count int) bool { return count < model.maxIterations }, model.displacement)
}

func (model *eadesModel) displacement(forcer, forcee *Particle) geometry.Point {
	avoidSamePosition(model.random, forcer, forcee)
	displacement := forcee.Position.Sub(forcer.Position)
	length := displacement.Length()
	distance := math.Max(0, length-forcer.Radius()-forcee.Radius())
	connection := model.graph.Connection(forcer, forcee)
	var force float64
	if connection > 0 {
		force = -eadesAttractive(distance, model.springLength) * float64(connection)
	} else {
		force = eadesRepulsive(distance, model.repulsionFactor) * float64(forcer.Priority)
	}
	return displacement.Scale(force / length)
}

func eadesRepulsive(distance, factor float64) float64 {
	if distance > 0 {
		return factor / (distance * distance)
	}
	return factor * 100
}

func eadesAttractive(distance, springLength float64) float64 {
	if distance > 0 {
		return math.Log(distance / springLength)
	}
	return -100
}

func (model *eadesModel) run(
	monitor core.ProgressMonitor,
	moreIterations func(int) bool,
	calculate func(*Particle, *Particle) geometry.Point,
) {
	runForceIterations(&model.abstractForceModel, monitor, moreIterations, calculate, nil)
}

type fruchtermanReingoldModel struct {
	abstractForceModel
	temperature float64
	threshold   float64
	k           float64
}

func (model *fruchtermanReingoldModel) layout(fgraph *FGraph, monitor core.ProgressMonitor) {
	model.initialize(fgraph)
	model.temperature = fgraph.Temperature
	model.threshold = model.temperature / float64(fgraph.Iterations)
	totalWidth, totalHeight := 0.0, 0.0
	for _, node := range fgraph.Nodes {
		totalWidth += node.Size.X
		totalHeight += node.Size.Y
	}
	area := totalWidth * totalHeight
	c := fgraph.SpacingNodeNode * 0.01
	model.k = math.Sqrt(area/float64(2*len(fgraph.Nodes))) * c
	runForceIterations(
		&model.abstractForceModel,
		monitor,
		func(int) bool { return model.temperature > 0 },
		model.displacement,
		func() { model.temperature -= model.threshold },
	)
}

func (model *fruchtermanReingoldModel) displacement(forcer, forcee *Particle) geometry.Point {
	avoidSamePosition(model.random, forcer, forcee)
	displacement := forcee.Position.Sub(forcer.Position)
	length := displacement.Length()
	distance := math.Max(0, length-forcer.Radius()-forcee.Radius())
	force := frRepulsive(distance, model.k) * float64(forcer.Priority)
	if connection := model.graph.Connection(forcer, forcee); connection > 0 {
		force -= frAttractive(distance, model.k) * float64(connection)
	}
	return displacement.Scale(force * model.temperature / length)
}

func frRepulsive(distance, k float64) float64 {
	if distance > 0 {
		return k * k / distance
	}
	return k * k * 100
}

func frAttractive(distance, k float64) float64 { return distance * distance / k }

func runForceIterations(
	model *abstractForceModel,
	monitor core.ProgressMonitor,
	moreIterations func(int) bool,
	calculate func(*Particle, *Particle) geometry.Point,
	afterIteration func(),
) {
	monitor.Begin("Component Layout", 1)
	iterations := 0
	for moreIterations(iterations) && !monitor.Canceled() {
		particles := model.graph.Particles()
		for _, node := range model.graph.Nodes {
			for _, forcer := range particles {
				if forcer != &node.Particle {
					node.Displacement = node.Displacement.Add(calculate(forcer, &node.Particle))
				}
			}
		}
		for _, node := range model.graph.Nodes {
			node.Displacement = node.Displacement.Bound(
				-model.displacementBound, -model.displacementBound,
				model.displacementBound, model.displacementBound,
			)
			node.Position = node.Position.Add(node.Displacement)
			node.Displacement = geometry.Point{}
		}
		model.iterationDone()
		if afterIteration != nil {
			afterIteration()
		}
		iterations++
	}
	monitor.Done()
}
