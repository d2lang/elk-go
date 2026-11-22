// Copyright (c) 2010, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package p5edges implements the fifth phase of ELK Layered 0.8.1.
package p5edges

import (
	"math"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
	"github.com/d2lang/elk-go/internal/layered"
	"github.com/d2lang/elk-go/internal/layered/p5edges/orthogonal"
)

// OrthogonalEdgeRouter assigns layer x coordinates and routes orthogonal
// hyperedges through the gaps between consecutive layers.
type OrthogonalEdgeRouter struct{}

var _ layered.ConfiguredProcessor = (*OrthogonalEdgeRouter)(nil)

func (*OrthogonalEdgeRouter) ProcessorConfiguration(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) layered.ProcessorConfiguration {
	graph := arena.Graph(graphID)
	properties := basegraph.Get(&graph.Properties, layered.GraphProperties)
	var configuration layered.ProcessorConfiguration
	if properties&layered.GraphPropertyHyperedges != 0 {
		configuration.
			AddBefore(layered.PhaseNodePlacement, layered.HyperedgeDummyMerger).
			AddBefore(layered.PhaseNodeOrdering, layered.InvertedPortProcessor)
	}
	if properties&layered.GraphPropertyNonFreePorts != 0 ||
		basegraph.Get(&graph.Properties, layered.FeedbackEdges) {
		configuration.AddBefore(layered.PhaseNodeOrdering, layered.InvertedPortProcessor)
		if properties&layered.GraphPropertyNorthSouthPorts != 0 {
			configuration.
				AddBefore(layered.PhaseNodeOrdering, layered.NorthSouthPortPreprocessor).
				AddAfter(layered.PhaseEdgeRouting, layered.NorthSouthPortPostprocessor)
		}
	}
	if properties&layered.GraphPropertyExternalPorts != 0 {
		configuration.
			AddBefore(layered.PhaseNodeOrdering, layered.HierarchicalPortConstraintProcessor).
			AddBefore(layered.PhaseNodePlacement, layered.HierarchicalPortDummySizeProcessor).
			AddAfter(layered.PhaseEdgeRouting, layered.HierarchicalPortOrthogonalEdgeRouter)
	}
	if properties&layered.GraphPropertySelfLoops != 0 {
		configuration.
			AddBefore(layered.PhaseCycleBreaking, layered.SelfLoopPreprocessor).
			AddBefore(layered.PhaseNodePlacement, layered.SelfLoopPortRestorer).
			AddBefore(layered.PhaseNodePlacement, layered.SelfLoopRouter).
			AddAfter(layered.PhaseEdgeRouting, layered.SelfLoopPostprocessor)
	}
	if properties&layered.GraphPropertyHypernodes != 0 {
		configuration.AddAfter(layered.PhaseEdgeRouting, layered.HypernodeProcessor)
	}
	if properties&layered.GraphPropertyCenterLabels != 0 {
		configuration.
			AddBefore(layered.PhaseLayering, layered.LabelDummyInserter).
			AddBefore(layered.PhaseNodePlacement, layered.LabelDummySwitcher).
			AddBefore(layered.PhaseNodePlacement, layered.LabelSideSelector).
			AddAfter(layered.PhaseEdgeRouting, layered.LabelDummyRemover)
	}
	if properties&layered.GraphPropertyEndLabels != 0 {
		configuration.
			AddBefore(layered.PhaseNodePlacement, layered.LabelSideSelector).
			AddBefore(layered.PhaseNodePlacement, layered.EndLabelPreprocessor).
			AddAfter(layered.PhaseEdgeRouting, layered.EndLabelPostprocessor)
	}
	return configuration
}

func (*OrthogonalEdgeRouter) Process(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	graph := arena.Graph(graphID)
	nodeNodeSpacing := basegraph.Get(&graph.Properties, layered.SpacingNodeNodeBetweenLayers)
	edgeEdgeSpacing := basegraph.Get(&graph.Properties, layered.SpacingEdgeEdgeBetweenLayers)
	edgeNodeSpacing := basegraph.Get(&graph.Properties, layered.SpacingEdgeNodeBetweenLayers)
	// A graph none of whose ports has an outgoing edge gives every
	// RouteEdges call no segment: it returns 0 slots, draws nothing and
	// changes nothing in the arena, except that the first call of a
	// generator for a graph gives the graph a Random when it has none. That
	// is all done here instead, without a generator.
	var generator *orthogonal.Generator
	if hasOutgoingEdges(arena, graph) {
		generator = orthogonal.NewGenerator(orthogonal.WestToEast, edgeEdgeSpacing)
	} else if basegraph.Get(&graph.Properties, layered.Random) == nil {
		basegraph.Set(&graph.Properties, layered.Random,
			javacompat.NewRandom(int64(basegraph.Get(&graph.Properties, layered.RandomSeed))))
	}

	xPosition := 0.0
	var leftLayer *layered.LayerID
	var leftNodes []layered.LNodeID
	leftLayerIndex := -1
	nextLayerIndex := 0
	for {
		var rightLayer *layered.LayerID
		var rightNodes []layered.LNodeID
		if nextLayerIndex < len(graph.Layers) {
			layerID := graph.Layers[nextLayerIndex]
			rightLayer = &layerID
			rightNodes = arena.Layer(layerID).Nodes
			nextLayerIndex++
		}
		rightLayerIndex := nextLayerIndex - 1
		if leftLayer != nil {
			placeNodesHorizontally(arena, *leftLayer, xPosition)
			xPosition += arena.Layer(*leftLayer).Size.X
		}
		startPosition := xPosition
		if leftLayer != nil {
			startPosition += edgeNodeSpacing
		}
		slots := 0
		if generator != nil {
			slots = generator.RouteEdges(arena, graphID, leftNodes, leftLayerIndex, rightNodes, startPosition)
		}
		leftExternal := allExternalWestOrEastPorts(arena, leftNodes)
		rightExternal := allExternalWestOrEastPorts(arena, rightNodes)
		if slots > 0 {
			routingWidth := float64(slots-1) * edgeEdgeSpacing
			if leftLayer != nil {
				routingWidth += edgeNodeSpacing
			}
			if rightLayer != nil {
				routingWidth += edgeNodeSpacing
			}
			if routingWidth < nodeNodeSpacing && !leftExternal && !rightExternal {
				routingWidth = nodeNodeSpacing
			}
			xPosition += routingWidth
		} else if !leftExternal && !rightExternal {
			xPosition += nodeNodeSpacing
		}
		leftLayer, leftNodes, leftLayerIndex = rightLayer, rightNodes, rightLayerIndex
		if rightLayer == nil {
			break
		}
	}
	graph.Size.X = xPosition
	if generator != nil {
		// The generator goes back to the pool only from a call that
		// returned: one that panics may leave its scratch half-updated, so
		// it is dropped with the call instead.
		generator.Release()
	}
	return nil
}

func allExternalWestOrEastPorts(arena *layered.LGraphArena, nodes []layered.LNodeID) bool {
	for _, nodeID := range nodes {
		node := arena.Node(nodeID)
		side := basegraph.Get(&node.Properties, layered.ExternalPortSide)
		if node.Type != layered.ExternalPortNode || side != layered.WestSide && side != layered.EastSide {
			return false
		}
	}
	return true
}

// placeNodesHorizontally ports LGraphUtil.placeNodesHorizontally. Automatic
// alignment biases a node according to its number of input and output ports.
func placeNodesHorizontally(arena *layered.LGraphArena, layerID layered.LayerID, xOffset float64) {
	layer := arena.Layer(layerID)
	maxLeftMargin, maxRightMargin := 0.0, 0.0
	for _, nodeID := range layer.Nodes {
		node := arena.Node(nodeID)
		maxLeftMargin = math.Max(maxLeftMargin, node.Margin.Left)
		maxRightMargin = math.Max(maxRightMargin, node.Margin.Right)
	}
	for _, nodeID := range layer.Nodes {
		node := arena.Node(nodeID)
		alignment := basegraph.Get(&node.Properties, layered.AlignmentOption)
		ratio := 0.0
		switch alignment {
		case layered.AlignmentLeft:
			ratio = 0
		case layered.AlignmentRight:
			ratio = 1
		case layered.AlignmentCenter:
			ratio = 0.5
		default:
			incomingPorts, outgoingPorts := 0, 0
			for _, portID := range node.Ports {
				port := arena.Port(portID)
				if len(port.IncomingEdges) > 0 {
					incomingPorts++
				}
				if len(port.OutgoingEdges) > 0 {
					outgoingPorts++
				}
			}
			if incomingPorts+outgoingPorts == 0 {
				ratio = 0.5
			} else {
				ratio = float64(outgoingPorts) / float64(incomingPorts+outgoingPorts)
			}
		}
		xPosition := (layer.Size.X - node.Size.X) * ratio
		if ratio > 0.5 {
			xPosition -= maxRightMargin * 2 * (ratio - 0.5)
		} else if ratio < 0.5 {
			xPosition += maxLeftMargin * 2 * (0.5 - ratio)
		}
		if xPosition < node.Margin.Left {
			xPosition = node.Margin.Left
		}
		maximum := layer.Size.X - node.Margin.Right - node.Size.X
		if xPosition > maximum {
			xPosition = maximum
		}
		node.Position.X = xOffset + xPosition
	}
}

// hasOutgoingEdges reports whether a port of a node in one of the graph's
// layers has an outgoing edge (RouteEdges makes segments only for such
// ports).
func hasOutgoingEdges(arena *layered.LGraphArena, graph *layered.LGraph) bool {
	for _, layerID := range graph.Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			for _, portID := range arena.Node(nodeID).Ports {
				if len(arena.Port(portID).OutgoingEdges) != 0 {
					return true
				}
			}
		}
	}
	return false
}
