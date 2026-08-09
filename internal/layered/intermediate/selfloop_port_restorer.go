// Copyright (c) 2019 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"fmt"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
	"github.com/d2lang/elk-go/internal/layered/intermediate/loops"
	"github.com/d2lang/elk-go/internal/layered/intermediate/loops/ordering"
)

// RestoreSelfLoopPorts restores hidden self-loop ports and computes the loop
// types needed by routing.
func RestoreSelfLoopPorts(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			node := arena.Node(nodeID)
			if node.Type != layered.NormalNode {
				continue
			}
			holder, found := basegraph.TryGet(&node.Properties, loops.HolderProperty)
			if !found {
				continue
			}
			if err := restoreNodeSelfLoopPorts(arena, holder); err != nil {
				return err
			}
		}
	}
	return nil
}

func restoreNodeSelfLoopPorts(arena *layered.LGraphArena, holder *loops.Holder) error {
	if holder.PortsHidden {
		switch basegraph.Get(&arena.Node(holder.LNode).Properties, layered.OriginalPortConstraints) {
		case layered.PortConstraintsUndefined, layered.PortConstraintsFree:
			if err := ordering.AssignPortSides(arena, holder); err != nil {
				return err
			}
		case layered.PortConstraintsFixedSide:
			// The sides were preserved while the ports were hidden.
		default:
			return fmt.Errorf("hidden self-loop ports require non-fixed port order")
		}
	}

	for _, loop := range holder.HyperLoops {
		if err := loop.ComputePortsPerSide(arena); err != nil {
			return err
		}
	}
	if holder.PortsHidden {
		return ordering.RestorePorts(arena, holder)
	}
	return nil
}
