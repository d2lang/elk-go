// Copyright (c) 2010, 2020 Kiel University and others.
// Copyright (c) 2014, 2015 Kiel University and others.
// Copyright (c) 2017, 2018 Kiel University and others.
// Copyright (c) 2018 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p5edges

import (
	"fmt"
	"math"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
	"github.com/d2lang/elk-go/internal/layered"
)

const splineStraightVerticalDifference = 0.2

// SplineEdgeRouter ports the ELK 0.8.1 spline segment dependency graph,
// ranking, layer spacing, and default sloppy control-point construction.
type SplineEdgeRouter struct{}

type splineSide uint8

const (
	splineLeft splineSide = iota
	splineRight
)

type splineDependency struct {
	source, target *splineSegment
	weight         int
}

type splineSegment struct {
	edge                  layered.LEdgeID
	leftPorts, rightPorts []layered.LPortID
	outgoing, incoming    []*splineDependency
	mark                  int
	inWeight, outWeight   int
	rank                  int
	straight              bool
	topY, bottomY         float64
	centerY               float64
	sourceY, targetY      float64
	normalSource          bool
	normalTarget          bool
	boundingX             float64
	boundingWidth         float64
	xDelta                float64
	westOfInitialLayer    bool
}

func (*SplineEdgeRouter) ProcessorConfiguration(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) layered.ProcessorConfiguration {
	properties := basegraph.Get(&arena.Graph(graphID).Properties, layered.GraphProperties)
	var configuration layered.ProcessorConfiguration
	configuration.
		AddBefore(layered.PhaseNodeOrdering, layered.InvertedPortProcessor).
		AddAfter(layered.PhaseEdgeRouting, layered.FinalSplineBendpointsCalculator)
	if properties&layered.GraphPropertySelfLoops != 0 {
		configuration.
			AddBefore(layered.PhaseCycleBreaking, layered.SelfLoopPreprocessor).
			AddBefore(layered.PhaseNodePlacement, layered.SelfLoopPortRestorer).
			AddBefore(layered.PhaseNodePlacement, layered.SelfLoopRouter).
			AddAfter(layered.PhaseEdgeRouting, layered.SelfLoopPostprocessor)
	}
	if properties&layered.GraphPropertyCenterLabels != 0 {
		configuration.
			AddBefore(layered.PhaseLayering, layered.LabelDummyInserter).
			AddBefore(layered.PhaseNodePlacement, layered.LabelDummySwitcher).
			AddBefore(layered.PhaseNodePlacement, layered.LabelSideSelector).
			AddAfter(layered.PhaseEdgeRouting, layered.LabelDummyRemover)
	}
	if properties&layered.GraphPropertyNorthSouthPorts != 0 {
		configuration.
			AddBefore(layered.PhaseNodeOrdering, layered.NorthSouthPortPreprocessor).
			AddBefore(layered.PhaseEdgeRouting, layered.NorthSouthPortPostprocessor)
	}
	if properties&layered.GraphPropertyEndLabels != 0 {
		configuration.
			AddBefore(layered.PhaseNodePlacement, layered.LabelSideSelector).
			AddBefore(layered.PhaseNodePlacement, layered.EndLabelPreprocessor).
			AddAfter(layered.PhaseEdgeRouting, layered.EndLabelPostprocessor)
	}
	return configuration
}

func (*SplineEdgeRouter) Process(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	graph := arena.Graph(graphID)
	if len(graph.Layers) == 0 {
		graph.Size.X = 0
		return nil
	}
	mode := basegraph.Get(&graph.Properties, layered.SplineRoutingModeOption)
	if mode != layered.SplineRoutingSloppy && mode != layered.SplineRoutingConservative {
		return fmt.Errorf("layered: spline routing mode %v is not implemented", mode)
	}

	nodeSpacing := basegraph.Get(&graph.Properties, layered.SpacingNodeNodeBetweenLayers)
	edgeNodeSpacing := basegraph.Get(&graph.Properties, layered.SpacingEdgeNodeBetweenLayers)
	edgeSpacing := basegraph.Get(&graph.Properties, layered.SpacingEdgeEdgeBetweenLayers)
	sloppyFactor := basegraph.Get(&graph.Properties, layered.SplineSloppyLayerSpacingFactor)
	random := basegraph.Get(&graph.Properties, layered.Random)
	if random == nil {
		random = javacompat.NewRandom(1)
	}

	firstLayer := graph.Layers[0]
	leftExternal := allExternalWestOrEastPorts(arena, arena.Layer(firstLayer).Nodes)
	lastLayer := graph.Layers[len(graph.Layers)-1]
	rightExternal := allExternalWestOrEastPorts(arena, arena.Layer(lastLayer).Nodes)

	xPosition := 0.0
	var leftLayer *layered.LayerID
	for pairIndex := 0; pairIndex <= len(graph.Layers); pairIndex++ {
		var rightLayer *layered.LayerID
		if pairIndex < len(graph.Layers) {
			copyID := graph.Layers[pairIndex]
			rightLayer = &copyID
		}
		segments := collectSplineSegments(arena, leftLayer, rightLayer)
		createSplineDependencies(arena, segments)
		breakSplineCycles(segments, random)
		numberSplineSegments(segments)
		slotCount := 0
		for _, segment := range segments {
			if !segment.straight {
				slotCount = max(slotCount, segment.rank+1)
			}
		}

		rightPosition := xPosition
		xDelta := 0.0
		specialLeft := leftLayer == nil || leftExternal && *leftLayer == firstLayer
		specialRight := rightLayer == nil || rightExternal && *rightLayer == lastLayer
		if slotCount > 0 {
			increment := 0.0
			if leftLayer != nil {
				increment += edgeNodeSpacing
			}
			increment += float64(slotCount-1) * edgeSpacing
			if rightLayer != nil {
				increment += edgeNodeSpacing
				increment = math.Max(increment, splineSloppySpacing(arena, *rightLayer, edgeSpacing, nodeSpacing, sloppyFactor))
			}
			if increment < nodeSpacing && !specialLeft && !specialRight {
				xDelta = (nodeSpacing - increment) / 2
				increment = nodeSpacing
			}
			rightPosition += increment
		} else if !specialLeft && !specialRight {
			rightPosition += nodeSpacing
		}
		if rightLayer != nil {
			placeNodesHorizontally(arena, *rightLayer, rightPosition)
		}
		for _, segment := range segments {
			segment.boundingX = xPosition
			segment.boundingWidth = rightPosition - xPosition
			segment.xDelta = xDelta
			segment.westOfInitialLayer = leftLayer == nil
			writeSplineControls(arena, segment, mode, edgeNodeSpacing, edgeSpacing)
		}
		xPosition = rightPosition
		if rightLayer != nil {
			xPosition += arena.Layer(*rightLayer).Size.X
		}
		leftLayer = rightLayer
	}
	graph.Size.X = xPosition
	return nil
}

func collectSplineSegments(
	arena *layered.LGraphArena,
	leftLayer, rightLayer *layered.LayerID,
) []*splineSegment {
	leftPorts := make(map[layered.LPortID]struct{})
	rightPorts := make(map[layered.LPortID]struct{})
	edges := make([]layered.LEdgeID, 0)
	seenEdges := make(map[layered.LEdgeID]struct{})
	addEdge := func(edgeID layered.LEdgeID) {
		if _, found := seenEdges[edgeID]; !found {
			seenEdges[edgeID] = struct{}{}
			edges = append(edges, edgeID)
		}
	}
	if leftLayer != nil {
		for _, nodeID := range arena.Layer(*leftLayer).Nodes {
			for _, sourcePort := range arena.NodePortsOnSide(nodeID, layered.EastSide) {
				leftPorts[sourcePort] = struct{}{}
				for _, edgeID := range arena.Port(sourcePort).OutgoingEdges {
					if arena.EdgeIsSelfLoop(edgeID) {
						continue
					}
					targetPort := *arena.Edge(edgeID).Target
					targetNode := arena.Node(*arena.Port(targetPort).Node)
					if rightLayer != nil && targetNode.Layer != nil && *targetNode.Layer == *rightLayer {
						rightPorts[targetPort] = struct{}{}
						addEdge(edgeID)
					} else if targetNode.Layer != nil && *targetNode.Layer == *leftLayer {
						leftPorts[targetPort] = struct{}{}
						addEdge(edgeID)
					}
				}
			}
		}
	}
	if rightLayer != nil {
		for _, nodeID := range arena.Layer(*rightLayer).Nodes {
			for _, sourcePort := range arena.NodePortsOnSide(nodeID, layered.WestSide) {
				rightPorts[sourcePort] = struct{}{}
				for _, edgeID := range arena.Port(sourcePort).OutgoingEdges {
					if arena.EdgeIsSelfLoop(edgeID) {
						continue
					}
					targetPort := *arena.Edge(edgeID).Target
					targetNode := arena.Node(*arena.Port(targetPort).Node)
					if leftLayer != nil && targetNode.Layer != nil && *targetNode.Layer == *leftLayer {
						leftPorts[targetPort] = struct{}{}
						addEdge(edgeID)
					} else if targetNode.Layer != nil && *targetNode.Layer == *rightLayer {
						rightPorts[targetPort] = struct{}{}
						addEdge(edgeID)
					}
				}
			}
		}
	}

	segments := make([]*splineSegment, 0, len(edges))
	for _, edgeID := range edges {
		edge := arena.Edge(edgeID)
		sourcePort, targetPort := *edge.Source, *edge.Target
		segment := &splineSegment{edge: edgeID}
		if _, found := leftPorts[sourcePort]; found {
			segment.leftPorts = append(segment.leftPorts, sourcePort)
		} else {
			segment.rightPorts = append(segment.rightPorts, sourcePort)
		}
		if _, found := leftPorts[targetPort]; found {
			segment.leftPorts = append(segment.leftPorts, targetPort)
		} else {
			segment.rightPorts = append(segment.rightPorts, targetPort)
		}
		sourceY, targetY := absolutePortAnchor(arena, sourcePort).Y, absolutePortAnchor(arena, targetPort).Y
		segment.sourceY, segment.targetY = sourceY, targetY
		segment.normalSource = splineNormalNode(arena.Node(arena.EdgeSourceNode(edgeID)).Type)
		segment.normalTarget = splineNormalNode(arena.Node(arena.EdgeTargetNode(edgeID)).Type)
		segment.straight = math.Abs(sourceY-targetY) < splineStraightVerticalDifference
		segment.centerY = (sourceY + targetY) / 2
		if sourceY < targetY {
			segment.topY = 0.1*segment.centerY + 0.9*sourceY
			segment.bottomY = 0.1*segment.centerY + 0.9*targetY
		} else {
			segment.topY = 0.1*segment.centerY + 0.9*targetY
			segment.bottomY = 0.1*segment.centerY + 0.9*sourceY
		}
		segments = append(segments, segment)
	}
	return segments
}

func createSplineDependencies(arena *layered.LGraphArena, segments []*splineSegment) {
	for firstIndex, first := range segments {
		for _, second := range segments[firstIndex+1:] {
			if first.topY > second.bottomY || second.topY > first.bottomY {
				continue
			}
			firstCounter, secondCounter := 0, 0
			for _, portID := range first.rightPorts {
				if splineBetween(absolutePortAnchor(arena, portID).Y, second.topY, second.bottomY) {
					firstCounter++
				}
			}
			for _, portID := range first.leftPorts {
				if splineBetween(absolutePortAnchor(arena, portID).Y, second.topY, second.bottomY) {
					firstCounter--
				}
			}
			for _, portID := range second.rightPorts {
				if splineBetween(absolutePortAnchor(arena, portID).Y, first.topY, first.bottomY) {
					secondCounter++
				}
			}
			for _, portID := range second.leftPorts {
				if splineBetween(absolutePortAnchor(arena, portID).Y, first.topY, first.bottomY) {
					secondCounter--
				}
			}
			if firstCounter < secondCounter {
				addSplineDependency(first, second, secondCounter-firstCounter)
			} else if secondCounter < firstCounter {
				addSplineDependency(second, first, firstCounter-secondCounter)
			} else {
				addSplineDependency(second, first, 0)
				addSplineDependency(first, second, 0)
			}
		}
	}
}

func splineBetween(value, first, second float64) bool {
	return value >= math.Min(first, second) && value <= math.Max(first, second)
}

func addSplineDependency(source, target *splineSegment, weight int) {
	dependency := &splineDependency{source: source, target: target, weight: weight}
	source.outgoing = append(source.outgoing, dependency)
	target.incoming = append(target.incoming, dependency)
}

func breakSplineCycles(segments []*splineSegment, random *javacompat.Random) {
	sources, sinks := make([]*splineSegment, 0), make([]*splineSegment, 0)
	nextMark := -1
	for _, segment := range segments {
		segment.mark = nextMark
		nextMark--
		segment.inWeight, segment.outWeight = 0, 0
		for _, dependency := range segment.outgoing {
			segment.outWeight += dependency.weight
		}
		for _, dependency := range segment.incoming {
			segment.inWeight += dependency.weight
		}
		if segment.outWeight == 0 {
			sinks = append(sinks, segment)
		} else if segment.inWeight == 0 {
			sources = append(sources, segment)
		}
	}
	unprocessed := append([]*splineSegment(nil), segments...)
	markBase := len(segments)
	nextLeft, nextRight := markBase+1, markBase-1
	for len(unprocessed) > 0 {
		for len(sinks) > 0 {
			segment := sinks[0]
			sinks = sinks[1:]
			if !removeSplineSegment(&unprocessed, segment) {
				continue
			}
			segment.mark = nextRight
			nextRight--
			updateSplineNeighbors(segment, &sources, &sinks)
		}
		for len(sources) > 0 {
			segment := sources[0]
			sources = sources[1:]
			if !removeSplineSegment(&unprocessed, segment) {
				continue
			}
			segment.mark = nextLeft
			nextLeft++
			updateSplineNeighbors(segment, &sources, &sinks)
		}
		if len(unprocessed) == 0 {
			break
		}
		maximumOutflow := math.MinInt
		maximum := make([]*splineSegment, 0)
		for _, segment := range unprocessed {
			outflow := segment.outWeight - segment.inWeight
			if outflow >= maximumOutflow {
				if outflow > maximumOutflow {
					maximum = maximum[:0]
					maximumOutflow = outflow
				}
				maximum = append(maximum, segment)
			}
		}
		chosen := maximum[random.NextIntN(int32(len(maximum)))]
		removeSplineSegment(&unprocessed, chosen)
		chosen.mark = nextLeft
		nextLeft++
		updateSplineNeighbors(chosen, &sources, &sinks)
	}
	shiftBase := len(segments) + 1
	for _, segment := range segments {
		if segment.mark < markBase {
			segment.mark += shiftBase
		}
	}
	for _, source := range segments {
		retained := source.outgoing[:0]
		for _, dependency := range source.outgoing {
			if source.mark <= dependency.target.mark {
				retained = append(retained, dependency)
				continue
			}
			dependency.target.incoming = removeSplineDependency(dependency.target.incoming, dependency)
			if dependency.weight > 0 {
				oldTarget := dependency.target
				dependency.source = oldTarget
				oldTarget.outgoing = append(oldTarget.outgoing, dependency)
				dependency.target = source
				source.incoming = append(source.incoming, dependency)
			}
		}
		source.outgoing = retained
	}
}

func removeSplineSegment(list *[]*splineSegment, wanted *splineSegment) bool {
	for index, segment := range *list {
		if segment == wanted {
			*list = append((*list)[:index], (*list)[index+1:]...)
			return true
		}
	}
	return false
}

func removeSplineDependency(list []*splineDependency, wanted *splineDependency) []*splineDependency {
	for index, dependency := range list {
		if dependency == wanted {
			return append(list[:index], list[index+1:]...)
		}
	}
	return list
}

func updateSplineNeighbors(segment *splineSegment, sources, sinks *[]*splineSegment) {
	for _, dependency := range segment.outgoing {
		if dependency.target.mark < 0 && dependency.weight > 0 {
			dependency.target.inWeight -= dependency.weight
			if dependency.target.inWeight <= 0 && dependency.target.outWeight > 0 {
				*sources = append(*sources, dependency.target)
			}
		}
	}
	for _, dependency := range segment.incoming {
		if dependency.source.mark < 0 && dependency.weight > 0 {
			dependency.source.outWeight -= dependency.weight
			if dependency.source.outWeight <= 0 && dependency.source.inWeight > 0 {
				*sinks = append(*sinks, dependency.source)
			}
		}
	}
}

func numberSplineSegments(segments []*splineSegment) {
	sources, rightwardTargets := make([]*splineSegment, 0), make([]*splineSegment, 0)
	for _, segment := range segments {
		segment.rank = 0
		segment.inWeight, segment.outWeight = len(segment.incoming), len(segment.outgoing)
		if segment.inWeight == 0 {
			sources = append(sources, segment)
		}
		if segment.outWeight == 0 && len(segment.leftPorts) == 0 {
			rightwardTargets = append(rightwardTargets, segment)
		}
	}
	maximumRank := -1
	for len(sources) > 0 {
		segment := sources[0]
		sources = sources[1:]
		for _, dependency := range segment.outgoing {
			target := dependency.target
			target.rank = max(target.rank, segment.rank+1)
			maximumRank = max(maximumRank, target.rank)
			target.inWeight--
			if target.inWeight == 0 {
				sources = append(sources, target)
			}
		}
	}
	if maximumRank < 0 {
		return
	}
	for _, segment := range rightwardTargets {
		segment.rank = maximumRank
	}
	for len(rightwardTargets) > 0 {
		segment := rightwardTargets[0]
		rightwardTargets = rightwardTargets[1:]
		for _, dependency := range segment.incoming {
			source := dependency.source
			if len(source.leftPorts) > 0 {
				continue
			}
			source.rank = min(source.rank, segment.rank-1)
			source.outWeight--
			if source.outWeight == 0 {
				rightwardTargets = append(rightwardTargets, source)
			}
		}
	}
}

func splineSloppySpacing(
	arena *layered.LGraphArena,
	rightLayer layered.LayerID,
	edgeSpacing, nodeSpacing, factor float64,
) float64 {
	maximumDifference := 0.0
	for _, nodeID := range arena.Layer(rightLayer).Nodes {
		current := 0.0
		for _, edgeID := range arena.NodeIncomingEdges(nodeID) {
			edge := arena.Edge(edgeID)
			current = math.Max(current, math.Abs(absolutePortAnchor(arena, *edge.Target).Y-absolutePortAnchor(arena, *edge.Source).Y))
		}
		maximumDifference = math.Max(maximumDifference, current)
	}
	return factor * math.Min(1, edgeSpacing/nodeSpacing) * maximumDifference
}

func writeSplineControls(
	arena *layered.LGraphArena,
	segment *splineSegment,
	mode layered.SplineRoutingMode,
	edgeNodeSpacing, edgeSpacing float64,
) {
	edge := arena.Edge(segment.edge)
	if edge.Source == nil || edge.Target == nil {
		return
	}
	centerX := segment.boundingX + segment.boundingWidth/2
	controls := make(geometry.Chain, 0, 4)
	if segment.straight {
		controls = append(controls, geometry.Point{X: centerX, Y: segment.centerY})
	} else if mode == layered.SplineRoutingConservative ||
		arena.Port(*edge.Source).Side == layered.WestSide ||
		arena.Port(*edge.Target).Side == layered.EastSide {
		routingX := segment.boundingX
		if !segment.westOfInitialLayer {
			routingX += edgeNodeSpacing
		}
		routingX += segment.xDelta + float64(segment.rank)*edgeSpacing
		sourceX, targetX := segment.boundingX, segment.boundingX+segment.boundingWidth
		if arena.Port(*edge.Source).Side == layered.WestSide {
			sourceX = segment.boundingX + segment.boundingWidth
		}
		if arena.Port(*edge.Target).Side == layered.EastSide {
			targetX = segment.boundingX
		}
		controls = append(controls,
			geometry.Point{X: sourceX, Y: segment.sourceY},
			geometry.Point{X: routingX, Y: segment.sourceY},
			geometry.Point{X: routingX, Y: segment.targetY},
			geometry.Point{X: targetX, Y: segment.targetY})
	} else {
		if !segment.normalSource {
			controls = append(controls,
				geometry.Point{X: segment.boundingX, Y: segment.sourceY},
				geometry.Point{X: centerX, Y: segment.sourceY})
		}
		if !segment.normalTarget {
			controls = append(controls,
				geometry.Point{X: centerX, Y: segment.targetY},
				geometry.Point{X: segment.boundingX + segment.boundingWidth, Y: segment.targetY})
		}
		if segment.normalSource && segment.normalTarget {
			// The degree-dependent ordinate is evaluated by the final processor,
			// after SelfLoopPostprocessor has reattached hidden loop edges.
			controls = append(controls, geometry.Point{X: centerX, Y: math.NaN()})
		}
	}
	edge.BendPoints = edge.BendPoints[:0]
	basegraph.Set(&edge.Properties, layered.SplineControlPointSegments, []geometry.Chain{controls})
	basegraph.Set(&edge.Properties, layered.SplineCompactionSegments,
		[]layered.SplineCompactionSegment{{
			BoundingBox: geometry.Rectangle{
				X:      segment.boundingX,
				Y:      math.Min(segment.sourceY, segment.targetY),
				Width:  segment.boundingWidth,
				Height: math.Abs(segment.sourceY - segment.targetY),
			},
			Straight:         segment.straight,
			Edge:             segment.edge,
			SourcePort:       *edge.Source,
			TargetPort:       *edge.Target,
			SourceY:          segment.sourceY,
			TargetY:          segment.targetY,
			NormalSource:     segment.normalSource,
			NormalTarget:     segment.normalTarget,
			WestOfFirstLayer: segment.westOfInitialLayer,
			XDelta:           segment.xDelta,
			Rank:             segment.rank,
		}})
}

func splineNormalNode(nodeType layered.NodeType) bool {
	return nodeType == layered.NormalNode || nodeType == layered.BreakingPointNode
}

// FinalizeSplineBendpoints mirrors FinalSplineBendpointsCalculator after long
// edges have been joined. It assembles the per-between-layer NUB control
// polygons and raises every inner knot to cubic multiplicity, yielding the
// Bezier control-point chain emitted by elkjs.
func FinalizeSplineBendpoints(arena *layered.LGraphArena, graphID layered.LGraphID) {
	for edgeIndex := range arena.Edges {
		edgeID := layered.LEdgeID(edgeIndex)
		edge := arena.Edge(edgeID)
		if edge.Source == nil || edge.Target == nil || arena.EdgeIsSelfLoop(edgeID) {
			continue
		}
		if arena.NodeGraph(arena.EdgeSourceNode(edgeID)) != graphID {
			continue
		}
		segments, found := basegraph.TryGet(&edge.Properties, layered.SplineControlPointSegments)
		if !found || len(segments) == 0 {
			continue
		}
		if compactionSegments, found := basegraph.TryGet(
			&edge.Properties, layered.SplineCompactionSegments,
		); found && len(compactionSegments) == len(segments) {
			applyPostCompactionSplineRouting(
				arena, graphID, edgeID, segments, compactionSegments)
		}
		resolveSloppySplineCenters(arena, edgeID, segments)
		controlPoints := geometry.Chain{absolutePortAnchor(arena, *edge.Source)}
		var last geometry.Point
		addMidpoint := false
		for _, segment := range segments {
			if len(segment) == 0 {
				continue
			}
			if addMidpoint {
				controlPoints = append(controlPoints, geometry.Point{
					X: (last.X + segment[0].X) / 2,
					Y: (last.Y + segment[0].Y) / 2,
				})
				addMidpoint = false
			} else {
				addMidpoint = true
			}
			last = segment[len(segment)-1]
			controlPoints = append(controlPoints, segment...)
		}
		controlPoints = append(controlPoints, absolutePortAnchor(arena, *edge.Target))
		if basegraph.Get(&arena.Graph(graphID).Properties, layered.SplineRoutingModeOption) == layered.SplineRoutingConservative {
			controlPoints = insertSplineStraighteningControls(
				controlPoints, arena.Port(*edge.Source).Side, arena.Port(*edge.Target).Side)
		}
		edge.BendPoints = CubicNUBToBezier(controlPoints)
		basegraph.Unset(&edge.Properties, layered.SplineControlPointSegments)
		basegraph.Unset(&edge.Properties, layered.SplineCompactionSegments)
	}
}

// applyPostCompactionSplineRouting implements FinalSplineBendpointsCalculator's
// segmentAllowsSloppyRouting check. Horizontal compaction can pull an initial
// or final curved segment far enough from its incident node that ELK switches
// that segment from sloppy to conservative control points.
func applyPostCompactionSplineRouting(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	edgeID layered.LEdgeID,
	controls []geometry.Chain,
	segments []layered.SplineCompactionSegment,
) {
	graph := arena.Graph(graphID)
	if basegraph.Get(&graph.Properties, layered.PostCompactionStrategy) ==
		layered.GraphCompactionNone ||
		basegraph.Get(&graph.Properties, layered.SplineRoutingModeOption) !=
			layered.SplineRoutingSloppy {
		return
	}
	edgeNodeSpacing := basegraph.Get(
		&graph.Properties, layered.SpacingEdgeNodeBetweenLayers)
	edgeEdgeSpacing := basegraph.Get(
		&graph.Properties, layered.SpacingEdgeEdgeBetweenLayers)

	for index := range segments {
		segment := &segments[index]
		if segment.Straight || !(segment.NormalSource || segment.NormalTarget) {
			continue
		}
		// Inverted-port segments always use their dedicated four-control-point
		// route, independently of sloppy routing.
		if arena.Port(segment.SourcePort).Side == layered.WestSide ||
			arena.Port(segment.TargetPort).Side == layered.EastSide {
			continue
		}
		allowsSloppy := true
		if index == 0 {
			source := arena.Node(arena.EdgeSourceNode(edgeID))
			if source.Layer != nil {
				threshold := arena.Layer(*source.Layer).Size.X - source.Size.X/2
				distance := segment.BoundingBox.X -
					(source.Position.X + source.Size.X)
				if distance > threshold {
					allowsSloppy = false
				}
			}
		}
		if index == len(segments)-1 {
			target := arena.Node(arena.EdgeTargetNode(edgeID))
			if target.Layer != nil {
				threshold := arena.Layer(*target.Layer).Size.X - target.Size.X/2
				distance := target.Position.X - segment.BoundingBox.MaxX()
				if distance > threshold {
					allowsSloppy = false
				}
			}
		}
		if allowsSloppy {
			continue
		}

		startX := segment.BoundingBox.X
		endX := segment.BoundingBox.MaxX()
		routingX := startX
		if !segment.WestOfFirstLayer {
			routingX += edgeNodeSpacing
		}
		routingX += segment.XDelta + float64(segment.Rank)*edgeEdgeSpacing
		controls[index] = geometry.Chain{
			{X: startX, Y: segment.SourceY},
			{X: routingX, Y: segment.SourceY},
			{X: routingX, Y: segment.TargetY},
			{X: endX, Y: segment.TargetY},
		}
	}
}

func insertSplineStraighteningControls(
	points geometry.Chain,
	sourceSide, targetSide layered.PortSide,
) geometry.Chain {
	if len(points) < 2 {
		return points
	}
	first, second := points[0], points[1]
	sourceDirection := splineDirection(sourceSide)
	sourceDirection.X *= 5
	sourceDirection.Y *= 5
	sourceDelta := second.Sub(first)
	straightSource := first.Add(geometry.Point{
		X: splineAbsoluteMinimum(sourceDirection.X, sourceDelta.X),
		Y: splineAbsoluteMinimum(sourceDirection.Y, sourceDelta.Y),
	})
	points = append(points, geometry.Point{})
	copy(points[2:], points[1:])
	points[1] = straightSource

	lastIndex := len(points) - 1
	last, secondLast := points[lastIndex], points[lastIndex-1]
	targetDirection := splineDirection(targetSide)
	targetDirection.X *= 5
	targetDirection.Y *= 5
	targetDelta := secondLast.Sub(last)
	straightTarget := last.Add(geometry.Point{
		X: splineAbsoluteMinimum(targetDirection.X, targetDelta.X),
		Y: splineAbsoluteMinimum(targetDirection.Y, targetDelta.Y),
	})
	points = append(points, geometry.Point{})
	copy(points[lastIndex+1:], points[lastIndex:])
	points[lastIndex] = straightTarget
	return points
}

func splineDirection(side layered.PortSide) geometry.Point {
	switch side {
	case layered.NorthSide:
		return geometry.Point{Y: -1}
	case layered.EastSide:
		return geometry.Point{X: 1}
	case layered.SouthSide:
		return geometry.Point{Y: 1}
	case layered.WestSide:
		return geometry.Point{X: -1}
	default:
		return geometry.Point{}
	}
}

func splineAbsoluteMinimum(first, second float64) float64 {
	if math.Abs(first) < math.Abs(second) {
		return first
	}
	return second
}

func resolveSloppySplineCenters(
	arena *layered.LGraphArena,
	edgeID layered.LEdgeID,
	segments []geometry.Chain,
) {
	needsResolution := false
	for _, segment := range segments {
		for _, point := range segment {
			needsResolution = needsResolution || math.IsNaN(point.Y)
		}
	}
	if !needsResolution {
		return
	}
	edge := arena.Edge(edgeID)
	source, target := absolutePortAnchor(arena, *edge.Source), absolutePortAnchor(arena, *edge.Target)
	indegree, outdegree := 0, 0
	for _, portID := range arena.Node(arena.EdgeTargetNode(edgeID)).Ports {
		indegree += len(arena.Port(portID).IncomingEdges)
	}
	for _, portID := range arena.Node(arena.EdgeSourceNode(edgeID)).Ports {
		outdegree += len(arena.Port(portID).OutgoingEdges)
	}
	degreeDifference := 0
	if outdegree > indegree {
		degreeDifference = 1
	} else if outdegree < indegree {
		degreeDifference = -1
	}
	centerY := (target.Y+source.Y)/2 + (target.Y-source.Y)*(0.4*float64(degreeDifference))
	for segmentIndex := range segments {
		for pointIndex := range segments[segmentIndex] {
			if math.IsNaN(segments[segmentIndex][pointIndex].Y) {
				segments[segmentIndex][pointIndex].Y = centerY
			}
		}
	}
}

// CubicNUBToBezier converts a clamped uniform cubic B-spline control polygon
// to ELK's concatenated Bezier control-point representation.
func CubicNUBToBezier(points geometry.Chain) geometry.Chain {
	const degree = 3
	points = points.Clone()
	for len(points) < degree+1 {
		points = append(geometry.Chain{points[0]}, points...)
	}
	denominator := len(points) - degree
	knots := make([]float64, 0, len(points)+degree+1)
	for index := 0; index <= degree; index++ {
		knots = append(knots, 0)
	}
	for index := 1; index < denominator; index++ {
		knots = append(knots, float64(index)/float64(denominator))
	}
	for index := 0; index <= degree; index++ {
		knots = append(knots, 1)
	}
	inner := append([]float64(nil), knots[degree+1:len(knots)-degree-1]...)
	for index := 0; index < len(inner); {
		knot := inner[index]
		multiplicity := 1
		for index+multiplicity < len(inner) && math.Abs(inner[index+multiplicity]-knot) < 1e-6 {
			multiplicity++
		}
		for insertion := multiplicity; insertion < degree; insertion++ {
			points, knots = insertSplineKnot(points, knots, degree, knot)
		}
		index += multiplicity
	}
	return append(geometry.Chain(nil), points[1:len(points)-1]...)
}

func insertSplineKnot(
	points geometry.Chain,
	knots []float64,
	degree int,
	knot float64,
) (geometry.Chain, []float64) {
	n := len(points) - 1
	m := len(knots) - 1
	span := degree
	for span+1 < len(knots) && knot >= knots[span+1]-1e-12 {
		span++
	}
	if span > n {
		span = n
	}
	multiplicity := 0
	for _, candidate := range knots {
		if math.Abs(candidate-knot) < 1e-9 {
			multiplicity++
		}
	}
	newKnots := make([]float64, m+2)
	copy(newKnots[:span+1], knots[:span+1])
	newKnots[span+1] = knot
	copy(newKnots[span+2:], knots[span+1:])
	result := make(geometry.Chain, n+2)
	copy(result[:span-degree+1], points[:span-degree+1])
	copy(result[span-multiplicity+1:], points[span-multiplicity:])
	for index := span - degree + 1; index <= span-multiplicity; index++ {
		alpha := (knot - knots[index]) / (knots[index+degree] - knots[index])
		result[index] = geometry.Point{
			X: (1-alpha)*points[index-1].X + alpha*points[index].X,
			Y: (1-alpha)*points[index-1].Y + alpha*points[index].Y,
		}
	}
	return result, newKnots
}
