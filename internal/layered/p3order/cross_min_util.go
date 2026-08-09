// Copyright (c) 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import "github.com/d2lang/elk-go/internal/layered"

// portsInNorthSouthEastWestOrder implements CrossMinUtil's deliberately
// asymmetric iteration order. LNode's side views are clockwise; crossing
// counting needs NORTH/EAST as-is and SOUTH/WEST reversed.
func portsInNorthSouthEastWestOrder(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	side layered.PortSide,
) []layered.LPortID {
	view := arena.NodePortSideView(nodeID, side)
	switch side {
	case layered.NorthSide, layered.EastSide:
		return view
	case layered.SouthSide, layered.WestSide:
		result := append([]layered.LPortID(nil), view...)
		for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
			result[left], result[right] = result[right], result[left]
		}
		return result
	default:
		return nil
	}
}
