// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0/.
//
// SPDX-License-Identifier: EPL-2.0

package oracle

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/d2lang/elk-go/internal/builtins"
	"github.com/d2lang/elk-go/internal/core"
)

type releaseOptionClass string

const (
	optionObservableExact releaseOptionClass = "observable-exact"
	optionLayoutJSONNoOp  releaseOptionClass = "layoutjson-no-op"
	optionLayoutJSONError releaseOptionClass = "layoutjson-error"
)

type releaseOptionAudit struct {
	id       string
	class    releaseOptionClass
	evidence string
}

// official012AddedOptionAudit is the executable classification of every option
// added between the formerly embedded elkjs 0.8.2 surface and official 0.12.0.
// A case reference is checked against releaseSemanticCases below; the two JSON
// coordinate modes use exact core transfer tests, and the two generated-
// importer limitations use exact official/candidate error tests.
var official012AddedOptionAudit = []releaseOptionAudit{
	{"org.eclipse.elk.childAreaHeight", optionLayoutJSONNoOp, "case:core-topdown-programmatic-child-area"},
	{"org.eclipse.elk.childAreaWidth", optionLayoutJSONNoOp, "case:core-topdown-programmatic-child-area"},
	{"org.eclipse.elk.json.edgeCoords", optionObservableExact, "test:TestJSONEdgeCoordinateModes"},
	{"org.eclipse.elk.json.shapeCoords", optionObservableExact, "test:TestJSONShapeCoordinatesRoot"},
	{"org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cbGroupOrderStrategy", optionObservableExact, "case:layered-group-model-order"},
	{"org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cbPreferredSourceId", optionObservableExact, "case:layered-cycle-preferred-source"},
	{"org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cbPreferredTargetId", optionObservableExact, "case:layered-cycle-preferred-target"},
	{"org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cmEnforcedGroupOrders", optionLayoutJSONError, "test:TestReleaseImporterErrorParity/cmEnforcedGroupOrders"},
	{"org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cmGroupOrderStrategy", optionObservableExact, "case:layered-crossing-minimization-groups"},
	{"org.eclipse.elk.layered.considerModelOrder.groupModelOrder.componentGroupId", optionLayoutJSONNoOp, "case:layered-unused-component-group-id"},
	{"org.eclipse.elk.layered.considerModelOrder.groupModelOrder.crossingMinimizationId", optionObservableExact, "case:layered-crossing-minimization-groups"},
	{"org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cycleBreakingId", optionObservableExact, "case:layered-group-model-order"},
	{"org.eclipse.elk.layered.considerModelOrder.portModelOrder", optionObservableExact, "case:layered-port-model-order"},
	{"org.eclipse.elk.layered.crossingMinimization.inLayerPredOf", optionLayoutJSONNoOp, "case:layered-service-in-layer-constraints"},
	{"org.eclipse.elk.layered.crossingMinimization.inLayerSuccOf", optionLayoutJSONNoOp, "case:layered-service-in-layer-constraints"},
	{"org.eclipse.elk.layered.layerUnzipping.layerSplit", optionObservableExact, "case:layered-layer-split-value"},
	{"org.eclipse.elk.layered.layerUnzipping.minimizeEdgeLength", optionObservableExact, "case:layered-layer-split-minimize-edge-length"},
	{"org.eclipse.elk.layered.layerUnzipping.resetOnLongEdges", optionObservableExact, "case:layered-layer-split-long-edge-reset"},
	{"org.eclipse.elk.layered.layerUnzipping.strategy", optionObservableExact, "case:layered-layer-splitting"},
	{"org.eclipse.elk.layered.nodePlacement.networkSimplex.nodeFlexibility.recomputeNodePlacement", optionObservableExact, "case:layered-network-simplex-recompute"},
	{"org.eclipse.elk.mrtree.compaction", optionObservableExact, "case:mrtree-compaction"},
	{"org.eclipse.elk.mrtree.edgeEndTextureLength", optionObservableExact, "case:mrtree-edge-end-texture"},
	{"org.eclipse.elk.mrtree.edgeRoutingMode", optionObservableExact, "case:mrtree-edge-routing"},
	{"org.eclipse.elk.mrtree.positionConstraint", optionObservableExact, "case:mrtree-interactive-order"},
	{"org.eclipse.elk.mrtree.treeLevel", optionLayoutJSONNoOp, "case:mrtree-programmatic-tree-level"},
	{"org.eclipse.elk.radial.centerOnRoot", optionObservableExact, "case:radial-center-on-root"},
	{"org.eclipse.elk.radial.rotate", optionObservableExact, "case:radial-rotation"},
	{"org.eclipse.elk.radial.rotation.computeAdditionalWedgeSpace", optionObservableExact, "case:radial-additional-wedge-space"},
	{"org.eclipse.elk.radial.rotation.outgoingEdgeAngles", optionObservableExact, "case:radial-outgoing-edge-angles"},
	{"org.eclipse.elk.radial.rotation.targetAngle", optionObservableExact, "case:radial-rotation"},
	{"org.eclipse.elk.rectpacking.inNewRow", optionObservableExact, "case:rectpacking-forced-new-row"},
	{"org.eclipse.elk.rectpacking.orderBySize", optionObservableExact, "case:rectpacking-order-by-size"},
	{"org.eclipse.elk.rectpacking.packing.compaction.iterations", optionObservableExact, "case:rectpacking-compaction-iterations"},
	{"org.eclipse.elk.rectpacking.packing.compaction.rowHeightReevaluation", optionObservableExact, "case:rectpacking-row-height-reevaluation"},
	{"org.eclipse.elk.rectpacking.packing.strategy", optionObservableExact, "case:rectpacking-packing-strategy"},
	{"org.eclipse.elk.rectpacking.trybox", optionObservableExact, "case:rectpacking-try-box"},
	{"org.eclipse.elk.rectpacking.whiteSpaceElimination.strategy", optionObservableExact, "case:rectpacking-whitespace-strategy"},
	{"org.eclipse.elk.rectpacking.widthApproximation.lastPlaceShift", optionObservableExact, "case:rectpacking-last-place-shift"},
	{"org.eclipse.elk.rectpacking.widthApproximation.optimizationGoal", optionObservableExact, "case:rectpacking-optimization-goal"},
	{"org.eclipse.elk.rectpacking.widthApproximation.strategy", optionObservableExact, "case:rectpacking-width-strategy"},
	{"org.eclipse.elk.rectpacking.widthApproximation.targetWidth", optionObservableExact, "case:rectpacking-width-strategy"},
	{"org.eclipse.elk.softwrappingFuzziness", optionLayoutJSONNoOp, "case:core-softwrapping-fuzziness-json-no-op"},
	{"org.eclipse.elk.topdown.hierarchicalNodeAspectRatio", optionObservableExact, "case:core-topdown-hierarchical-size"},
	{"org.eclipse.elk.topdown.hierarchicalNodeWidth", optionObservableExact, "case:core-topdown-hierarchical-size"},
	{"org.eclipse.elk.topdown.nodeType", optionObservableExact, "case:core-topdown-layout"},
	{"org.eclipse.elk.topdown.scaleCap", optionObservableExact, "case:core-topdown-scale-cap"},
	{"org.eclipse.elk.topdown.scaleFactor", optionLayoutJSONNoOp, "case:core-topdown-programmatic-scale-factor"},
	{"org.eclipse.elk.topdown.sizeApproximator", optionLayoutJSONError, "test:TestReleaseImporterErrorParity/topdownSizeApproximator"},
	{"org.eclipse.elk.topdown.sizeCategories", optionLayoutJSONNoOp, "case:core-topdown-size-categories-json-no-op"},
	{"org.eclipse.elk.topdown.sizeCategoriesHierarchicalNodeWeight", optionLayoutJSONNoOp, "case:core-topdown-hierarchical-weight-json-no-op"},
	{"org.eclipse.elk.topdownLayout", optionObservableExact, "case:core-topdown-layout"},
}

var official012AddedOptionIDs = []string{
	"org.eclipse.elk.childAreaHeight",
	"org.eclipse.elk.childAreaWidth",
	"org.eclipse.elk.json.edgeCoords",
	"org.eclipse.elk.json.shapeCoords",
	"org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cbGroupOrderStrategy",
	"org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cbPreferredSourceId",
	"org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cbPreferredTargetId",
	"org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cmEnforcedGroupOrders",
	"org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cmGroupOrderStrategy",
	"org.eclipse.elk.layered.considerModelOrder.groupModelOrder.componentGroupId",
	"org.eclipse.elk.layered.considerModelOrder.groupModelOrder.crossingMinimizationId",
	"org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cycleBreakingId",
	"org.eclipse.elk.layered.considerModelOrder.portModelOrder",
	"org.eclipse.elk.layered.crossingMinimization.inLayerPredOf",
	"org.eclipse.elk.layered.crossingMinimization.inLayerSuccOf",
	"org.eclipse.elk.layered.layerUnzipping.layerSplit",
	"org.eclipse.elk.layered.layerUnzipping.minimizeEdgeLength",
	"org.eclipse.elk.layered.layerUnzipping.resetOnLongEdges",
	"org.eclipse.elk.layered.layerUnzipping.strategy",
	"org.eclipse.elk.layered.nodePlacement.networkSimplex.nodeFlexibility.recomputeNodePlacement",
	"org.eclipse.elk.mrtree.compaction",
	"org.eclipse.elk.mrtree.edgeEndTextureLength",
	"org.eclipse.elk.mrtree.edgeRoutingMode",
	"org.eclipse.elk.mrtree.positionConstraint",
	"org.eclipse.elk.mrtree.treeLevel",
	"org.eclipse.elk.radial.centerOnRoot",
	"org.eclipse.elk.radial.rotate",
	"org.eclipse.elk.radial.rotation.computeAdditionalWedgeSpace",
	"org.eclipse.elk.radial.rotation.outgoingEdgeAngles",
	"org.eclipse.elk.radial.rotation.targetAngle",
	"org.eclipse.elk.rectpacking.inNewRow",
	"org.eclipse.elk.rectpacking.orderBySize",
	"org.eclipse.elk.rectpacking.packing.compaction.iterations",
	"org.eclipse.elk.rectpacking.packing.compaction.rowHeightReevaluation",
	"org.eclipse.elk.rectpacking.packing.strategy",
	"org.eclipse.elk.rectpacking.trybox",
	"org.eclipse.elk.rectpacking.whiteSpaceElimination.strategy",
	"org.eclipse.elk.rectpacking.widthApproximation.lastPlaceShift",
	"org.eclipse.elk.rectpacking.widthApproximation.optimizationGoal",
	"org.eclipse.elk.rectpacking.widthApproximation.strategy",
	"org.eclipse.elk.rectpacking.widthApproximation.targetWidth",
	"org.eclipse.elk.softwrappingFuzziness",
	"org.eclipse.elk.topdown.hierarchicalNodeAspectRatio",
	"org.eclipse.elk.topdown.hierarchicalNodeWidth",
	"org.eclipse.elk.topdown.nodeType",
	"org.eclipse.elk.topdown.scaleCap",
	"org.eclipse.elk.topdown.scaleFactor",
	"org.eclipse.elk.topdown.sizeApproximator",
	"org.eclipse.elk.topdown.sizeCategories",
	"org.eclipse.elk.topdown.sizeCategoriesHierarchicalNodeWeight",
	"org.eclipse.elk.topdownLayout",
}

var official012RemovedOptionIDs = []string{
	"org.eclipse.elk.disco.componentCompaction.componentLayoutAlgorithm",
	"org.eclipse.elk.disco.componentCompaction.strategy",
	"org.eclipse.elk.disco.debug.discoGraph",
	"org.eclipse.elk.disco.debug.discoPolys",
	"org.eclipse.elk.polyomino.fill",
	"org.eclipse.elk.polyomino.highLevelSort",
	"org.eclipse.elk.polyomino.lowLevelSort",
	"org.eclipse.elk.polyomino.traversalStrategy",
	"org.eclipse.elk.rectpacking.expandToAspectRatio",
	"org.eclipse.elk.rectpacking.lastPlaceShift",
	"org.eclipse.elk.rectpacking.onlyFirstIteration",
	"org.eclipse.elk.rectpacking.optimizationGoal",
	"org.eclipse.elk.rectpacking.rowCompaction",
	"org.eclipse.elk.rectpacking.targetWidth",
}

func TestReleaseOptionAuditCoverage(t *testing.T) {
	if len(official012AddedOptionAudit) != 51 {
		t.Fatalf("added-option audit has %d entries, want 51", len(official012AddedOptionAudit))
	}
	if len(official012RemovedOptionIDs) != 14 {
		t.Fatalf("removed-option audit has %d entries, want 14", len(official012RemovedOptionIDs))
	}

	semanticCases := make(map[string]releaseSemanticCase)
	for _, testCase := range releaseSemanticCases() {
		semanticCases[testCase.name] = testCase
	}
	allowedStandaloneTests := map[string]struct{}{
		"TestJSONEdgeCoordinateModes":                            {},
		"TestJSONShapeCoordinatesRoot":                           {},
		"TestReleaseImporterErrorParity/cmEnforcedGroupOrders":   {},
		"TestReleaseImporterErrorParity/topdownSizeApproximator": {},
	}

	registry := core.NewOptionRegistry()
	builtins.RegisterOptions(registry)
	gotAdded := make([]string, 0, len(official012AddedOptionAudit))
	seen := make(map[string]struct{}, len(official012AddedOptionAudit))
	for _, audit := range official012AddedOptionAudit {
		if _, duplicate := seen[audit.id]; duplicate {
			t.Fatalf("added-option audit repeats %q", audit.id)
		}
		seen[audit.id] = struct{}{}
		gotAdded = append(gotAdded, audit.id)
		if registry.ByID(audit.id) == nil {
			t.Errorf("0.12 added option %q is not registered", audit.id)
		}

		switch {
		case strings.HasPrefix(audit.evidence, "case:"):
			name := strings.TrimPrefix(audit.evidence, "case:")
			testCase, ok := semanticCases[name]
			if !ok {
				t.Errorf("option %q references unknown semantic case %q", audit.id, name)
				continue
			}
			switch audit.class {
			case optionObservableExact:
				if !testCase.observable {
					t.Errorf("observable option %q references non-observable case %q", audit.id, name)
				}
			case optionLayoutJSONNoOp:
				if !testCase.oracleNoOp {
					t.Errorf("LayoutJSON no-op %q references non-no-op case %q", audit.id, name)
				}
			default:
				t.Errorf("option %q has class %q but case evidence", audit.id, audit.class)
			}
		case strings.HasPrefix(audit.evidence, "test:"):
			name := strings.TrimPrefix(audit.evidence, "test:")
			if _, ok := allowedStandaloneTests[name]; !ok {
				t.Errorf("option %q references unrecognized standalone test %q", audit.id, name)
			}
			if audit.class != optionObservableExact && audit.class != optionLayoutJSONError {
				t.Errorf("option %q has class %q but standalone-test evidence", audit.id, audit.class)
			}
		default:
			t.Errorf("option %q has invalid evidence %q", audit.id, audit.evidence)
		}
	}
	if !reflect.DeepEqual(gotAdded, official012AddedOptionIDs) {
		t.Errorf("added-option audit IDs differ from pinned 0.8.2-to-0.12.0 delta\ngot:  %v\nwant: %v", gotAdded, official012AddedOptionIDs)
	}

	removed := append([]string(nil), official012RemovedOptionIDs...)
	sort.Strings(removed)
	if !reflect.DeepEqual(removed, official012RemovedOptionIDs) {
		t.Error("removed-option inventory must remain sorted")
	}
	for _, id := range official012RemovedOptionIDs {
		if registry.ByID(id) != nil {
			t.Errorf("0.12 removed option %q remains registered", id)
		}
	}
}
