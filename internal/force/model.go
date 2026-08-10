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
		if forcer.bendEdge != nil && forcee.bendEdge != nil {
			// ELK 0.12 first separates coincident bend particles
			// orthogonally to their respective edge directions. Its second
			// update intentionally targets the first particle as well.
			firstVector := forcer.bendEdge.TargetPoint().Sub(forcer.bendEdge.SourcePoint())
			firstLength := forceVectorLength(firstVector)
			forcer.Position = forcer.Position.Add(geometry.Point{
				X: firstVector.X / firstLength * 2,
				Y: -firstVector.Y / firstLength * 2,
			})
			secondVector := forcee.bendEdge.TargetPoint().Sub(forcee.bendEdge.SourcePoint())
			secondLength := forceVectorLength(secondVector)
			length := 2.0
			if forcer.bendEdge == forcee.bendEdge {
				// Java compares the two freshly allocated KVector instances by
				// identity, so this branch is unreachable in the source too.
				length = 2
			}
			forcer.Position = forcer.Position.Add(geometry.Point{
				X: secondVector.X / secondLength * length,
				Y: -secondVector.Y / secondLength * length,
			})
		} else {
			forcer.Position.X += random.NextDouble() - 0.5
			forcer.Position.Y += random.NextDouble() - 0.5
			forcee.Position.X += random.NextDouble() - 0.5
			forcee.Position.Y += random.NextDouble() - 0.5
		}
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
	length := forceVectorLength(displacement)
	distance := math.Max(0, length-forcer.Radius()-forcee.Radius())
	connection := model.graph.Connection(forcer, forcee)
	var force float64
	if connection > 0 {
		force = -eadesAttractive(distance, model.springLength) * float64(connection)
	} else {
		force = eadesRepulsive(distance, model.repulsionFactor) * float64(forcer.Priority)
	}
	scale := float64(force / length)
	return displacement.Scale(scale)
}

func eadesRepulsive(distance, factor float64) float64 {
	if distance > 0 {
		distanceSquared := float64(distance * distance)
		return factor / distanceSquared
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
	length := forceVectorLength(displacement)
	distance := math.Max(0, length-forcer.Radius()-forcee.Radius())
	force := frRepulsive(distance, model.k) * float64(forcer.Priority)
	if connection := model.graph.Connection(forcer, forcee); connection > 0 {
		// Same-type conversions here and below are deliberate. The Go spec
		// otherwise permits fused floating-point evaluation across statements,
		// while GWT's generated JavaScript rounds these intermediate products.
		attraction := float64(frAttractive(distance, model.k) * float64(connection))
		force -= attraction
	}
	scaledForce := float64(force * model.temperature)
	scale := float64(scaledForce / length)
	return displacement.Scale(scale)
}

func forceVectorLength(vector geometry.Point) float64 {
	// Explicit conversions preserve the two multiplications as separately
	// rounded operations before the addition, matching KVector.length in GWT.
	xSquared := float64(vector.X * vector.X)
	ySquared := float64(vector.Y * vector.Y)
	return math.Sqrt(xSquared + ySquared)
}

func frRepulsive(distance, k float64) float64 {
	kSquared := float64(k * k)
	if distance > 0 {
		return kSquared / distance
	}
	return float64(kSquared * 100)
}

func frAttractive(distance, k float64) float64 {
	distanceSquared := float64(distance * distance)
	return distanceSquared / k
}

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
		model.iterationDone()
		if afterIteration != nil {
			afterIteration()
		}
		particles := model.graph.Particles()
		for _, forcee := range particles {
			for _, forcer := range particles {
				if forcer != forcee {
					forcee.Displacement = forcee.Displacement.Add(calculate(forcer, forcee))
				}
			}
		}
		for _, particle := range particles {
			particle.Displacement = particle.Displacement.Bound(
				-model.displacementBound, -model.displacementBound,
				model.displacementBound, model.displacementBound,
			)
			particle.Position = particle.Position.Add(particle.Displacement)
			particle.Displacement = geometry.Point{}
		}
		iterations++
	}
	monitor.Done()
}
