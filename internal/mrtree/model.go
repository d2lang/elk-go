// Copyright (c) 2013, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// http://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package mrtree

import (
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/selfloops"
)

// These types retain the shape and incidence semantics of TGraph, TNode, and
// TEdge. During Walker placement node positions are centers. The direction and
// node-position processors then turn them into the upper-left coordinates used
// by ELK 0.12's routing and component-packing phases.
type tGraph struct {
	nodes                []*tNode
	edges                []*tEdge
	padding              geometry.Spacing
	spacing              float64
	spacingEdgeNode      float64
	edgeEndTextureLength float64
	aspectRatio          float64
	separate             bool
	searchOrder          string
	weighting            string
	direction            string
	edgeRoutingMode      string
	compaction           bool
	fixedGraphSize       bool
	graphMin             geometry.Point
	graphMax             geometry.Point
}

type nodeLoops struct {
	*selfloops.Plan
	shapeSize geometry.Point
}

type tNode struct {
	id       int
	origin   *graph.NodeID
	position geometry.Point
	size     geometry.Point
	loops    *nodeLoops
	props    graph.PropertyMap
	outgoing []*tEdge
	incoming []*tEdge

	root, dummy           bool
	fan, descendants      int
	leftNeighbor          *tNode
	rightNeighbor         *tNode
	leftSibling           *tNode
	rightSibling          *tNode
	positionIndex         int
	prelim, modifier      float64
	levelHeight           float64
	treeLevel             int
	levelMin              float64
	levelMax              float64
	compactConstraints    []*tNode
	compactLevelAscension bool
	xCoordinate           int
	yCoordinate           int
}

type tEdge struct {
	origin     *graph.EdgeID
	source     *tNode
	target     *tNode
	bendPoints geometry.Chain
	dummy      bool
}

func (node *tNode) children() []*tNode {
	children := make([]*tNode, 0, len(node.outgoing))
	for _, edge := range node.outgoing {
		children = append(children, edge.target)
	}
	return children
}

func (node *tNode) parent() *tNode {
	if len(node.incoming) == 0 {
		return nil
	}
	return node.incoming[0].source
}

func (node *tNode) leaf() bool { return len(node.outgoing) == 0 }

func (g *tGraph) addDummyChild(parent, child *tNode) {
	edge := &tEdge{source: parent, target: child, dummy: true}
	g.edges = append(g.edges, edge)
	parent.outgoing = append(parent.outgoing, edge)
	child.incoming = append(child.incoming, edge)
}
