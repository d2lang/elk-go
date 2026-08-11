// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0/.
//
// SPDX-License-Identifier: EPL-2.0

package builtins

import (
	"reflect"
	"testing"

	"github.com/d2lang/elk-go/internal/core"
)

type compatibilityOption struct {
	id           string
	group        string
	kind         core.OptionKind
	targets      core.Targets
	enums        []string
	metadataOnly bool
}

// TestKnownLayoutOptionsGapParity pins the metadata returned by official
// elkjs 0.12.0's knownLayoutOptions() for the provider-independent option surface.
// Enum values come from the matching ELK 0.12.0 generated metadata sources.
func TestKnownLayoutOptionsGapParity(t *testing.T) {
	parents, nodes, edges, labels := core.Parents, core.Nodes, core.Edges, core.Labels
	want := []compatibilityOption{
		{"org.eclipse.elk.commentBox", "", core.BooleanOption, nodes, nil, false},
		{"org.eclipse.elk.hypernode", "", core.BooleanOption, nodes, nil, false},
		{"org.eclipse.elk.insideSelfLoops.activate", "insideSelfLoops", core.BooleanOption, nodes, nil, false},
		{"org.eclipse.elk.insideSelfLoops.yo", "insideSelfLoops", core.BooleanOption, edges, nil, false},
		{"org.eclipse.elk.interactiveLayout", "", core.BooleanOption, parents, nil, false},
		{"org.eclipse.elk.layered.compaction.postCompaction.constraints", "compaction.postCompaction", core.EnumOption, parents, []string{"QUADRATIC", "SCANLINE"}, false},
		{"org.eclipse.elk.layered.compaction.postCompaction.strategy", "compaction.postCompaction", core.EnumOption, parents, []string{"NONE", "LEFT", "RIGHT", "LEFT_RIGHT_CONSTRAINT_LOCKING", "LEFT_RIGHT_CONNECTION_LOCKING", "EDGE_LENGTH"}, false},
		{"org.eclipse.elk.layered.considerModelOrder.noModelOrder", "considerModelOrder", core.BooleanOption, nodes, nil, false},
		{"org.eclipse.elk.layered.crossingMinimization.positionChoiceConstraint", "crossingMinimization", core.IntegerOption, nodes, nil, false},
		{"org.eclipse.elk.layered.edgeRouting.splines.mode", "edgeRouting.splines", core.EnumOption, parents, []string{"SLOPPY", "CONSERVATIVE", "CONSERVATIVE_SOFT"}, false},
		{"org.eclipse.elk.layered.edgeRouting.splines.sloppy.layerSpacingFactor", "edgeRouting.splines.sloppy", core.DoubleOption, parents, nil, false},
		{"org.eclipse.elk.layered.highDegreeNodes.threshold", "highDegreeNodes", core.IntegerOption, parents, nil, false},
		{"org.eclipse.elk.layered.highDegreeNodes.treatment", "highDegreeNodes", core.BooleanOption, parents, nil, false},
		{"org.eclipse.elk.layered.highDegreeNodes.treeHeight", "highDegreeNodes", core.IntegerOption, parents, nil, false},
		{"org.eclipse.elk.layered.layering.layerChoiceConstraint", "layering", core.IntegerOption, nodes, nil, false},
		{"org.eclipse.elk.layered.layering.nodePromotion.maxIterations", "layering.nodePromotion", core.IntegerOption, parents, nil, false},
		{"org.eclipse.elk.layered.layering.nodePromotion.strategy", "layering.nodePromotion", core.EnumOption, parents, []string{"NONE", "NIKOLOV", "NIKOLOV_PIXEL", "NIKOLOV_IMPROVED", "NIKOLOV_IMPROVED_PIXEL", "DUMMYNODE_PERCENTAGE", "NODECOUNT_PERCENTAGE", "NO_BOUNDARY", "MODEL_ORDER_LEFT_TO_RIGHT", "MODEL_ORDER_RIGHT_TO_LEFT"}, false},
		{"org.eclipse.elk.layered.nodePlacement.networkSimplex.nodeFlexibility", "nodePlacement.networkSimplex", core.EnumOption, nodes, []string{"NONE", "PORT_POSITION", "NODE_SIZE_WHERE_SPACE_PERMITS", "NODE_SIZE"}, false},
		{"org.eclipse.elk.layered.nodePlacement.networkSimplex.nodeFlexibility.default", "nodePlacement.networkSimplex.nodeFlexibility", core.EnumOption, parents, []string{"NONE", "PORT_POSITION", "NODE_SIZE_WHERE_SPACE_PERMITS", "NODE_SIZE"}, false},
		{"org.eclipse.elk.layered.portSortingStrategy", "", core.EnumOption, parents, []string{"INPUT_ORDER", "PORT_DEGREE"}, false},
		{"org.eclipse.elk.layered.wrapping.additionalEdgeSpacing", "wrapping", core.DoubleOption, parents, nil, false},
		{"org.eclipse.elk.layered.wrapping.correctionFactor", "wrapping", core.DoubleOption, parents, nil, false},
		{"org.eclipse.elk.layered.wrapping.cutting.cuts", "wrapping.cutting", core.ObjectOption, parents, nil, false},
		{"org.eclipse.elk.layered.wrapping.cutting.msd.freedom", "wrapping.cutting.msd", core.IntegerOption, parents, nil, false},
		{"org.eclipse.elk.layered.wrapping.cutting.strategy", "wrapping.cutting", core.EnumOption, parents, []string{"ARD", "MSD", "MANUAL"}, false},
		{"org.eclipse.elk.layered.wrapping.multiEdge.distancePenalty", "wrapping.multiEdge", core.DoubleOption, parents, nil, false},
		{"org.eclipse.elk.layered.wrapping.multiEdge.improveCuts", "wrapping.multiEdge", core.BooleanOption, parents, nil, false},
		{"org.eclipse.elk.layered.wrapping.multiEdge.improveWrappedEdges", "wrapping.multiEdge", core.BooleanOption, parents, nil, false},
		{"org.eclipse.elk.layered.wrapping.strategy", "wrapping", core.EnumOption, parents, []string{"OFF", "SINGLE_EDGE", "MULTI_EDGE"}, false},
		{"org.eclipse.elk.layered.wrapping.validify.forbiddenIndices", "wrapping.validify", core.ObjectOption, parents, nil, false},
		{"org.eclipse.elk.layered.wrapping.validify.strategy", "wrapping.validify", core.EnumOption, parents, []string{"NO", "GREEDY", "LOOK_BACK"}, false},
		{"org.eclipse.elk.partitioning.activate", "partitioning", core.BooleanOption, parents, nil, false},
		{"org.eclipse.elk.partitioning.partition", "partitioning", core.IntegerOption, parents | nodes, nil, false},
		{"org.eclipse.elk.portAlignment.east", "portAlignment", core.EnumOption, nodes, []string{"DISTRIBUTED", "JUSTIFIED", "BEGIN", "CENTER", "END"}, false},
		{"org.eclipse.elk.portAlignment.north", "portAlignment", core.EnumOption, nodes, []string{"DISTRIBUTED", "JUSTIFIED", "BEGIN", "CENTER", "END"}, false},
		{"org.eclipse.elk.portAlignment.south", "portAlignment", core.EnumOption, nodes, []string{"DISTRIBUTED", "JUSTIFIED", "BEGIN", "CENTER", "END"}, false},
		{"org.eclipse.elk.portAlignment.west", "portAlignment", core.EnumOption, nodes, []string{"DISTRIBUTED", "JUSTIFIED", "BEGIN", "CENTER", "END"}, false},
		{"org.eclipse.elk.portLabels.nextToPortIfPossible", "portLabels", core.BooleanOption, nodes, nil, false},
		{"org.eclipse.elk.portLabels.treatAsGroup", "portLabels", core.BooleanOption, nodes, nil, false},
		{"org.eclipse.elk.spacing.commentComment", "spacing", core.DoubleOption, parents, nil, false},
		{"org.eclipse.elk.spacing.commentNode", "spacing", core.DoubleOption, parents, nil, false},
		{"org.eclipse.elk.animTimeFactor", "", core.IntegerOption, parents, nil, true},
		{"org.eclipse.elk.animate", "", core.BooleanOption, parents, nil, true},
		{"org.eclipse.elk.edge.type", "edge", core.EnumOption, edges, []string{"NONE", "DIRECTED", "UNDIRECTED", "ASSOCIATION", "GENERALIZATION", "DEPENDENCY"}, true},
		{"org.eclipse.elk.font.name", "font", core.StringOption, labels, nil, true},
		{"org.eclipse.elk.font.size", "font", core.IntegerOption, labels, nil, true},
		{"org.eclipse.elk.labelManager", "", core.ObjectOption, parents | labels, nil, true},
		{"org.eclipse.elk.layoutAncestors", "", core.BooleanOption, parents, nil, true},
		{"org.eclipse.elk.maxAnimTime", "", core.IntegerOption, parents, nil, true},
		{"org.eclipse.elk.minAnimTime", "", core.IntegerOption, parents, nil, true},
		{"org.eclipse.elk.progressBar", "", core.BooleanOption, parents, nil, true},
		{"org.eclipse.elk.validateGraph", "", core.BooleanOption, parents, nil, true},
		{"org.eclipse.elk.validateOptions", "", core.BooleanOption, parents, nil, true},
		{"org.eclipse.elk.zoomToFit", "", core.BooleanOption, parents, nil, true},
	}
	if len(want) != 54 {
		t.Fatalf("compatibility option surface has %d entries, want 54", len(want))
	}

	registry := core.NewOptionRegistry()
	RegisterOptions(registry)
	seen := make(map[string]struct{}, len(want))
	for _, expected := range want {
		if _, duplicate := seen[expected.id]; duplicate {
			t.Fatalf("duplicate compatibility option %q", expected.id)
		}
		seen[expected.id] = struct{}{}
		got := registry.ByID(expected.id)
		if got == nil {
			t.Errorf("known layout option %q is not registered", expected.id)
			continue
		}
		if got.Group != expected.group || got.Kind != expected.kind || got.Targets != expected.targets ||
			got.MetadataOnly != expected.metadataOnly || !reflect.DeepEqual(got.EnumValues, expected.enums) {
			t.Errorf("option %q = {group:%q kind:%v targets:%v enums:%v metadataOnly:%v}, want %+v",
				expected.id, got.Group, got.Kind, got.Targets, got.EnumValues, got.MetadataOnly, expected)
		}
		sample := "true"
		switch got.Kind {
		case core.StringOption:
			sample = "value"
		case core.IntegerOption:
			sample = "1"
		case core.DoubleOption:
			sample = "1.5"
		case core.EnumOption:
			if len(got.EnumValues) == 0 {
				t.Errorf("enum option %q has no values", expected.id)
				continue
			}
			sample = got.EnumValues[0]
		case core.ObjectOption:
			sample = "{}"
			if expected.id == "org.eclipse.elk.layered.wrapping.cutting.cuts" ||
				expected.id == "org.eclipse.elk.layered.wrapping.validify.forbiddenIndices" {
				sample = "[]"
			}
		}
		if _, valid := got.ParseValue(sample); !valid {
			t.Errorf("known layout option %q silently rejects representative value %q", expected.id, sample)
		}
	}
}
