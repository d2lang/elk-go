// Copyright (c) 2016, 2020 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package builtins

// The filename is retained for source-history continuity. The declarations
// below are not a frozen 0.8 compatibility layer: they complete and normalize
// the current official elkjs 0.12.0 metadata surface.

import (
	"fmt"

	"github.com/d2lang/elk-go/internal/core"
)

// compatibilityOptionSpecs completes the metadata surface exposed by
// ELK.js 0.12.0's knownLayoutOptions(). The algorithms consume these values
// by their canonical property IDs, so the declarations live at the registry
// boundary instead of duplicating properties across implementation packages.
var compatibilityOptionSpecs = [...]core.OptionSpec{
	{ID: core.ResolvedAlgorithm.ID, Kind: core.ObjectOption, Targets: core.Parents, MetadataOnly: true},
	{ID: "org.eclipse.elk.margins", Kind: core.ObjectOption, Targets: core.Nodes,
		Parse: core.SpacingParser},
	{ID: core.SpacingIndividual.ID, Group: "spacing", Kind: core.ObjectOption,
		Targets: core.Nodes | core.Edges | core.Ports | core.Labels, Parse: core.IndividualSpacingsParser},
	{ID: "org.eclipse.elk.portAlignment.default", Group: "portAlignment", Kind: core.EnumOption,
		Targets: core.Nodes, EnumValues: "DISTRIBUTED JUSTIFIED BEGIN CENTER END"},
	{ID: "org.eclipse.elk.layered.allowNonFlowPortsToSwitchSides", Kind: core.BooleanOption,
		Targets: core.Ports, LegacyIDs: "org.eclipse.elk.layered.northOrSouthPort"},
	{ID: "org.eclipse.elk.layered.compaction.connectedComponents", Group: "compaction",
		Kind: core.BooleanOption, Targets: core.Parents},
	{ID: "org.eclipse.elk.layered.considerModelOrder.components", Group: "considerModelOrder",
		Kind: core.EnumOption, Targets: core.Parents,
		EnumValues: "NONE INSIDE_PORT_SIDE_GROUPS FORCE_MODEL_ORDER"},
	{ID: "org.eclipse.elk.layered.considerModelOrder.longEdgeStrategy", Group: "considerModelOrder",
		Kind: core.EnumOption, Targets: core.Parents,
		EnumValues: "DUMMY_NODE_OVER DUMMY_NODE_UNDER EQUAL"},
	{ID: "org.eclipse.elk.layered.considerModelOrder.crossingCounterNodeInfluence", Group: "considerModelOrder",
		Kind: core.DoubleOption, Targets: core.Parents},
	{ID: "org.eclipse.elk.layered.considerModelOrder.crossingCounterPortInfluence", Group: "considerModelOrder",
		Kind: core.DoubleOption, Targets: core.Parents},
}

// registerCompatibilityOptions registers compatibilityOptionSpecs and pins
// the descriptive metadata of the options registered so far.
func registerCompatibilityOptions(registry *core.OptionRegistry) {
	for index := range compatibilityOptionSpecs {
		registry.RegisterMissing(compatibilityOptionSpecs[index])
	}
	normalizeCompatibilityOptionMetadata(registry)
}

// compatibilityGroups and compatibilityTargets pin descriptive fields that
// provider-local registries do not consume during layout but elkjs exposes
// through knownLayoutOptions(). Keeping the corrections centralized makes the
// public metadata oracle auditable without coupling algorithm packages to the
// JS API. Every option is listed at most once in each table.
var compatibilityGroups = [...]struct{ id, group string }{
	{"org.eclipse.elk.box.packingMode", "box"},
	{"org.eclipse.elk.edge.thickness", "edge"},
	{"org.eclipse.elk.edgeLabels.inline", "edgeLabels"},
	{"org.eclipse.elk.edgeLabels.placement", "edgeLabels"},
	{"org.eclipse.elk.layered.considerModelOrder.strategy", "considerModelOrder"},
	{"org.eclipse.elk.layered.crossingMinimization.forceNodeModelOrder", "crossingMinimization"},
	{"org.eclipse.elk.layered.crossingMinimization.greedySwitch.activationThreshold", "crossingMinimization.greedySwitch"},
	{"org.eclipse.elk.layered.crossingMinimization.greedySwitch.type", "crossingMinimization.greedySwitch"},
	{"org.eclipse.elk.layered.crossingMinimization.greedySwitchHierarchical.type", "crossingMinimization.greedySwitchHierarchical"},
	{"org.eclipse.elk.layered.crossingMinimization.hierarchicalSweepiness", "crossingMinimization"},
	{"org.eclipse.elk.layered.crossingMinimization.positionId", "crossingMinimization"},
	{"org.eclipse.elk.layered.crossingMinimization.semiInteractive", "crossingMinimization"},
	{"org.eclipse.elk.layered.crossingMinimization.strategy", "crossingMinimization"},
	{"org.eclipse.elk.layered.cycleBreaking.strategy", "cycleBreaking"},
	{"org.eclipse.elk.layered.edgeLabels.centerLabelPlacementStrategy", "edgeLabels"},
	{"org.eclipse.elk.layered.edgeLabels.sideSelection", "edgeLabels"},
	{"org.eclipse.elk.layered.edgeRouting.polyline.slopedEdgeZoneWidth", "edgeRouting.polyline"},
	{"org.eclipse.elk.layered.edgeRouting.selfLoopDistribution", "edgeRouting"},
	{"org.eclipse.elk.layered.edgeRouting.selfLoopOrdering", "edgeRouting"},
	{"org.eclipse.elk.layered.layering.coffmanGraham.layerBound", "layering.coffmanGraham"},
	{"org.eclipse.elk.layered.layering.layerConstraint", "layering"},
	{"org.eclipse.elk.layered.layering.layerId", "layering"},
	{"org.eclipse.elk.layered.layering.minWidth.upperBoundOnWidth", "layering.minWidth"},
	{"org.eclipse.elk.layered.layering.minWidth.upperLayerEstimationScalingFactor", "layering.minWidth"},
	{"org.eclipse.elk.layered.layering.strategy", "layering"},
	{"org.eclipse.elk.layered.nodePlacement.bk.edgeStraightening", "nodePlacement.bk"},
	{"org.eclipse.elk.layered.nodePlacement.bk.fixedAlignment", "nodePlacement.bk"},
	{"org.eclipse.elk.layered.nodePlacement.favorStraightEdges", "nodePlacement"},
	{"org.eclipse.elk.layered.nodePlacement.linearSegments.deflectionDampening", "nodePlacement.linearSegments"},
	{"org.eclipse.elk.layered.nodePlacement.strategy", "nodePlacement"},
	{"org.eclipse.elk.layered.priority.direction", "priority"},
	{"org.eclipse.elk.layered.priority.shortness", "priority"},
	{"org.eclipse.elk.layered.priority.straightness", "priority"},
	{"org.eclipse.elk.layered.spacing.baseValue", "spacing"},
	{"org.eclipse.elk.layered.spacing.edgeEdgeBetweenLayers", "spacing"},
	{"org.eclipse.elk.layered.spacing.edgeNodeBetweenLayers", "spacing"},
	{"org.eclipse.elk.layered.spacing.nodeNodeBetweenLayers", "spacing"},
	{"org.eclipse.elk.nodeLabels.padding", "nodeLabels"},
	{"org.eclipse.elk.nodeSize.constraints", "nodeSize"},
	{"org.eclipse.elk.nodeSize.fixedGraphSize", "nodeSize"},
	{"org.eclipse.elk.nodeSize.minimum", "nodeSize"},
	{"org.eclipse.elk.nodeSize.options", "nodeSize"},
	{"org.eclipse.elk.port.anchor", "port"},
	{"org.eclipse.elk.port.borderOffset", "port"},
	{"org.eclipse.elk.port.index", "port"},
	{"org.eclipse.elk.port.side", "port"},
	{"org.eclipse.elk.spacing.componentComponent", "spacing"},
	{"org.eclipse.elk.spacing.edgeEdge", "spacing"},
	{"org.eclipse.elk.spacing.edgeLabel", "spacing"},
	{"org.eclipse.elk.spacing.edgeNode", "spacing"},
	{"org.eclipse.elk.spacing.labelLabel", "spacing"},
	{"org.eclipse.elk.spacing.labelNode", "spacing"},
	{"org.eclipse.elk.spacing.labelPortHorizontal", "spacing"},
	{"org.eclipse.elk.spacing.labelPortVertical", "spacing"},
	{"org.eclipse.elk.spacing.nodeNode", "spacing"},
	{"org.eclipse.elk.spacing.nodeSelfLoop", "spacing"},
	{"org.eclipse.elk.spacing.portPort", "spacing"},
	{"org.eclipse.elk.spacing.portsSurrounding", "spacing"},
}

var compatibilityTargets = [...]struct {
	id      string
	targets core.Targets
}{
	{"org.eclipse.elk.edgeLabels.inline", core.Labels},
	{"org.eclipse.elk.hierarchyHandling", core.Parents | core.Nodes},
	{"org.eclipse.elk.layered.edgeLabels.centerLabelPlacementStrategy", core.Parents | core.Labels},
	{"org.eclipse.elk.layered.edgeRouting.selfLoopDistribution", core.Nodes},
	{"org.eclipse.elk.layered.edgeRouting.selfLoopOrdering", core.Nodes},
	{"org.eclipse.elk.noLayout", core.Nodes | core.Edges | core.Ports | core.Labels},
	{"org.eclipse.elk.nodeLabels.padding", core.Parents},
	{"org.eclipse.elk.padding", core.Parents | core.Nodes},
	{"org.eclipse.elk.portConstraints", core.Nodes},
	{"org.eclipse.elk.spacing.portPort", core.Parents | core.Nodes},
}

func normalizeCompatibilityOptionMetadata(registry *core.OptionRegistry) {
	for index := range compatibilityGroups {
		correction := compatibilityGroups[index]
		option := registry.ByID(correction.id)
		if option == nil {
			panic(fmt.Sprintf("compatibility metadata group references unregistered option %q", correction.id))
		}
		option.Group = correction.group
	}
	for index := range compatibilityTargets {
		correction := compatibilityTargets[index]
		option := registry.ByID(correction.id)
		if option == nil {
			panic(fmt.Sprintf("compatibility metadata targets reference unregistered option %q", correction.id))
		}
		option.Targets = correction.targets
	}
}
