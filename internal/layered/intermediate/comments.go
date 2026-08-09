// Copyright (c) 2010, 2020 Kiel University and others.
// Copyright (c) 2011, 2015 Kiel University and others.
// Copyright (c) 2011, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"fmt"
	"math"

	"github.com/d2lang/elk-go/internal/core"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// PreprocessComments removes singly connected comment boxes from the normal
// layered pipeline and attaches them to the connected node for later placement.
func PreprocessComments(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	nodes := append([]layered.LNodeID(nil), arena.Graph(graphID).LayerlessNodes...)
	for _, nodeID := range nodes {
		if !basegraph.Get(&arena.Node(nodeID).Properties, core.CommentBox) {
			continue
		}
		edgeCount := 0
		var edgeID layered.LEdgeID
		var oppositePort layered.LPortID
		for _, portID := range append([]layered.LPortID(nil), arena.Node(nodeID).Ports...) {
			port := arena.Port(portID)
			edgeCount += len(port.IncomingEdges) + len(port.OutgoingEdges)
			if len(port.IncomingEdges) == 1 {
				edgeID = port.IncomingEdges[0]
				if arena.Edge(edgeID).Source != nil {
					oppositePort = *arena.Edge(edgeID).Source
				}
			}
			if len(port.OutgoingEdges) == 1 {
				edgeID = port.OutgoingEdges[0]
				if arena.Edge(edgeID).Target != nil {
					oppositePort = *arena.Edge(edgeID).Target
				}
			}
		}
		if edgeCount == 1 && portDegree(arena, oppositePort) == 1 &&
			arena.Port(oppositePort).Node != nil &&
			!basegraph.Get(&arena.Node(*arena.Port(oppositePort).Node).Properties, core.CommentBox) {
			realNode := *arena.Port(oppositePort).Node
			processCommentBox(arena, graphID, nodeID, edgeID, oppositePort, realNode)
			continue
		}

		// Comments that cannot be extracted stay regular nodes. Reverse oddly
		// connected incident edges in Java list order.
		var reversed []layered.LEdgeID
		for _, portID := range append([]layered.LPortID(nil), arena.Node(nodeID).Ports...) {
			for _, candidate := range append([]layered.LEdgeID(nil), arena.Port(portID).OutgoingEdges...) {
				target := arena.Edge(candidate).Target
				if target != nil && len(arena.Port(*target).OutgoingEdges) > 0 {
					reversed = append(reversed, candidate)
				}
			}
			for _, candidate := range append([]layered.LEdgeID(nil), arena.Port(portID).IncomingEdges...) {
				source := arena.Edge(candidate).Source
				if source != nil && len(arena.Port(*source).IncomingEdges) > 0 {
					reversed = append(reversed, candidate)
				}
			}
		}
		for _, candidate := range reversed {
			arena.ReverseEdge(graphID, candidate, true)
		}
	}
	return nil
}

func processCommentBox(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	boxID layered.LNodeID,
	edgeID layered.LEdgeID,
	oppositePort layered.LPortID,
	realNodeID layered.LNodeID,
) {
	topFirst, onlyTop, onlyBottom := true, false, false
	if basegraph.Get(&arena.Node(realNodeID).Properties, layered.PortConstraintsOption).IsSideFixed() {
		hasNorth, hasSouth := false, false
	portLoop:
		for _, portID := range arena.Node(realNodeID).Ports {
			for _, connected := range connectedPorts(arena, portID) {
				if arena.Port(connected).Node == nil ||
					basegraph.Get(&arena.Node(*arena.Port(connected).Node).Properties, core.CommentBox) {
					continue
				}
				switch arena.Port(portID).Side {
				case layered.NorthSide:
					hasNorth = true
					break portLoop
				case layered.SouthSide:
					hasSouth = true
					break portLoop
				}
			}
		}
		onlyTop = hasSouth && !hasNorth
		onlyBottom = hasNorth && !hasSouth
	}
	labels := arena.Node(realNodeID).Labels
	if !onlyTop && !onlyBottom && len(labels) > 0 {
		labelPosition := 0.0
		for _, labelID := range labels {
			label := arena.Label(labelID)
			labelPosition += label.Position.Y + label.Size.Y/2
		}
		labelPosition /= float64(len(labels))
		topFirst = labelPosition >= arena.Node(realNodeID).Size.Y/2
	} else {
		topFirst = !onlyBottom
	}

	topBoxes, hasTop := basegraph.TryGet(&arena.Node(realNodeID).Properties, layered.TopComments)
	bottomBoxes, hasBottom := basegraph.TryGet(&arena.Node(realNodeID).Properties, layered.BottomComments)
	putOnTop := false
	if topFirst {
		switch {
		case !hasTop:
			putOnTop = true
		case onlyTop:
			putOnTop = true
		case !hasBottom:
			putOnTop = false
		default:
			putOnTop = len(topBoxes) <= len(bottomBoxes)
		}
	} else {
		switch {
		case !hasBottom:
			putOnTop = false
		case onlyBottom:
			putOnTop = false
		case !hasTop:
			putOnTop = true
		default:
			putOnTop = len(topBoxes) < len(bottomBoxes)
		}
	}
	if putOnTop {
		basegraph.Set(&arena.Node(realNodeID).Properties, layered.TopComments,
			append(topBoxes, boxID))
	} else {
		basegraph.Set(&arena.Node(realNodeID).Properties, layered.BottomComments,
			append(bottomBoxes, boxID))
	}

	basegraph.Set(&arena.Node(boxID).Properties, layered.CommentConnectionPort, oppositePort)
	if arena.Edge(edgeID).Target != nil && *arena.Edge(edgeID).Target == oppositePort {
		arena.SetEdgeTarget(edgeID, nil)
		if portDegree(arena, oppositePort) == 0 {
			arena.SetPortNode(oppositePort, nil)
		}
		removeHierarchicalCommentPortDummy(arena, oppositePort)
	} else {
		arena.SetEdgeSource(edgeID, nil)
		if portDegree(arena, oppositePort) == 0 {
			arena.SetPortNode(oppositePort, nil)
		}
	}
	arena.Edge(edgeID).BendPoints = nil
	arena.Graph(graphID).LayerlessNodes = removeCommentNode(
		arena.Graph(graphID).LayerlessNodes, boxID)
}

func connectedPorts(arena *layered.LGraphArena, portID layered.LPortID) []layered.LPortID {
	ports := make([]layered.LPortID, 0)
	for _, edgeID := range arena.Port(portID).OutgoingEdges {
		if arena.Edge(edgeID).Target != nil {
			ports = append(ports, *arena.Edge(edgeID).Target)
		}
	}
	for _, edgeID := range arena.Port(portID).IncomingEdges {
		if arena.Edge(edgeID).Source != nil {
			ports = append(ports, *arena.Edge(edgeID).Source)
		}
	}
	return ports
}

func portDegree(arena *layered.LGraphArena, portID layered.LPortID) int {
	port := arena.Port(portID)
	return len(port.IncomingEdges) + len(port.OutgoingEdges)
}

func removeHierarchicalCommentPortDummy(arena *layered.LGraphArena, portID layered.LPortID) {
	dummyID, found := basegraph.TryGet(&arena.Port(portID).Properties, layered.PortDummy)
	if !found || arena.Node(dummyID).Layer == nil {
		return
	}
	layerID := *arena.Node(dummyID).Layer
	graphID := arena.Layer(layerID).Graph
	arena.SetNodeLayer(dummyID, nil)
	if len(arena.Layer(layerID).Nodes) != 0 {
		return
	}
	layers := arena.Graph(graphID).Layers
	for index, candidate := range layers {
		if candidate == layerID {
			arena.Graph(graphID).Layers = append(layers[:index], layers[index+1:]...)
			return
		}
	}
}

// CalculateCommentNodeMargins reserves the exact top, bottom, and horizontal
// protrusion required by comments extracted during preprocessing.
func CalculateCommentNodeMargins(arena *layered.LGraphArena, graphID layered.LGraphID) {
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			topBoxes, hasTop := basegraph.TryGet(&arena.Node(nodeID).Properties, layered.TopComments)
			bottomBoxes, hasBottom := basegraph.TryGet(&arena.Node(nodeID).Properties, layered.BottomComments)
			if !hasTop && !hasBottom {
				continue
			}
			commentGap := layered.IndividualOrDefault(arena, nodeID, &core.SpacingCommentComment)
			nodeGap := layered.IndividualOrDefault(arena, nodeID, &core.SpacingCommentNode)
			topWidth, bottomWidth := 0.0, 0.0
			if hasTop {
				maxHeight := 0.0
				for _, boxID := range topBoxes {
					box := arena.Node(boxID)
					maxHeight = math.Max(maxHeight, box.Size.Y)
					topWidth += box.Size.X
				}
				topWidth += commentGap * float64(len(topBoxes)-1)
				arena.Node(nodeID).Margin.Top += maxHeight + nodeGap
			}
			if hasBottom {
				maxHeight := 0.0
				for _, boxID := range bottomBoxes {
					box := arena.Node(boxID)
					maxHeight = math.Max(maxHeight, box.Size.Y)
					bottomWidth += box.Size.X
				}
				bottomWidth += commentGap * float64(len(bottomBoxes)-1)
				arena.Node(nodeID).Margin.Bottom += maxHeight + nodeGap
			}
			maximumWidth := math.Max(topWidth, bottomWidth)
			if maximumWidth > arena.Node(nodeID).Size.X {
				protrusion := (maximumWidth - arena.Node(nodeID).Size.X) / 2
				arena.Node(nodeID).Margin.Left = math.Max(arena.Node(nodeID).Margin.Left, protrusion)
				arena.Node(nodeID).Margin.Right = math.Max(arena.Node(nodeID).Margin.Right, protrusion)
			}
		}
	}
}

// PostprocessComments reconnects extracted comment edges and puts the comment
// boxes back into their connected node's layer.
func PostprocessComments(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	for _, layerID := range append([]layered.LayerID(nil), arena.Graph(graphID).Layers...) {
		var boxes []layered.LNodeID
		for _, nodeID := range append([]layered.LNodeID(nil), arena.Layer(layerID).Nodes...) {
			topBoxes, hasTop := basegraph.TryGet(&arena.Node(nodeID).Properties, layered.TopComments)
			bottomBoxes, hasBottom := basegraph.TryGet(&arena.Node(nodeID).Properties, layered.BottomComments)
			if !hasTop && !hasBottom {
				continue
			}
			if err := placeComments(arena, nodeID, topBoxes, hasTop, bottomBoxes, hasBottom); err != nil {
				return err
			}
			if hasTop {
				boxes = append(boxes, topBoxes...)
			}
			if hasBottom {
				boxes = append(boxes, bottomBoxes...)
			}
		}
		for _, boxID := range boxes {
			arena.SetNodeLayer(boxID, &layerID)
		}
	}
	return nil
}

func placeComments(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	topBoxes []layered.LNodeID,
	hasTop bool,
	bottomBoxes []layered.LNodeID,
	hasBottom bool,
) error {
	node := arena.Node(nodeID)
	commentGap := layered.IndividualOrDefault(arena, nodeID, &core.SpacingCommentComment)
	if hasTop {
		width, maxHeight := commentGap*float64(len(topBoxes)-1), 0.0
		for _, boxID := range topBoxes {
			width += arena.Node(boxID).Size.X
			maxHeight = math.Max(maxHeight, arena.Node(boxID).Size.Y)
		}
		x := node.Position.X - (width-node.Size.X)/2
		baseline := node.Position.Y - node.Margin.Top + maxHeight
		anchorIncrement := node.Size.X / float64(len(topBoxes)+1)
		anchorX := anchorIncrement
		for _, boxID := range topBoxes {
			box := arena.Node(boxID)
			box.Position.X, box.Position.Y = x, baseline-box.Size.Y
			x += box.Size.X + commentGap
			boxPortID, nodePortID, err := reconnectCommentEdge(arena, boxID)
			if err != nil {
				return err
			}
			boxPort := arena.Port(boxPortID)
			boxPort.Position.X = box.Size.X/2 - boxPort.Anchor.X
			boxPort.Position.Y = box.Size.Y
			if portDegree(arena, nodePortID) == 1 {
				nodePort := arena.Port(nodePortID)
				nodePort.Position.X = anchorX - nodePort.Anchor.X
				nodePort.Position.Y = 0
				arena.SetPortNode(nodePortID, &nodeID)
			}
			anchorX += anchorIncrement
		}
	}
	if hasBottom {
		width, maxHeight := commentGap*float64(len(bottomBoxes)-1), 0.0
		for _, boxID := range bottomBoxes {
			width += arena.Node(boxID).Size.X
			maxHeight = math.Max(maxHeight, arena.Node(boxID).Size.Y)
		}
		x := node.Position.X - (width-node.Size.X)/2
		baseline := node.Position.Y + node.Size.Y + node.Margin.Bottom - maxHeight
		anchorIncrement := node.Size.X / float64(len(bottomBoxes)+1)
		anchorX := anchorIncrement
		for _, boxID := range bottomBoxes {
			box := arena.Node(boxID)
			box.Position.X, box.Position.Y = x, baseline
			x += box.Size.X + commentGap
			boxPortID, nodePortID, err := reconnectCommentEdge(arena, boxID)
			if err != nil {
				return err
			}
			boxPort := arena.Port(boxPortID)
			boxPort.Position.X = box.Size.X/2 - boxPort.Anchor.X
			boxPort.Position.Y = 0
			if portDegree(arena, nodePortID) == 1 {
				nodePort := arena.Port(nodePortID)
				nodePort.Position.X = anchorX - nodePort.Anchor.X
				nodePort.Position.Y = node.Size.Y
				arena.SetPortNode(nodePortID, &nodeID)
			}
			anchorX += anchorIncrement
		}
	}
	return nil
}

func reconnectCommentEdge(
	arena *layered.LGraphArena,
	boxID layered.LNodeID,
) (layered.LPortID, layered.LPortID, error) {
	nodePortID, found := basegraph.TryGet(
		&arena.Node(boxID).Properties, layered.CommentConnectionPort)
	if !found {
		return 0, 0, fmt.Errorf("layered: comment box %d has no connection port", boxID)
	}
	for _, boxPortID := range arena.Node(boxID).Ports {
		if outgoing := arena.Port(boxPortID).OutgoingEdges; len(outgoing) > 0 {
			edgeID := outgoing[0]
			arena.SetEdgeTarget(edgeID, &nodePortID)
			return boxPortID, nodePortID, nil
		}
		if incoming := arena.Port(boxPortID).IncomingEdges; len(incoming) > 0 {
			edgeID := incoming[0]
			arena.SetEdgeSource(edgeID, &nodePortID)
			return boxPortID, nodePortID, nil
		}
	}
	return 0, 0, fmt.Errorf("layered: comment box %d has no connecting edge", boxID)
}

func removeCommentNode(values []layered.LNodeID, target layered.LNodeID) []layered.LNodeID {
	for index, value := range values {
		if value == target {
			return append(values[:index], values[index+1:]...)
		}
	}
	return values
}
