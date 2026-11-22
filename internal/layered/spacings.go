// Copyright (c) 2015 Kiel University and others.
// Copyright (c) 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package layered

import (
	"sync"

	basegraph "github.com/d2lang/elk-go/internal/graph"
)

type spacingTables struct {
	horizontal [6][6]*basegraph.Property[float64]
	vertical   [6][6]*basegraph.Property[float64]
}

var (
	spacingsOnce sync.Once
	spacingsData spacingTables
)

func layeredSpacingTables() *spacingTables {
	spacingsOnce.Do(func() {
		both := func(first, second NodeType, vertical, horizontal *basegraph.Property[float64]) {
			spacingsData.vertical[first][second] = vertical
			spacingsData.vertical[second][first] = vertical
			spacingsData.horizontal[first][second] = horizontal
			spacingsData.horizontal[second][first] = horizontal
		}
		vertical := func(first, second NodeType, property *basegraph.Property[float64]) {
			spacingsData.vertical[first][second] = property
			spacingsData.vertical[second][first] = property
		}

		both(NormalNode, NormalNode, &SpacingNodeNode, &SpacingNodeNodeBetweenLayers)
		both(NormalNode, LongEdgeNode, &SpacingEdgeNode, &SpacingEdgeNodeBetweenLayers)
		vertical(NormalNode, NorthSouthPortNode, &SpacingEdgeNode)
		vertical(NormalNode, ExternalPortNode, &SpacingEdgeNode)
		both(NormalNode, LabelNode, &SpacingNodeNode, &SpacingNodeNodeBetweenLayers)

		both(LongEdgeNode, LongEdgeNode, &SpacingEdgeEdge, &SpacingEdgeEdgeBetweenLayers)
		vertical(LongEdgeNode, NorthSouthPortNode, &SpacingEdgeEdge)
		vertical(LongEdgeNode, ExternalPortNode, &SpacingEdgeEdge)
		both(LongEdgeNode, LabelNode, &SpacingEdgeNode, &SpacingEdgeNodeBetweenLayers)

		vertical(NorthSouthPortNode, NorthSouthPortNode, &SpacingEdgeEdge)
		vertical(NorthSouthPortNode, ExternalPortNode, &SpacingEdgeEdge)
		vertical(NorthSouthPortNode, LabelNode, &SpacingLabelNode)

		vertical(ExternalPortNode, ExternalPortNode, &SpacingPortPort)
		both(ExternalPortNode, LabelNode, &SpacingLabelPortVertical, &SpacingLabelPortHorizontal)

		both(LabelNode, LabelNode, &SpacingEdgeEdge, &SpacingEdgeEdge)

		both(BreakingPointNode, BreakingPointNode, &SpacingEdgeEdge, &SpacingEdgeEdgeBetweenLayers)
		both(BreakingPointNode, NormalNode, &SpacingEdgeNode, &SpacingEdgeNodeBetweenLayers)
		both(BreakingPointNode, LabelNode, &SpacingEdgeNode, &SpacingEdgeNodeBetweenLayers)
		both(BreakingPointNode, LongEdgeNode, &SpacingEdgeNode, &SpacingEdgeNodeBetweenLayers)
	})
	return &spacingsData
}

func individualOrDefault(
	arena *LGraphArena,
	nodeID LNodeID,
	property *basegraph.Property[float64],
) float64 {
	node := arena.Node(nodeID)
	if individual, found := basegraph.TryGet(&node.Properties, SpacingIndividual); found {
		if value, explicit := basegraph.TryGet(&individual.Properties, *property); explicit {
			return value
		}
	}
	return basegraph.Get(&arena.Graph(arena.NodeGraph(nodeID)).Properties, *property)
}

// IndividualOrDefault exposes LayeredSpacings.getIndividualOrDefault to
// source-shaped phase implementations.
func IndividualOrDefault(
	arena *LGraphArena,
	nodeID LNodeID,
	property *basegraph.Property[float64],
) float64 {
	return individualOrDefault(arena, nodeID, property)
}

func nodeSpacing(
	arena *LGraphArena,
	first, second LNodeID,
	table *[6][6]*basegraph.Property[float64],
) float64 {
	firstType, secondType := arena.Node(first).Type, arena.Node(second).Type
	property := table[firstType][secondType]
	if property == nil {
		panic("layered: unspecified spacing between node types")
	}
	firstValue := individualOrDefault(arena, first, property)
	secondValue := individualOrDefault(arena, second, property)
	if firstValue > secondValue {
		return firstValue
	}
	return secondValue
}

// SpacingCache answers VerticalSpacing for nodes while neither the spacing
// options of their graphs nor any node's individual spacings change: it reads
// each graph-level spacing property of one graph once (those reads have no
// effect on the graph: spacing properties are plain values with constant
// defaults) and answers the rest exactly as VerticalSpacing does.
type SpacingCache struct {
	arena      *LGraphArena
	graph      LGraphID
	properties [8]*basegraph.Property[float64]
	values     [8]float64
	count      int
}

// BoundTo reports whether the cache answers for graphID of arena.
func (cache *SpacingCache) BoundTo(arena *LGraphArena, graphID LGraphID) bool {
	return cache.arena == arena && cache.graph == graphID
}

// Reset empties the cache and binds it to graphID.
func (cache *SpacingCache) Reset(arena *LGraphArena, graphID LGraphID) {
	*cache = SpacingCache{arena: arena, graph: graphID}
}

// Vertical is VerticalSpacing(arena, first, second).
func (cache *SpacingCache) Vertical(first, second LNodeID) float64 {
	arena := cache.arena
	property := layeredSpacingTables().vertical[arena.Node(first).Type][arena.Node(second).Type]
	if property == nil {
		panic("layered: unspecified spacing between node types")
	}
	firstValue := cache.individualOrDefault(first, property)
	secondValue := cache.individualOrDefault(second, property)
	if firstValue > secondValue {
		return firstValue
	}
	return secondValue
}

func (cache *SpacingCache) individualOrDefault(nodeID LNodeID, property *basegraph.Property[float64]) float64 {
	arena := cache.arena
	if individual, found := basegraph.TryGet(&arena.Node(nodeID).Properties, SpacingIndividual); found {
		if value, explicit := basegraph.TryGet(&individual.Properties, *property); explicit {
			return value
		}
	}
	graphID := arena.NodeGraph(nodeID)
	if graphID != cache.graph {
		return basegraph.Get(&arena.Graph(graphID).Properties, *property)
	}
	for index := 0; index < cache.count; index++ {
		if cache.properties[index] == property {
			return cache.values[index]
		}
	}
	value := basegraph.Get(&arena.Graph(graphID).Properties, *property)
	if cache.count < len(cache.properties) {
		cache.properties[cache.count], cache.values[cache.count] = property, value
		cache.count++
	}
	return value
}

func HorizontalSpacing(arena *LGraphArena, first, second LNodeID) float64 {
	return nodeSpacing(arena, first, second, &layeredSpacingTables().horizontal)
}

func VerticalSpacing(arena *LGraphArena, first, second LNodeID) float64 {
	return nodeSpacing(arena, first, second, &layeredSpacingTables().vertical)
}

func HorizontalSpacingByType(arena *LGraphArena, graphID LGraphID, first, second NodeType) float64 {
	property := layeredSpacingTables().horizontal[first][second]
	if property == nil {
		panic("layered: unspecified horizontal spacing between node types")
	}
	return basegraph.Get(&arena.Graph(graphID).Properties, *property)
}

func VerticalSpacingByType(arena *LGraphArena, graphID LGraphID, first, second NodeType) float64 {
	property := layeredSpacingTables().vertical[first][second]
	if property == nil {
		panic("layered: unspecified vertical spacing between node types")
	}
	return basegraph.Get(&arena.Graph(graphID).Properties, *property)
}

// ApplySpacingBaseValue reproduces LayeredSpacings.withBaseValue. Explicit
// values remain authoritative; all absent dependent spacings are scaled by
// their ELK 0.8.1 default-to-nodeNode-default ratio.
func ApplySpacingBaseValue(properties *basegraph.PropertyMap, base float64) {
	baseDefault := basegraph.Get(&basegraph.PropertyMap{}, SpacingNodeNode)
	basegraph.Set(properties, SpacingNodeNode, base)
	dependents := []*basegraph.Property[float64]{
		&SpacingComponentComponent,
		&SpacingEdgeEdge,
		&SpacingEdgeLabel,
		&SpacingEdgeNode,
		&SpacingLabelLabel,
		&SpacingLabelNode,
		&SpacingLabelPortHorizontal,
		&SpacingLabelPortVertical,
		&SpacingNodeSelfLoop,
		&SpacingPortPort,
		&SpacingEdgeEdgeBetweenLayers,
		&SpacingEdgeNodeBetweenLayers,
		&SpacingNodeNodeBetweenLayers,
	}
	for _, property := range dependents {
		if basegraph.Has(properties, *property) {
			continue
		}
		defaultValue := basegraph.Get(&basegraph.PropertyMap{}, *property)
		basegraph.Set(properties, *property, base*defaultValue/baseDefault)
	}
}
