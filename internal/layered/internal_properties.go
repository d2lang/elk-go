// Copyright (c) 2010, 2015 Kiel University and others.
// Copyright (c) 2017 Kiel University and others.
// Copyright (c) 2017, 2018 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package layered

import (
	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
)

// SplineCompactionSegment preserves the SplineSegment state consumed by
// HorizontalGraphCompactor between phase 5 and final spline bend-point
// calculation. Control-point chains alone cannot recover a straight segment's
// bounding box or distinguish it from a one-control-point sloppy curve.
type SplineCompactionSegment struct {
	BoundingBox      geometry.Rectangle
	Straight         bool
	Edge             LEdgeID
	SourcePort       LPortID
	TargetPort       LPortID
	SourceY          float64
	TargetY          float64
	NormalSource     bool
	NormalTarget     bool
	WestOfFirstLayer bool
	XDelta           float64
	Rank             int
}

// OriginKind and Origin preserve the exact original graph element behind an
// internal LGraph element. The extra L* alternatives are used by dummy nodes.
type OriginKind uint8

const (
	OriginNode OriginKind = iota
	OriginPort
	OriginEdge
	OriginLabel
	OriginLEdge
	OriginLNode
	OriginLPort
)

type Origin struct {
	Kind  OriginKind
	Node  basegraph.NodeID
	Port  basegraph.PortID
	Edge  basegraph.EdgeID
	Label basegraph.LabelID
	LNode LNodeID
	LPort LPortID
	LEdge LEdgeID
}

// canonicalID returns the ID of an origin that equals its kind constructor's
// result for that ID.
func (origin Origin) canonicalID() (uint32, bool) {
	var id uint32
	var canonical Origin
	switch origin.Kind {
	case OriginNode:
		id, canonical = uint32(origin.Node), NodeOrigin(origin.Node)
	case OriginPort:
		id, canonical = uint32(origin.Port), PortOrigin(origin.Port)
	case OriginEdge:
		id, canonical = uint32(origin.Edge), EdgeOrigin(origin.Edge)
	case OriginLabel:
		id, canonical = uint32(origin.Label), LabelOrigin(origin.Label)
	case OriginLEdge:
		id, canonical = uint32(origin.LEdge), LEdgeOrigin(origin.LEdge)
	case OriginLNode:
		id, canonical = uint32(origin.LNode), LNodeOrigin(origin.LNode)
	case OriginLPort:
		id, canonical = uint32(origin.LPort), LPortOrigin(origin.LPort)
	default:
		return 0, false
	}
	return id, origin == canonical
}

func NodeOrigin(id basegraph.NodeID) Origin   { return Origin{Kind: OriginNode, Node: id} }
func PortOrigin(id basegraph.PortID) Origin   { return Origin{Kind: OriginPort, Port: id} }
func EdgeOrigin(id basegraph.EdgeID) Origin   { return Origin{Kind: OriginEdge, Edge: id} }
func LabelOrigin(id basegraph.LabelID) Origin { return Origin{Kind: OriginLabel, Label: id} }
func LNodeOrigin(id LNodeID) Origin           { return Origin{Kind: OriginLNode, LNode: id} }
func LPortOrigin(id LPortID) Origin           { return Origin{Kind: OriginLPort, LPort: id} }
func LEdgeOrigin(id LEdgeID) Origin           { return Origin{Kind: OriginLEdge, LEdge: id} }

var (
	OriginalElement                              = basegraph.NewProperty[Origin]("origin")
	CoordinateSystemOrigin                       = basegraph.NewProperty[LGraphID]("coordinateOrigin")
	CompoundNode                                 = basegraph.PropertyWithDefault("compoundNode", func() bool { return false })
	InsideConnections                            = basegraph.PropertyWithDefault("insideConnections", func() bool { return false })
	OriginalBendPoints                           = basegraph.CloneablePropertyWithDefault("originalBendpoints", func() geometry.Chain { return nil })
	OriginalDummyNodePosition                    = basegraph.NewProperty[float64]("originalDummyNodePosition")
	OriginalLabelEdge                            = basegraph.NewProperty[LEdgeID]("originalLabelEdge")
	RepresentedLabels                            = basegraph.CloneablePropertyWithDefault("representedLabels", func() []LLabelID { return nil })
	EndLabelEdge                                 = basegraph.NewProperty[LEdgeID]("endLabel.origin")
	InternalLabelSide                            = basegraph.PropertyWithDefault("labelSide", func() LabelSide { return LabelSideUnknown })
	MaximumEdgeThickness                         = basegraph.PropertyWithDefault("maxEdgeThickness", func() float64 { return 0 })
	Reversed                                     = basegraph.PropertyWithDefault("reversed", func() bool { return false })
	Random                                       = basegraph.NewProperty[*javacompat.Random]("random")
	LongEdgeSource                               = basegraph.NewProperty[LPortID]("longEdgeSource")
	LongEdgeTarget                               = basegraph.NewProperty[LPortID]("longEdgeTarget")
	LongEdgeHasLabelDummies                      = basegraph.PropertyWithDefault("longEdgeHasLabelDummies", func() bool { return false })
	LongEdgeBeforeLabelDummy                     = basegraph.PropertyWithDefault("longEdgeBeforeLabelDummy", func() bool { return false })
	InternalEdgeConstraint                       = basegraph.PropertyWithDefault("edgeConstraint", func() EdgeConstraint { return EdgeConstraintNone })
	InLayerLayoutUnit                            = basegraph.NewProperty[LNodeID]("inLayerLayoutUnit")
	InternalInLayerConstraint                    = basegraph.PropertyWithDefault("inLayerConstraint", func() InLayerConstraint { return InLayerConstraintNone })
	InLayerSuccessorConstraints                  = basegraph.CloneablePropertyWithDefault("inLayerSuccessorConstraint", func() []LNodeID { return nil })
	InLayerSuccessorConstraintsBetweenNonDummies = basegraph.PropertyWithDefault("inLayerSuccessorConstraintBetweenNonDummies", func() bool { return false })
	PortDummy                                    = basegraph.NewProperty[LNodeID]("portDummy")
	CrossingHint                                 = basegraph.PropertyWithDefault("crossingHint", func() int { return 0 })
	GraphProperties                              = basegraph.PropertyWithDefault("graphProperties", func() GraphProperty { return 0 })
	ExternalPortSide                             = basegraph.PropertyWithDefault("externalPortSide", func() PortSide { return UndefinedSide })
	ExternalPortSize                             = basegraph.CloneablePropertyWithConstantDefault("externalPortSize", func() geometry.Point { return geometry.Point{} })
	ExternalPortReplacedDummies                  = basegraph.CloneablePropertyWithDefault("externalPortReplacedDummies", func() []LNodeID { return nil })
	ExternalPortReplacedDummy                    = basegraph.NewProperty[LNodeID]("externalPortReplacedDummy")
	ExternalPortConnections                      = basegraph.PropertyWithDefault("externalPortConnections", func() uint8 { return 0 })
	PortRatioOrPosition                          = basegraph.PropertyWithDefault("portRatioOrPosition", func() float64 { return 0 })
	BarycenterAssociates                         = basegraph.CloneablePropertyWithDefault("barycenterAssociates", func() []LNodeID { return nil })
	TopComments                                  = basegraph.CloneablePropertyWithDefault("TopSideComments", func() []LNodeID { return nil })
	BottomComments                               = basegraph.CloneablePropertyWithDefault("BottomSideComments", func() []LNodeID { return nil })
	CommentConnectionPort                        = basegraph.NewProperty[LPortID]("CommentConnectionPort")
	InputCollect                                 = basegraph.PropertyWithDefault("inputCollect", func() bool { return false })
	OutputCollect                                = basegraph.PropertyWithDefault("outputCollect", func() bool { return false })
	Cyclic                                       = basegraph.PropertyWithDefault("cyclic", func() bool { return false })
	TargetOffset                                 = basegraph.CloneablePropertyWithConstantDefault("targetOffset", func() geometry.Point { return geometry.Point{} })
	PartitionDummy                               = basegraph.PropertyWithDefault("partitionConstraint", func() bool { return false })
	OriginalPortConstraints                      = basegraph.NewProperty[PortConstraints]("originalPortConstraints")
	SplineNorthSouthPortY                        = basegraph.NewProperty[float64]("splines.nsPortY")
	SplineControlPointSegments                   = basegraph.NewProperty[[]geometry.Chain]("splines.controlPointSegments")
	SplineCompactionSegments                     = basegraph.NewProperty[[]SplineCompactionSegment]("splines.compactionSegments")
	ModelOrder                                   = basegraph.NewProperty[int]("modelOrder")
	MaximumModelOrderNodes                       = basegraph.NewProperty[int]("modelOrder.maximum")
	LongEdgeTargetNode                           = basegraph.NewProperty[LNodeID]("longEdgeTargetNode")
	CrossHierarchyEdges                          = basegraph.NewProperty[*CrossHierarchyMap]("crossHierarchyMap")

	// ELK 0.8.1 intentionally gives both properties the same key. Crossing
	// minimization observes them sequentially, so "fixing" the typo changes
	// random-number consumption and layout output.
	FirstTryWithInitialOrder  = basegraph.PropertyWithDefault("firstTryWithInitialOrder", func() bool { return false })
	SecondTryWithInitialOrder = basegraph.PropertyWithDefault("firstTryWithInitialOrder", func() bool { return false })

	HiddenNodes          = basegraph.CloneablePropertyWithDefault("layerConstraints.hiddenNodes", func() []LNodeID { return nil })
	OriginalOppositePort = basegraph.NewProperty[LPortID]("layerConstraints.opposidePort")
	TargetNodeModelOrder = basegraph.NewProperty[map[LNodeID]int]("targetNode.modelOrder")
)
