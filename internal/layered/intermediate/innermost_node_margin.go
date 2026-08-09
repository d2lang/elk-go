// Copyright (c) 2010, 2019 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"math"

	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/layered"
)

// CalculateInnermostNodeMargins ports ELK 0.8.1's
// InnermostNodeMarginCalculator. In particular, it deliberately excludes edge
// head and tail labels: EndLabelPreprocessor places those precisely after this
// baseline margin has been recomputed.
func CalculateInnermostNodeMargins(arena *layered.LGraphArena, graphID layered.LGraphID) {
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			calculateInnermostNodeMargin(arena, nodeID)
		}
	}
}

func calculateInnermostNodeMargin(arena *layered.LGraphArena, nodeID layered.LNodeID) {
	node := arena.Node(nodeID)
	boundingBox := geometry.Rectangle{
		X:      node.Position.X,
		Y:      node.Position.Y,
		Width:  node.Size.X,
		Height: node.Size.Y,
	}

	for _, labelID := range node.Labels {
		label := arena.Label(labelID)
		boundingBox = boundingBox.Union(geometry.Rectangle{
			X:      node.Position.X + label.Position.X,
			Y:      node.Position.Y + label.Position.Y,
			Width:  label.Size.X,
			Height: label.Size.Y,
		})
	}
	for _, portID := range node.Ports {
		port := arena.Port(portID)
		portX := node.Position.X + port.Position.X
		portY := node.Position.Y + port.Position.Y
		boundingBox = boundingBox.Union(geometry.Rectangle{
			X:      portX,
			Y:      portY,
			Width:  port.Size.X,
			Height: port.Size.Y,
		})
		for _, labelID := range port.Labels {
			label := arena.Label(labelID)
			boundingBox = boundingBox.Union(geometry.Rectangle{
				X:      portX + label.Position.X,
				Y:      portY + label.Position.Y,
				Width:  label.Size.X,
				Height: label.Size.Y,
			})
		}
	}

	node.Margin.Top = math.Max(0, node.Position.Y-boundingBox.Y)
	node.Margin.Bottom = math.Max(0, boundingBox.MaxY()-(node.Position.Y+node.Size.Y))
	node.Margin.Left = math.Max(0, node.Position.X-boundingBox.X)
	node.Margin.Right = math.Max(0, boundingBox.MaxX()-(node.Position.X+node.Size.X))
}
