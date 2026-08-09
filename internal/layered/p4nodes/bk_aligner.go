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
	for _, layerID := range aligner.Arena.Graph(aligner.Graph).Layers {
		for _, nodeID := range aligner.Arena.Layer(layerID).Nodes {
			index := aligner.Arena.Node(nodeID).ID
			layout.Root[index] = nodePointer(nodeID)
			layout.Align[index] = nodePointer(nodeID)
			layout.InnerShift[index] = floatPointer(0)
		}
	}
	layers := append([]layered.LayerID(nil), aligner.Arena.Graph(aligner.Graph).Layers...)
	if layout.HDirection == HorizontalLeft {
		reverseLayers(layers)
	}
	for _, layerID := range layers {
		r := -1
		nodes := append([]layered.LNodeID(nil), aligner.Arena.Layer(layerID).Nodes...)
		if layout.VDirection == VerticalUp {
			r = math.MaxInt
			reverseNodes(nodes)
		}
		for _, nodeID := range nodes {
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
	markedEdges map[layered.LEdgeID]bool,
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
	if markedEdges[neighbor.Edge] || !positionAllowed {
		return r
	}
	neighborIndex := aligner.Arena.Node(neighbor.Node).ID
	layout.Align[neighborIndex] = nodePointer(nodeID)
	rootID := requireNode(layout.Root[neighborIndex], "root")
	layout.Root[index] = nodePointer(rootID)
	layout.Align[index] = nodePointer(rootID)
	rootIndex := aligner.Arena.Node(rootID).ID
	layout.OD[rootIndex] = layout.OD[rootIndex] && aligner.Arena.Node(nodeID).Type == layered.LongEdgeNode
	return neighborPosition
}

// InsideBlockShift accounts for node sizes, margins, and port positions after
// the original BK block construction.
func (aligner *BKAligner) InsideBlockShift(layout *BKAlignedLayout) {
	for _, rootID := range blockRoots(aligner.Arena, aligner.Graph, layout) {
		spaceAbove := aligner.Arena.Node(rootID).Margin.Top
		spaceBelow := aligner.Arena.Node(rootID).Size.Y + aligner.Arena.Node(rootID).Margin.Bottom
		layout.InnerShift[aligner.Arena.Node(rootID).ID] = floatPointer(0)
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
			layout.InnerShift[aligner.Arena.Node(next).ID] = floatPointer(nextInnerShift)
			spaceAbove = math.Max(spaceAbove, aligner.Arena.Node(next).Margin.Top-nextInnerShift)
			spaceBelow = math.Max(spaceBelow, nextInnerShift+aligner.Arena.Node(next).Size.Y+aligner.Arena.Node(next).Margin.Bottom)
			current = next
		}
		current = rootID
		for {
			index := aligner.Arena.Node(current).ID
			layout.InnerShift[index] = floatPointer(requireFloat(layout.InnerShift[index], "inner shift") + spaceAbove)
			current = requireNode(layout.Align[index], "align")
			if current == rootID {
				break
			}
		}
		layout.BlockSize[aligner.Arena.Node(rootID).ID] = floatPointer(spaceAbove + spaceBelow)
	}
}

func blockRoots(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	layout *BKAlignedLayout,
) []layered.LNodeID {
	roots := make([]layered.LNodeID, 0)
	seen := make(map[layered.LNodeID]bool)
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			rootID := requireNode(layout.Root[arena.Node(nodeID).ID], "root")
			if !seen[rootID] {
				seen[rootID] = true
				roots = append(roots, rootID)
			}
		}
	}
	return roots
}

func edgeBetween(
	arena *layered.LGraphArena,
	first, second layered.LNodeID,
) (layered.LEdgeID, bool) {
	for _, edgeID := range arena.NodeConnectedEdges(first) {
		if arena.EdgeOtherNode(edgeID, first) == second {
			return edgeID, true
		}
	}
	return 0, false
}

func reverseLayers(values []layered.LayerID) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

func reverseNodes(values []layered.LNodeID) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}
