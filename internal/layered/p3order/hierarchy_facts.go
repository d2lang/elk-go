// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// The hierarchical port helpers (sortPortDummiesByPortPositions and
// sortPortsByDummyPositionsInLastLayer) read InsideConnections and PortDummy
// of ports and OriginalElement of port dummies after every nested sweep. A
// minimization with a workspace reads each of them once: nothing writes
// these properties while the minimizer runs and none of the reads
// materializes a default, so a remembered value equals a new read.

// portFact is a port's InsideConnections and PortDummy, once known.
type portFact struct {
	known    bool
	inside   bool
	hasDummy bool
	dummy    layered.LNodeID
}

// originFact is whether a node's OriginalElement is an LPort, and which,
// once known.
type originFact struct {
	known  bool
	isPort bool
	port   layered.LPortID
}

// insideConnections is Get(port, InsideConnections).
func (minimizer *LayerSweepCrossingMinimizer) insideConnections(arena *layered.LGraphArena, portID layered.LPortID) bool {
	if fact := minimizer.portFact(arena, portID); fact != nil {
		return fact.inside
	}
	return basegraph.Get(&arena.Port(portID).Properties, layered.InsideConnections)
}

// portDummy is TryGet(port, PortDummy).
func (minimizer *LayerSweepCrossingMinimizer) portDummy(arena *layered.LGraphArena, portID layered.LPortID) (layered.LNodeID, bool) {
	if fact := minimizer.portFact(arena, portID); fact != nil {
		return fact.dummy, fact.hasDummy
	}
	return basegraph.TryGet(&arena.Port(portID).Properties, layered.PortDummy)
}

// portFact returns portID's remembered facts, reading them on first use;
// nil without a workspace.
func (minimizer *LayerSweepCrossingMinimizer) portFact(arena *layered.LGraphArena, portID layered.LPortID) *portFact {
	ws := minimizer.ws
	if ws == nil {
		return nil
	}
	if ws.portFactList == nil {
		ws.portFactList = ws.portFacts.take(len(arena.Ports))
	}
	if int(portID) >= len(ws.portFactList) {
		return nil
	}
	fact := &ws.portFactList[portID]
	if !fact.known {
		properties := &arena.Port(portID).Properties
		fact.inside = basegraph.Get(properties, layered.InsideConnections)
		fact.dummy, fact.hasDummy = basegraph.TryGet(properties, layered.PortDummy)
		fact.known = true
	}
	return fact
}

// lportOrigin is the LPort origin of nodeID (TryGet(node, OriginalElement)
// with kind OriginLPort).
func (minimizer *LayerSweepCrossingMinimizer) lportOrigin(arena *layered.LGraphArena, nodeID layered.LNodeID) (layered.LPortID, bool) {
	ws := minimizer.ws
	if ws != nil {
		if ws.originFactList == nil {
			ws.originFactList = ws.originFacts.take(len(arena.Nodes))
		}
		if int(nodeID) < len(ws.originFactList) {
			fact := &ws.originFactList[nodeID]
			if !fact.known {
				origin, found := basegraph.TryGet(&arena.Node(nodeID).Properties, layered.OriginalElement)
				fact.isPort, fact.port = found && origin.Kind == layered.OriginLPort, origin.LPort
				fact.known = true
			}
			return fact.port, fact.isPort
		}
	}
	origin, found := basegraph.TryGet(&arena.Node(nodeID).Properties, layered.OriginalElement)
	return origin.LPort, found && origin.Kind == layered.OriginLPort
}
