// Copyright (c) 2016 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"sort"

	"github.com/d2lang/elk-go/internal/layered"
)

type hyperedge struct {
	id                                           int
	edges                                        []layered.LEdgeID
	ports                                        []layered.LPortID
	upperLeft, lowerLeft, upperRight, lowerRight int
}

type cornerType uint8

const (
	upperCorner cornerType = iota
	lowerCorner
)

type hyperedgeCorner struct {
	hyperedge        int
	position         int
	oppositePosition int
	typeOf           cornerType
}

// countHyperedgeCrossings implements the ELK 0.8.1 DIAGRAMS 2014 estimate.
// Java's final identity-hash tiebreak is represented by hyperedge creation
// order, which is also the stable identity order produced by GWT's runtime.
func countHyperedgeCrossings(
	arena *layered.LGraphArena,
	portPositions []int,
	leftLayer, rightLayer []layered.LNodeID,
) int {
	if len(leftLayer) == 0 || len(rightLayer) == 0 {
		return 0
	}
	sourceCount := 0
	for _, nodeID := range leftLayer {
		for _, portID := range arena.Node(nodeID).Ports {
			portEdges := 0
			for _, edgeID := range arena.Port(portID).OutgoingEdges {
				if !sameLayer(arena.Node(nodeID).Layer, arena.Node(arena.EdgeTargetNode(edgeID)).Layer) {
					portEdges++
				}
			}
			if portEdges > 0 {
				portPositions[arena.Port(portID).ID] = sourceCount
				sourceCount++
			}
		}
	}

	targetCount := 0
	for _, nodeID := range rightLayer {
		northInputPorts := 0
		for _, portID := range arena.Node(nodeID).Ports {
			if arena.Port(portID).Side != layered.NorthSide {
				break
			}
			for _, edgeID := range arena.Port(portID).IncomingEdges {
				if !sameLayer(arena.Node(nodeID).Layer, arena.Node(arena.EdgeSourceNode(edgeID)).Layer) {
					northInputPorts++
					break
				}
			}
		}
		otherInputPorts := 0
		ports := arena.Node(nodeID).Ports
		for index := len(ports) - 1; index >= 0; index-- {
			portID := ports[index]
			portEdges := 0
			for _, edgeID := range arena.Port(portID).IncomingEdges {
				if !sameLayer(arena.Node(nodeID).Layer, arena.Node(arena.EdgeSourceNode(edgeID)).Layer) {
					portEdges++
				}
			}
			if portEdges == 0 {
				continue
			}
			if arena.Port(portID).Side == layered.NorthSide {
				portPositions[arena.Port(portID).ID] = targetCount
				targetCount++
			} else {
				portPositions[arena.Port(portID).ID] = targetCount + northInputPorts + otherInputPorts
				otherInputPorts++
			}
		}
		targetCount += otherInputPorts
	}

	hyperedges := make([]hyperedge, 0)
	portToHyperedge := make(map[layered.LPortID]int)
	live := make([]int, 0)
	for _, nodeID := range leftLayer {
		for _, sourcePort := range arena.Node(nodeID).Ports {
			for _, edgeID := range arena.Port(sourcePort).OutgoingEdges {
				targetPort := *arena.Edge(edgeID).Target
				targetNode := *arena.Port(targetPort).Node
				if sameLayer(arena.Node(nodeID).Layer, arena.Node(targetNode).Layer) {
					continue
				}
				sourceHE, hasSource := portToHyperedge[sourcePort]
				targetHE, hasTarget := portToHyperedge[targetPort]
				switch {
				case !hasSource && !hasTarget:
					id := len(hyperedges)
					hyperedges = append(hyperedges, hyperedge{
						id: id, edges: []layered.LEdgeID{edgeID},
						ports: []layered.LPortID{sourcePort, targetPort},
					})
					live = append(live, id)
					portToHyperedge[sourcePort], portToHyperedge[targetPort] = id, id
				case !hasSource:
					hyperedges[targetHE].edges = append(hyperedges[targetHE].edges, edgeID)
					hyperedges[targetHE].ports = append(hyperedges[targetHE].ports, sourcePort)
					portToHyperedge[sourcePort] = targetHE
				case !hasTarget:
					hyperedges[sourceHE].edges = append(hyperedges[sourceHE].edges, edgeID)
					hyperedges[sourceHE].ports = append(hyperedges[sourceHE].ports, targetPort)
					portToHyperedge[targetPort] = sourceHE
				case sourceHE == targetHE:
					hyperedges[sourceHE].edges = append(hyperedges[sourceHE].edges, edgeID)
				default:
					hyperedges[sourceHE].edges = append(hyperedges[sourceHE].edges, edgeID)
					for _, portID := range hyperedges[targetHE].ports {
						portToHyperedge[portID] = sourceHE
					}
					hyperedges[sourceHE].edges = append(hyperedges[sourceHE].edges, hyperedges[targetHE].edges...)
					hyperedges[sourceHE].ports = append(hyperedges[sourceHE].ports, hyperedges[targetHE].ports...)
					live = removeHyperedge(live, targetHE)
				}
			}
		}
	}

	leftLayerID, rightLayerID := arena.Node(leftLayer[0]).Layer, arena.Node(rightLayer[0]).Layer
	for _, hyperedgeID := range live {
		hyperedges[hyperedgeID].upperLeft = sourceCount
		hyperedges[hyperedgeID].upperRight = targetCount
		for _, portID := range hyperedges[hyperedgeID].ports {
			position := portPositions[arena.Port(portID).ID]
			portLayer := arena.Node(*arena.Port(portID).Node).Layer
			if sameLayer(portLayer, leftLayerID) {
				if position < hyperedges[hyperedgeID].upperLeft {
					hyperedges[hyperedgeID].upperLeft = position
				}
				if position > hyperedges[hyperedgeID].lowerLeft {
					hyperedges[hyperedgeID].lowerLeft = position
				}
			} else if sameLayer(portLayer, rightLayerID) {
				if position < hyperedges[hyperedgeID].upperRight {
					hyperedges[hyperedgeID].upperRight = position
				}
				if position > hyperedges[hyperedgeID].lowerRight {
					hyperedges[hyperedgeID].lowerRight = position
				}
			}
		}
	}

	sorted := append([]int(nil), live...)
	sort.SliceStable(sorted, func(i, j int) bool {
		left, right := hyperedges[sorted[i]], hyperedges[sorted[j]]
		if left.upperLeft != right.upperLeft {
			return left.upperLeft < right.upperLeft
		}
		if left.upperRight != right.upperRight {
			return left.upperRight < right.upperRight
		}
		return left.id < right.id
	})

	southSequence := make([]int, len(sorted))
	compressDeltas := make([]int, targetCount+1)
	for index, hyperedgeID := range sorted {
		southSequence[index] = hyperedges[hyperedgeID].upperRight
		compressDeltas[southSequence[index]] = 1
	}
	delta := 0
	for index := range compressDeltas {
		if compressDeltas[index] == 1 {
			compressDeltas[index] = delta
		} else {
			delta--
		}
	}
	q := 0
	for index := range southSequence {
		position := southSequence[index]
		southSequence[index] += compressDeltas[position]
		q = maxInt(q, southSequence[index]+1)
	}

	firstIndex := 1
	for firstIndex < q {
		firstIndex *= 2
	}
	tree := make([]int, 2*firstIndex-1)
	firstIndex--
	crossings := 0
	for _, position := range southSequence {
		index := position + firstIndex
		tree[index]++
		for index > 0 {
			if index%2 > 0 {
				crossings += tree[index+1]
			}
			index = (index - 1) / 2
			tree[index]++
		}
	}

	leftCorners := make([]hyperedgeCorner, 0, len(sorted)*2)
	rightCorners := make([]hyperedgeCorner, 0, len(sorted)*2)
	for _, hyperedgeID := range sorted {
		he := hyperedges[hyperedgeID]
		leftCorners = append(leftCorners,
			hyperedgeCorner{he.id, he.upperLeft, he.lowerLeft, upperCorner},
			hyperedgeCorner{he.id, he.lowerLeft, he.upperLeft, lowerCorner},
		)
		rightCorners = append(rightCorners,
			hyperedgeCorner{he.id, he.upperRight, he.lowerRight, upperCorner},
			hyperedgeCorner{he.id, he.lowerRight, he.upperRight, lowerCorner},
		)
	}
	crossings += countCornerOverlaps(leftCorners)
	crossings += countCornerOverlaps(rightCorners)
	return crossings
}

func countCornerOverlaps(corners []hyperedgeCorner) int {
	sort.SliceStable(corners, func(i, j int) bool {
		left, right := corners[i], corners[j]
		if left.position != right.position {
			return left.position < right.position
		}
		if left.oppositePosition != right.oppositePosition {
			return left.oppositePosition < right.oppositePosition
		}
		if left.hyperedge != right.hyperedge {
			return left.hyperedge < right.hyperedge
		}
		return left.typeOf < right.typeOf
	})
	open, crossings := 0, 0
	for _, corner := range corners {
		if corner.typeOf == upperCorner {
			open++
		} else {
			open--
			crossings += open
		}
	}
	return crossings
}

func removeHyperedge(ids []int, remove int) []int {
	for index, id := range ids {
		if id == remove {
			return append(ids[:index], ids[index+1:]...)
		}
	}
	return ids
}

func sameLayer(first, second *layered.LayerID) bool {
	if first == nil || second == nil {
		return first == nil && second == nil
	}
	return *first == *second
}

func maxInt(first, second int) int {
	if first > second {
		return first
	}
	return second
}
