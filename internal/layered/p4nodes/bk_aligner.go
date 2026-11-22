// Copyright (c) 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import (
	"math"

	"github.com/d2lang/elk-go/internal/layered"
)

type BKAligner struct {
	Arena *layered.LGraphArena
	Graph layered.LGraphID
	Info  *NeighborhoodInformation

	// Buffers for blockRootsInto, reused by every InsideBlockShift call (each
	// call consumes its root list before the next one).
	roots []layered.LNodeID
	seen  []bool
}

func NewBKAligner(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	info *NeighborhoodInformation,
) *BKAligner {
	return &BKAligner{Arena: arena, Graph: graphID, Info: info}
}

// VerticalAlignment groups nodes into blocks while avoiding type-1 conflict
// edges. The two median neighbors are visited in the ELK-prescribed order.
func (aligner *BKAligner) VerticalAlignment(
	layout *BKAlignedLayout,
	markedEdges map[layered.LEdgeID]bool,
) {
	var marks edgeMarks
	for edgeID, marked := range markedEdges {
		if marked {
			marks.mark(edgeID)
		}
	}
	aligner.verticalAlignment(layout, &marks)
}

func (aligner *BKAligner) verticalAlignment(
	layout *BKAlignedLayout,
	markedEdges *edgeMarks,
) {
	for _, layerID := range aligner.Arena.Graph(aligner.Graph).Layers {
		for _, nodeID := range aligner.Arena.Layer(layerID).Nodes {
			index := aligner.Arena.Node(nodeID).ID
			layout.Root[index] = someNode(nodeID)
			layout.Align[index] = someNode(nodeID)
			layout.InnerShift[index] = someFloat(0)
		}
	}
	// The layers and their nodes are walked in reverse where ELK reverses
	// copies; alignment changes neither the layer list nor the node order.
	layers := aligner.Arena.Graph(aligner.Graph).Layers
	for layerStep := range layers {
		layerID := layers[layerStep]
		if layout.HDirection == HorizontalLeft {
			layerID = layers[len(layers)-1-layerStep]
		}
		r := -1
		if layout.VDirection == VerticalUp {
			r = math.MaxInt
		}
		nodes := aligner.Arena.Layer(layerID).Nodes
		for nodeStep := range nodes {
			nodeID := nodes[nodeStep]
			if layout.VDirection == VerticalUp {
				nodeID = nodes[len(nodes)-1-nodeStep]
			}
			neighbors := aligner.Info.LeftNeighbors[aligner.Arena.Node(nodeID).ID]
			if layout.HDirection == HorizontalLeft {
				neighbors = aligner.Info.RightNeighbors[aligner.Arena.Node(nodeID).ID]
			}
			if len(neighbors) == 0 {
				continue
			}
			d := float64(len(neighbors))
			low := int(math.Floor((d+1)/2)) - 1
			high := int(math.Ceil((d+1)/2)) - 1
			if layout.VDirection == VerticalUp {
				for median := high; median >= low; median-- {
					r = aligner.tryAlign(layout, markedEdges, nodeID, neighbors[median], r, true)
				}
			} else {
				for median := low; median <= high; median++ {
					r = aligner.tryAlign(layout, markedEdges, nodeID, neighbors[median], r, false)
				}
			}
		}
	}
}

func (aligner *BKAligner) tryAlign(
	layout *BKAlignedLayout,
	markedEdges *edgeMarks,
	nodeID layered.LNodeID,
	neighbor Neighbor,
	r int,
	up bool,
) int {
	index := aligner.Arena.Node(nodeID).ID
	if requireNode(layout.Align[index], "align") != nodeID {
		return r
	}
	neighborPosition := aligner.Info.NodeIndex[aligner.Arena.Node(neighbor.Node).ID]
	positionAllowed := r < neighborPosition
	if up {
		positionAllowed = r > neighborPosition
	}
	if markedEdges.has(neighbor.Edge) || !positionAllowed {
		return r
	}
	neighborIndex := aligner.Arena.Node(neighbor.Node).ID
	layout.Align[neighborIndex] = someNode(nodeID)
	rootID := requireNode(layout.Root[neighborIndex], "root")
	layout.Root[index] = someNode(rootID)
	layout.Align[index] = someNode(rootID)
	rootIndex := aligner.Arena.Node(rootID).ID
	layout.OD[rootIndex] = layout.OD[rootIndex] && aligner.Arena.Node(nodeID).Type == layered.LongEdgeNode
	return neighborPosition
}

// InsideBlockShift accounts for node sizes, margins, and port positions after
// the original BK block construction.
func (aligner *BKAligner) InsideBlockShift(layout *BKAlignedLayout) {
	aligner.roots, aligner.seen = blockRootsInto(aligner.Arena, aligner.Graph, layout, aligner.roots, aligner.seen)
	for _, rootID := range aligner.roots {
		spaceAbove := aligner.Arena.Node(rootID).Margin.Top
		spaceBelow := aligner.Arena.Node(rootID).Size.Y + aligner.Arena.Node(rootID).Margin.Bottom
		layout.InnerShift[aligner.Arena.Node(rootID).ID] = someFloat(0)
		current := rootID
		for {
			next := requireNode(layout.Align[aligner.Arena.Node(current).ID], "align")
			if next == rootID {
				break
			}
			edgeID, found := edgeBetween(aligner.Arena, current, next)
			if !found {
				panic("p4nodes: aligned nodes are not connected")
			}
			edge := aligner.Arena.Edge(edgeID)
			source, target := aligner.Arena.Port(*edge.Source), aligner.Arena.Port(*edge.Target)
			portPositionDifference := source.Position.Y + source.Anchor.Y - target.Position.Y - target.Anchor.Y
			if layout.HDirection == HorizontalLeft {
				portPositionDifference = target.Position.Y + target.Anchor.Y - source.Position.Y - source.Anchor.Y
			}
			nextInnerShift := requireFloat(layout.InnerShift[aligner.Arena.Node(current).ID], "inner shift") + portPositionDifference
			layout.InnerShift[aligner.Arena.Node(next).ID] = someFloat(nextInnerShift)
			spaceAbove = math.Max(spaceAbove, aligner.Arena.Node(next).Margin.Top-nextInnerShift)
			spaceBelow = math.Max(spaceBelow, nextInnerShift+aligner.Arena.Node(next).Size.Y+aligner.Arena.Node(next).Margin.Bottom)
			current = next
		}
		current = rootID
		for {
			index := aligner.Arena.Node(current).ID
			layout.InnerShift[index] = someFloat(requireFloat(layout.InnerShift[index], "inner shift") + spaceAbove)
			current = requireNode(layout.Align[index], "align")
			if current == rootID {
				break
			}
		}
		layout.BlockSize[aligner.Arena.Node(rootID).ID] = someFloat(spaceAbove + spaceBelow)
	}
}

// blockRootsInto returns the roots of the layout's blocks, each once, in the
// order in which a node of each block first comes, layer by layer, filling
// the given buffers (returned grown).
func blockRootsInto(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	layout *BKAlignedLayout,
	roots []layered.LNodeID,
	seen []bool,
) ([]layered.LNodeID, []bool) {
	// At most one root per node of the graph.
	if cap(roots) < len(layout.Root) {
		roots = make([]layered.LNodeID, 0, len(layout.Root))
	}
	roots = roots[:0]
	// Roots are nodes of this graph, so their scratch IDs index the layout.
	if cap(seen) < len(layout.Root) {
		seen = make([]bool, len(layout.Root))
	} else {
		seen = seen[:len(layout.Root)]
		clear(seen)
	}
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			rootID := requireNode(layout.Root[arena.Node(nodeID).ID], "root")
			if rootIndex := arena.Node(rootID).ID; !seen[rootIndex] {
				seen[rootIndex] = true
				roots = append(roots, rootID)
			}
		}
	}
	return roots, seen
}

func edgeBetween(
	arena *layered.LGraphArena,
	first, second layered.LNodeID,
) (layered.LEdgeID, bool) {
	// NodeConnectedEdges order: per port, incoming then outgoing.
	for _, portID := range arena.Node(first).Ports {
		port := arena.Port(portID)
		for _, edges := range [2][]layered.LEdgeID{port.IncomingEdges, port.OutgoingEdges} {
			for _, edgeID := range edges {
				if arena.EdgeOtherNode(edgeID, first) == second {
					return edgeID, true
				}
			}
		}
	}
	return 0, false
}
