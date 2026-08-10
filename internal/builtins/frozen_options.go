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
	"strings"

	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
)

// registerCompatibilityOptions completes the metadata surface exposed
// by ELK.js 0.12.0's knownLayoutOptions(). The algorithms consume these values
// by their canonical property IDs, so the declarations live at the registry
// boundary instead of duplicating properties across implementation packages.
func registerCompatibilityOptions(registry *core.OptionRegistry) {
	parents, nodes, edges, ports, labels := core.Parents, core.Nodes, core.Edges, core.Ports, core.Labels
	options := []core.OptionData{
		{ID: core.ResolvedAlgorithm.ID, Kind: core.ObjectOption, Targets: parents, MetadataOnly: true},
		{ID: "org.eclipse.elk.margins", Kind: core.ObjectOption, Targets: nodes,
			Parse: func(value string) (any, error) { return geometry.ParseSpacing(value) }},
		{ID: core.SpacingIndividual.ID, Group: "spacing", Kind: core.ObjectOption,
			Targets: nodes | edges | ports | labels, Parse: parseIndividualSpacings(registry)},
		{ID: "org.eclipse.elk.portAlignment.default", Group: "portAlignment", Kind: core.EnumOption,
			Targets: nodes, EnumValues: []string{"DISTRIBUTED", "JUSTIFIED", "BEGIN", "CENTER", "END"}},
		{ID: "org.eclipse.elk.layered.allowNonFlowPortsToSwitchSides", Kind: core.BooleanOption,
			Targets: ports, LegacyIDs: []string{"org.eclipse.elk.layered.northOrSouthPort"}},
		{ID: "org.eclipse.elk.layered.compaction.connectedComponents", Group: "compaction",
			Kind: core.BooleanOption, Targets: parents},
		{ID: "org.eclipse.elk.layered.considerModelOrder.components", Group: "considerModelOrder",
			Kind: core.EnumOption, Targets: parents,
			EnumValues: []string{"NONE", "INSIDE_PORT_SIDE_GROUPS", "FORCE_MODEL_ORDER"}},
		{ID: "org.eclipse.elk.layered.considerModelOrder.longEdgeStrategy", Group: "considerModelOrder",
			Kind: core.EnumOption, Targets: parents,
			EnumValues: []string{"DUMMY_NODE_OVER", "DUMMY_NODE_UNDER", "EQUAL"}},
		{ID: "org.eclipse.elk.layered.considerModelOrder.crossingCounterNodeInfluence", Group: "considerModelOrder",
			Kind: core.DoubleOption, Targets: parents},
		{ID: "org.eclipse.elk.layered.considerModelOrder.crossingCounterPortInfluence", Group: "considerModelOrder",
			Kind: core.DoubleOption, Targets: parents},
	}
	for _, option := range options {
		if registry.ByID(option.ID) != nil {
			continue
		}
		if err := registry.Register(option); err != nil {
			panic(err)
		}
	}
	normalizeCompatibilityOptionMetadata(registry)
}

// normalizeCompatibilityOptionMetadata pins descriptive fields that provider-local
// registries do not consume during layout but elkjs exposes through
// knownLayoutOptions(). Keeping the corrections centralized makes the public
// metadata oracle auditable without coupling algorithm packages to the JS API.
func normalizeCompatibilityOptionMetadata(registry *core.OptionRegistry) {
	groups := map[string]string{
		"org.eclipse.elk.box.packingMode":                                               "box",
		"org.eclipse.elk.edge.thickness":                                                "edge",
		"org.eclipse.elk.edgeLabels.inline":                                             "edgeLabels",
		"org.eclipse.elk.edgeLabels.placement":                                          "edgeLabels",
		"org.eclipse.elk.layered.considerModelOrder.strategy":                           "considerModelOrder",
		"org.eclipse.elk.layered.crossingMinimization.forceNodeModelOrder":              "crossingMinimization",
		"org.eclipse.elk.layered.crossingMinimization.greedySwitch.activationThreshold": "crossingMinimization.greedySwitch",
		"org.eclipse.elk.layered.crossingMinimization.greedySwitch.type":                "crossingMinimization.greedySwitch",
		"org.eclipse.elk.layered.crossingMinimization.greedySwitchHierarchical.type":    "crossingMinimization.greedySwitchHierarchical",
		"org.eclipse.elk.layered.crossingMinimization.hierarchicalSweepiness":           "crossingMinimization",
		"org.eclipse.elk.layered.crossingMinimization.positionId":                       "crossingMinimization",
		"org.eclipse.elk.layered.crossingMinimization.semiInteractive":                  "crossingMinimization",
		"org.eclipse.elk.layered.crossingMinimization.strategy":                         "crossingMinimization",
		"org.eclipse.elk.layered.cycleBreaking.strategy":                                "cycleBreaking",
		"org.eclipse.elk.layered.edgeLabels.centerLabelPlacementStrategy":               "edgeLabels",
		"org.eclipse.elk.layered.edgeLabels.sideSelection":                              "edgeLabels",
		"org.eclipse.elk.layered.edgeRouting.polyline.slopedEdgeZoneWidth":              "edgeRouting.polyline",
		"org.eclipse.elk.layered.edgeRouting.selfLoopDistribution":                      "edgeRouting",
		"org.eclipse.elk.layered.edgeRouting.selfLoopOrdering":                          "edgeRouting",
		"org.eclipse.elk.layered.layering.coffmanGraham.layerBound":                     "layering.coffmanGraham",
		"org.eclipse.elk.layered.layering.layerConstraint":                              "layering",
		"org.eclipse.elk.layered.layering.layerId":                                      "layering",
		"org.eclipse.elk.layered.layering.minWidth.upperBoundOnWidth":                   "layering.minWidth",
		"org.eclipse.elk.layered.layering.minWidth.upperLayerEstimationScalingFactor":   "layering.minWidth",
		"org.eclipse.elk.layered.layering.strategy":                                     "layering",
		"org.eclipse.elk.layered.nodePlacement.bk.edgeStraightening":                    "nodePlacement.bk",
		"org.eclipse.elk.layered.nodePlacement.bk.fixedAlignment":                       "nodePlacement.bk",
		"org.eclipse.elk.layered.nodePlacement.favorStraightEdges":                      "nodePlacement",
		"org.eclipse.elk.layered.nodePlacement.linearSegments.deflectionDampening":      "nodePlacement.linearSegments",
		"org.eclipse.elk.layered.nodePlacement.strategy":                                "nodePlacement",
		"org.eclipse.elk.layered.priority.direction":                                    "priority",
		"org.eclipse.elk.layered.priority.shortness":                                    "priority",
		"org.eclipse.elk.layered.priority.straightness":                                 "priority",
		"org.eclipse.elk.layered.spacing.baseValue":                                     "spacing",
		"org.eclipse.elk.layered.spacing.edgeEdgeBetweenLayers":                         "spacing",
		"org.eclipse.elk.layered.spacing.edgeNodeBetweenLayers":                         "spacing",
		"org.eclipse.elk.layered.spacing.nodeNodeBetweenLayers":                         "spacing",
		"org.eclipse.elk.nodeLabels.padding":                                            "nodeLabels",
		"org.eclipse.elk.nodeSize.constraints":                                          "nodeSize",
		"org.eclipse.elk.nodeSize.fixedGraphSize":                                       "nodeSize",
		"org.eclipse.elk.nodeSize.minimum":                                              "nodeSize",
		"org.eclipse.elk.nodeSize.options":                                              "nodeSize",
		"org.eclipse.elk.port.anchor":                                                   "port",
		"org.eclipse.elk.port.borderOffset":                                             "port",
		"org.eclipse.elk.port.index":                                                    "port",
		"org.eclipse.elk.port.side":                                                     "port",
		"org.eclipse.elk.spacing.componentComponent":                                    "spacing",
		"org.eclipse.elk.spacing.edgeEdge":                                              "spacing",
		"org.eclipse.elk.spacing.edgeLabel":                                             "spacing",
		"org.eclipse.elk.spacing.edgeNode":                                              "spacing",
		"org.eclipse.elk.spacing.labelLabel":                                            "spacing",
		"org.eclipse.elk.spacing.labelNode":                                             "spacing",
		"org.eclipse.elk.spacing.labelPortHorizontal":                                   "spacing",
		"org.eclipse.elk.spacing.labelPortVertical":                                     "spacing",
		"org.eclipse.elk.spacing.nodeNode":                                              "spacing",
		"org.eclipse.elk.spacing.nodeSelfLoop":                                          "spacing",
		"org.eclipse.elk.spacing.portPort":                                              "spacing",
		"org.eclipse.elk.spacing.portsSurrounding":                                      "spacing",
	}
	targets := map[string]core.Targets{
		"org.eclipse.elk.edgeLabels.inline":                               core.Labels,
		"org.eclipse.elk.hierarchyHandling":                               core.Parents | core.Nodes,
		"org.eclipse.elk.layered.edgeLabels.centerLabelPlacementStrategy": core.Parents | core.Labels,
		"org.eclipse.elk.layered.edgeRouting.selfLoopDistribution":        core.Nodes,
		"org.eclipse.elk.layered.edgeRouting.selfLoopOrdering":            core.Nodes,
		"org.eclipse.elk.noLayout":                                        core.Nodes | core.Edges | core.Ports | core.Labels,
		"org.eclipse.elk.nodeLabels.padding":                              core.Parents,
		"org.eclipse.elk.padding":                                         core.Parents | core.Nodes,
		"org.eclipse.elk.portConstraints":                                 core.Nodes,
		"org.eclipse.elk.spacing.portPort":                                core.Parents | core.Nodes,
	}
	for id, group := range groups {
		option := registry.ByID(id)
		if option == nil {
			panic(fmt.Sprintf("compatibility metadata group references unregistered option %q", id))
		}
		option.Group = group
	}
	for id, target := range targets {
		option := registry.ByID(id)
		if option == nil {
			panic(fmt.Sprintf("compatibility metadata targets reference unregistered option %q", id))
		}
		option.Targets = target
	}
}

// parseIndividualSpacings mirrors IndividualSpacings.parse in ELK 0.8.1.
// The unusual separator is part of the public ELK option string format.
func parseIndividualSpacings(registry *core.OptionRegistry) func(string) (any, error) {
	return func(value string) (any, error) {
		individual := &core.IndividualSpacings{}
		if value == "" {
			return individual, nil
		}
		for _, serialized := range strings.Split(value, ";,;") {
			parts := strings.Split(serialized, ":")
			if len(parts) < 2 {
				return nil, fmt.Errorf("invalid individual spacing %q", serialized)
			}
			option := registry.BySuffix(parts[0])
			if option == nil || option.ID == core.SpacingIndividual.ID {
				return nil, fmt.Errorf("invalid individual spacing option %q", parts[0])
			}
			parsed, ok := option.ParseValue(parts[1])
			if !ok {
				return nil, fmt.Errorf("invalid value %q for individual spacing %q", parts[1], parts[0])
			}
			individual.Properties.SetRaw(option.ID, parsed)
		}
		return individual, nil
	}
}
