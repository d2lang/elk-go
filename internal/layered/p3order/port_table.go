// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import "github.com/d2lang/elk-go/internal/layered"

// portTable is a dense copy of what the layer sweeps read about ports and
// edges, indexed by LPortID for every port of the arena, so the inner loops
// (barycenters, port ranks and barycenters, crossing counts) read small
// arrays instead of 200-byte ports, edges and pointers to IDs.
//
// Everything in it is constant from the end of the holders' initialization
// until the sweeps are over: no port, edge or node is added or removed, no
// edge is reconnected, ports keep their sides and nodes and nodes their
// layers (only the final transfer changes port sides), and the scratch IDs
// LPort.ID are written only by holder initialization. The table is built
// after every holder has been initialized, so each value is the one the
// arena expression it replaces returns during the sweeps. Only the order of
// nodes in layers and of ports in nodes changes, and the sweeps keep reading
// those from the node order and the arena.
type portTable struct {
	ports []portEntry // by LPortID
	ends  []edgeEnd   // each port's incoming, then outgoing, edge ends
}

// portEntry is one port: LPort.ID, Side, len(IncomingEdges) and
// len(OutgoingEdges), and where its edge ends start.
type portEntry struct {
	id    int32
	start int32
	in    int32
	out   int32
	side  layered.PortSide
}

// edgeEnd is one edge seen from one of its ports, in the order of that
// port's IncomingEdges or OutgoingEdges: the edge's other port (the
// source of an incoming edge, the target of an outgoing one, as
// EdgeOtherPort returns them), that port's LPort.ID, node and the node's
// layer, and EdgeIsInLayer.
type edgeEnd struct {
	otherLayer int64 // LayerID, or noLayer
	otherPort  layered.LPortID
	otherID    int32
	otherNode  layered.LNodeID
	inLayer    bool
}

const noLayer = -1

// layerKey encodes a node's Layer field so that sameLayer(a, b) is
// layerKey(a) == layerKey(b).
func layerKey(layer *layered.LayerID) int64 {
	if layer == nil {
		return noLayer
	}
	return int64(*layer)
}

func (table *portTable) incoming(entry *portEntry) []edgeEnd {
	return table.ends[entry.start : entry.start+entry.in]
}

func (table *portTable) outgoing(entry *portEntry) []edgeEnd {
	return table.ends[entry.start+entry.in : entry.start+entry.in+entry.out]
}

func (table *portTable) connected(entry *portEntry) []edgeEnd {
	return table.ends[entry.start : entry.start+entry.in+entry.out]
}

// buildPortTable builds the table, or returns nil when some port or edge is
// not well formed (an edge missing from its endpoint's list or with a
// missing endpoint, an endpoint without a node, an ID beyond int32), in
// which case the sweeps read the arena as before.
func buildPortTable(ws *crossMinWorkspace, arena *layered.LGraphArena) *portTable {
	edges := 0
	for index := range arena.Ports {
		port := &arena.Ports[index]
		if int(int32(port.ID)) != port.ID {
			return nil
		}
		edges += len(port.IncomingEdges) + len(port.OutgoingEdges)
	}
	table := &ws.portTable
	table.ports = ws.portEntries.take(len(arena.Ports))
	table.ends = ws.edgeEnds.take(edges)
	used := 0
	for index := range arena.Ports {
		portID := layered.LPortID(index)
		port := &arena.Ports[index]
		table.ports[index] = portEntry{
			id:    int32(port.ID),
			start: int32(used),
			in:    int32(len(port.IncomingEdges)),
			out:   int32(len(port.OutgoingEdges)),
			side:  port.Side,
		}
		if len(port.IncomingEdges)+len(port.OutgoingEdges) == 0 {
			continue
		}
		// Both endpoints of each edge must have nodes; the edge's endpoints
		// are this port and the other one, so its node and layer key are the
		// same for all its edges.
		if port.Node == nil {
			return nil
		}
		node := *port.Node
		layer := layerKey(arena.Node(node).Layer)
		for _, edgeID := range port.IncomingEdges {
			edge := arena.Edge(edgeID)
			if edge.Target == nil || *edge.Target != portID || edge.Source == nil {
				return nil
			}
			end, ok := makeEdgeEndAt(arena, node, layer, *edge.Source)
			if !ok {
				return nil
			}
			table.ends[used] = end
			used++
		}
		for _, edgeID := range port.OutgoingEdges {
			edge := arena.Edge(edgeID)
			if edge.Source == nil || *edge.Source != portID || edge.Target == nil {
				return nil
			}
			end, ok := makeEdgeEndAt(arena, node, layer, *edge.Target)
			if !ok {
				return nil
			}
			table.ends[used] = end
			used++
		}
	}
	return table
}

// makeEdgeEndAt describes an edge between a port of node (whose layer key is
// layer) and other, with other as its other port; it reports false when
// other has no node. The endpoints' nodes are node and other's, so
// EdgeIsInLayer (not a self loop, nodes in the same layer) is that the two
// nodes differ and have the same layer key.
func makeEdgeEndAt(arena *layered.LGraphArena, node layered.LNodeID, layer int64, other layered.LPortID) (edgeEnd, bool) {
	otherPort := arena.Port(other)
	if otherPort.Node == nil {
		return edgeEnd{}, false
	}
	otherNode := *otherPort.Node
	otherLayer := layerKey(arena.Node(otherNode).Layer)
	return edgeEnd{
		otherPort:  other,
		otherID:    int32(otherPort.ID),
		otherNode:  otherNode,
		otherLayer: otherLayer,
		inLayer:    otherNode != node && otherLayer == layer,
	}, true
}
