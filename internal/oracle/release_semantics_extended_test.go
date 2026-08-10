// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package oracle

// extendedReleaseSemanticCases holds the adversarial option-surface probes
// that complement the algorithm-centered release corpus.
func extendedReleaseSemanticCases() []releaseSemanticCase {
	var cases []releaseSemanticCase
	cases = append(cases, coreExtendedSemanticCases()...)
	cases = append(cases, mrTreeExtendedSemanticCases()...)
	cases = append(cases, radialExtendedSemanticCases()...)
	cases = append(cases, rectPackingExtendedSemanticCases()...)
	cases = append(cases, layeredExtendedSemanticCases()...)
	return cases
}

func coreExtendedSemanticCases() []releaseSemanticCase {
	flat := semanticGraph("layered", semanticOptions("elk.direction", "RIGHT"), []map[string]any{
		semanticNode("a", 30, 30, nil),
		semanticNode("b", 30, 30, nil),
		semanticNode("c", 30, 30, nil),
	}, []map[string]any{
		semanticEdge("ac", "a", "c", nil),
		semanticEdge("bc", "b", "c", nil),
	})

	container := semanticNode("box", 80, 50, semanticOptions(
		"elk.algorithm", "mrtree",
		"elk.direction", "RIGHT",
		"elk.topdownLayout", "true",
		"elk.topdown.nodeType", "HIERARCHICAL_NODE",
		"elk.nodeSize.fixedGraphSize", "true",
		"elk.topdown.hierarchicalNodeWidth", "150",
		"elk.topdown.hierarchicalNodeAspectRatio", "1.414",
		"elk.topdown.scaleCap", "1",
	))
	container["children"] = []map[string]any{
		semanticNode("inner-a", 60, 30, semanticOptions(
			"elk.topdownLayout", "true", "elk.topdown.nodeType", "HIERARCHICAL_NODE")),
		semanticNode("inner-b", 60, 30, semanticOptions(
			"elk.topdownLayout", "true", "elk.topdown.nodeType", "HIERARCHICAL_NODE")),
	}
	container["edges"] = []map[string]any{semanticEdge("inner-edge", "inner-a", "inner-b", nil)}
	topdown := semanticGraph("layered", semanticOptions(
		"elk.direction", "RIGHT",
		"elk.topdownLayout", "true",
		"elk.topdown.nodeType", "ROOT_NODE",
	), []map[string]any{
		container,
		semanticNode("peer", 80, 50, semanticOptions(
			"elk.topdownLayout", "true", "elk.topdown.nodeType", "HIERARCHICAL_NODE")),
	}, []map[string]any{semanticEdge("outer-edge", "box", "peer", nil)})

	nestedChild := semanticNode("category-nested", 30, 30, semanticOptions(
		"elk.algorithm", "mrtree",
		"elk.topdownLayout", "true",
		"elk.topdown.nodeType", "HIERARCHICAL_NODE",
		"elk.nodeSize.fixedGraphSize", "true",
		"elk.topdown.hierarchicalNodeWidth", "40",
		"elk.topdown.hierarchicalNodeAspectRatio", "1",
	))
	nestedChild["children"] = []map[string]any{
		semanticNode("category-grandchild", 15, 15, semanticOptions(
			"elk.topdownLayout", "true", "elk.topdown.nodeType", "HIERARCHICAL_NODE")),
	}
	categoryContainer := semanticNode("category-box", 40, 40, semanticOptions(
		"elk.algorithm", "mrtree",
		"elk.topdownLayout", "true",
		"elk.topdown.nodeType", "HIERARCHICAL_NODE",
		"elk.nodeSize.fixedGraphSize", "true",
		"elk.topdown.hierarchicalNodeWidth", "100",
		"elk.topdown.hierarchicalNodeAspectRatio", "1",
		"elk.topdown.sizeCategories", "3",
		"elk.topdown.sizeCategoriesHierarchicalNodeWeight", "4",
	))
	categoryContainer["children"] = []map[string]any{
		nestedChild,
		semanticNode("category-simple", 20, 20, semanticOptions(
			"elk.topdownLayout", "true", "elk.topdown.nodeType", "HIERARCHICAL_NODE")),
	}
	categoryContainer["edges"] = []map[string]any{semanticEdge("category-edge", "category-nested", "category-simple", nil)}
	categorized := semanticGraph("layered", semanticOptions(
		"elk.direction", "RIGHT",
		"elk.topdownLayout", "true",
		"elk.topdown.nodeType", "ROOT_NODE",
	), []map[string]any{
		categoryContainer,
		semanticNode("category-peer", 40, 40, semanticOptions(
			"elk.topdownLayout", "true", "elk.topdown.nodeType", "HIERARCHICAL_NODE")),
	}, []map[string]any{semanticEdge("category-outer", "category-box", "category-peer", nil)})

	scaledCompound := semanticNode("scaled", 60, 40, semanticOptions(
		"elk.algorithm", "fixed",
		"elk.position", "(20,30)",
		"elk.scaleFactor", "2",
	))
	scaledCompound["labels"] = []map[string]any{{
		"id": "scaled-label", "text": "scaled", "x": 3, "y": 4, "width": 18, "height": 9,
	}}
	scaledCompound["ports"] = []map[string]any{{
		"id": "scaled-port", "x": 50, "y": 12, "width": 8, "height": 10,
		"layoutOptions": semanticOptions("elk.port.anchor", "(2,3)"),
		"labels": []map[string]any{{
			"id": "scaled-port-label", "text": "port", "x": 1, "y": 2, "width": 12, "height": 7,
		}},
	}}
	scaledCompound["children"] = []map[string]any{
		semanticNode("scaled-inner", 20, 15, semanticOptions("elk.position", "(5,6)")),
	}
	scaled := semanticGraph("fixed", semanticOptions(
		"elk.padding", "[top=0,left=0,bottom=0,right=0]",
	), []map[string]any{
		scaledCompound,
		semanticNode("scaled-peer", 40, 30, semanticOptions("elk.position", "(200,10)")),
	}, nil)

	return []releaseSemanticCase{
		{
			name: "core-softwrapping-fuzziness-json-no-op", claim: "softwrappingFuzziness is metadata consumed by text-wrapping services, not elkjs LayoutJSON",
			upstream: "ELK Core.melk softwrappingFuzziness; no consumer in the elkjs 0.12 layout entry point", oracleNoOp: true,
			variants: []releaseSemanticVariant{
				semanticVariant("zero", flat, semanticRootOption("elk.softwrappingFuzziness", "0")),
				semanticVariant("one", flat, semanticRootOption("elk.softwrappingFuzziness", "1")),
			},
		},
		{
			name: "core-topdown-layout", claim: "the recursive engine lays out compound graphs from the root downward",
			upstream: "ELK 2ec99f2b5 topdown layout and subsequent 0.12 fixes", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("bottom-up", topdown,
					semanticElementOption("root", "elk.topdownLayout", "false"),
					semanticElementOption("box", "elk.topdownLayout", "false"),
					semanticElementOption("peer", "elk.topdownLayout", "false"),
					semanticElementOption("inner-a", "elk.topdownLayout", "false"),
					semanticElementOption("inner-b", "elk.topdownLayout", "false")),
				semanticVariant("top-down", topdown),
			},
		},
		{
			name: "core-topdown-hierarchical-size", claim: "hierarchical node width and aspect ratio determine the size allocated before child layout",
			upstream: "ELK 2ec99f2b5 topdown hierarchical node sizing", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("narrow-square", topdown,
					semanticElementOption("box", "elk.topdown.hierarchicalNodeWidth", "80"),
					semanticElementOption("box", "elk.topdown.hierarchicalNodeAspectRatio", "1")),
				semanticVariant("wide-landscape", topdown,
					semanticElementOption("box", "elk.topdown.hierarchicalNodeWidth", "220"),
					semanticElementOption("box", "elk.topdown.hierarchicalNodeAspectRatio", "3")),
			},
		},
		{
			name: "core-topdown-scale-cap", claim: "topdown scaleCap bounds the computed inner-layout scale",
			upstream: "ELK 2ec99f2b5 topdown scale cap", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("half", topdown, semanticElementOption("box", "elk.topdown.scaleCap", "0.5")),
				semanticVariant("one", topdown, semanticElementOption("box", "elk.topdown.scaleCap", "1")),
			},
		},
		{
			name: "core-topdown-size-categories-json-no-op", claim: "sizeCategories is only consumed by an ITopdownSizeApproximator, which elkjs JSON cannot instantiate",
			upstream: "ELK TopdownSizeApproximatorUtil sizeCategories and Json importer ElkReflect limitation", oracleNoOp: true,
			variants: []releaseSemanticVariant{
				semanticVariant("one", categorized, semanticElementOption("category-box", "elk.topdown.sizeCategories", "1")),
				semanticVariant("three", categorized, semanticElementOption("category-box", "elk.topdown.sizeCategories", "3")),
			},
		},
		{
			name: "core-topdown-hierarchical-weight-json-no-op", claim: "sizeCategoriesHierarchicalNodeWeight is only consumed by an ITopdownSizeApproximator, which elkjs JSON cannot instantiate",
			upstream: "ELK TopdownSizeApproximatorUtil hierarchical-node weight and Json importer ElkReflect limitation", oracleNoOp: true,
			variants: []releaseSemanticVariant{
				semanticVariant("weight-one", categorized, semanticElementOption("category-box", "elk.topdown.sizeCategoriesHierarchicalNodeWeight", "1")),
				semanticVariant("weight-twenty", categorized, semanticElementOption("category-box", "elk.topdown.sizeCategoriesHierarchicalNodeWeight", "20")),
			},
		},
		{
			name: "core-topdown-programmatic-scale-factor", claim: "topdown scaleFactor is computed output state; an input value is overwritten before LayoutJSON export",
			upstream: "ELK Core.melk programmatic topdown.scaleFactor and RecursiveGraphLayoutEngine", oracleNoOp: true,
			variants: []releaseSemanticVariant{
				semanticVariant("input-small", topdown, semanticElementOption("box", "elk.topdown.scaleFactor", "0.1")),
				semanticVariant("input-large", topdown, semanticElementOption("box", "elk.topdown.scaleFactor", "10")),
			},
		},
		{
			name: "core-topdown-programmatic-child-area", claim: "childAreaWidth and childAreaHeight are computed provider state; input values do not affect elkjs LayoutJSON",
			upstream: "ELK Core.melk programmatic childAreaWidth/childAreaHeight and RecursiveGraphLayoutEngine", oracleNoOp: true,
			variants: []releaseSemanticVariant{
				semanticVariant("small-area", topdown,
					semanticElementOption("box", "elk.childAreaWidth", "60"),
					semanticElementOption("box", "elk.childAreaHeight", "30")),
				semanticVariant("large-area", topdown,
					semanticElementOption("box", "elk.childAreaWidth", "300"),
					semanticElementOption("box", "elk.childAreaHeight", "180")),
			},
		},
		{
			name: "core-configured-node-scaling", claim: "generic scaleFactor scales a compound node, its labels, ports, port labels, and port anchors before its parent is laid out",
			upstream: "ELK ElkUtil.applyConfiguredNodeScaling and RecursiveGraphLayoutEngine", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("one", scaled, semanticElementOption("scaled", "elk.scaleFactor", "1")),
				semanticVariant("two", scaled),
			},
		},
	}
}

func mrTreeExtendedSemanticCases() []releaseSemanticCase {
	tree := semanticGraph("mrtree", semanticOptions(
		"elk.direction", "RIGHT",
		"elk.spacing.nodeNode", "28",
		"elk.mrtree.edgeRoutingMode", "AVOID_OVERLAP",
	), []map[string]any{
		semanticNode("root-node", 36, 30, nil),
		semanticNode("a", 70, 22, nil),
		semanticNode("b", 24, 55, nil),
		semanticNode("c", 55, 18, nil),
		semanticNode("d", 30, 44, nil),
		semanticNode("e", 48, 26, nil),
		semanticNode("f", 25, 33, nil),
	}, []map[string]any{
		semanticEdge("ra", "root-node", "a", nil),
		semanticEdge("rb", "root-node", "b", nil),
		semanticEdge("ac", "a", "c", nil),
		semanticEdge("ad", "a", "d", nil),
		semanticEdge("ce", "c", "e", nil),
		semanticEdge("bf", "b", "f", nil),
	})

	return []releaseSemanticCase{
		{
			name: "mrtree-compaction", claim: "MrTree can compact sparse levels after tree placement",
			upstream: "ELK 10f7b345d Interactive MrTree compaction processor", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("off", tree, semanticRootOption("elk.mrtree.compaction", "false")),
				semanticVariant("on", tree, semanticRootOption("elk.mrtree.compaction", "true")),
			},
		},
		{
			name: "mrtree-edge-end-texture", claim: "MrTree edge-end texture length changes avoidance-routing bendpoint geometry",
			upstream: "ELK 10f7b345d Interactive MrTree edge-end texture option", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("zero", tree, semanticRootOption("elk.mrtree.edgeEndTextureLength", "0")),
				semanticVariant("thirty", tree, semanticRootOption("elk.mrtree.edgeEndTextureLength", "30")),
			},
		},
		{
			name: "mrtree-programmatic-tree-level", claim: "treeLevel is recomputed by MrTree; caller-supplied values do not affect elkjs LayoutJSON",
			upstream: "ELK MrTree.melk programmatic treeLevel and LevelProcessor", oracleNoOp: true,
			variants: []releaseSemanticVariant{
				semanticVariant("input-zero", tree, semanticElementOption("c", "elk.mrtree.treeLevel", "0")),
				semanticVariant("input-nine", tree, semanticElementOption("c", "elk.mrtree.treeLevel", "9")),
			},
		},
	}
}

func radialExtendedSemanticCases() []releaseSemanticCase {
	graph := semanticGraph("radial", semanticOptions(
		"elk.radial.radius", "45",
		"elk.radial.rotate", "true",
		"elk.radial.rotation.targetAngle", "-0.75",
	), []map[string]any{
		semanticNode("root-node", 35, 20, nil),
		semanticNode("wide", 70, 25, nil),
		semanticNode("tall", 15, 45, nil),
		semanticNode("small", 30, 15, nil),
		semanticNode("leaf", 22, 28, nil),
	}, []map[string]any{
		semanticEdge("wide-edge", "root-node", "wide", nil),
		semanticEdge("tall-edge", "root-node", "tall", nil),
		semanticEdge("small-edge", "root-node", "small", nil),
		semanticEdge("leaf-edge", "wide", "leaf", nil),
	})

	compound := semanticNode("compound", 90, 70, semanticOptions(
		"elk.algorithm", "radial",
		"elk.radial.rotate", "true",
		"elk.topdownLayout", "true",
		"elk.topdown.nodeType", "HIERARCHICAL_NODE",
		"elk.nodeSize.fixedGraphSize", "true",
		"elk.topdown.hierarchicalNodeWidth", "120",
		"elk.topdown.hierarchicalNodeAspectRatio", "1.4",
	))
	compound["children"] = []map[string]any{
		semanticNode("inner-root", 25, 25, semanticOptions("elk.topdownLayout", "true", "elk.topdown.nodeType", "HIERARCHICAL_NODE")),
		semanticNode("inner-a", 35, 20, semanticOptions("elk.topdownLayout", "true", "elk.topdown.nodeType", "HIERARCHICAL_NODE")),
		semanticNode("inner-b", 20, 40, semanticOptions("elk.topdownLayout", "true", "elk.topdown.nodeType", "HIERARCHICAL_NODE")),
	}
	compound["edges"] = []map[string]any{
		semanticEdge("inner-a-edge", "inner-root", "inner-a", nil),
		semanticEdge("inner-b-edge", "inner-root", "inner-b", nil),
	}
	nested := semanticGraph("radial", semanticOptions(
		"elk.radial.radius", "220",
		"elk.topdownLayout", "true",
		"elk.topdown.nodeType", "ROOT_NODE",
	), []map[string]any{
		semanticNode("outer-root", 35, 35, semanticOptions("elk.topdownLayout", "true", "elk.topdown.nodeType", "HIERARCHICAL_NODE")),
		compound,
		semanticNode("outer-leaf", 55, 25, semanticOptions("elk.topdownLayout", "true", "elk.topdown.nodeType", "HIERARCHICAL_NODE")),
	}, []map[string]any{
		semanticEdge("to-compound", "outer-root", "compound", nil),
		semanticEdge("to-leaf", "outer-root", "outer-leaf", nil),
	})

	return []releaseSemanticCase{
		{
			name: "radial-additional-wedge-space", claim: "Radial rotation can reserve additional wedge space for node dimensions",
			upstream: "ELK 83898117b Radial rotation extensions", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("off", graph, semanticRootOption("elk.radial.rotation.computeAdditionalWedgeSpace", "false")),
				semanticVariant("on", graph, semanticRootOption("elk.radial.rotation.computeAdditionalWedgeSpace", "true")),
			},
		},
		{
			name: "radial-outgoing-edge-angles", claim: "Radial rotation can derive orientation from outgoing edge angles",
			upstream: "ELK 83898117b Radial rotation extensions", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("off", nested, semanticRootOption("elk.radial.rotation.outgoingEdgeAngles", "false")),
				semanticVariant("on", nested, semanticRootOption("elk.radial.rotation.outgoingEdgeAngles", "true")),
			},
		},
	}
}
func rectPackingExtendedSemanticCases() []releaseSemanticCase {
	base := semanticGraph("rectpacking", semanticOptions(
		"elk.padding", "[top=5,left=7,bottom=11,right=13]",
		"elk.spacing.nodeNode", "7",
		"elk.aspectRatio", "1.45",
	), []map[string]any{
		semanticNode("a", 23, 17, nil),
		semanticNode("b", 31, 11, nil),
		semanticNode("c", 13, 29, nil),
		semanticNode("d", 41, 7, nil),
		semanticNode("e", 17, 38, nil),
		semanticNode("f", 36, 19, nil),
		semanticNode("g", 21, 45, nil),
		semanticNode("h", 52, 12, nil),
	}, nil)

	rowHeight := semanticGraph("rectpacking", semanticOptions(
		"elk.rectpacking.widthApproximation.strategy", "TARGET_WIDTH",
		"elk.rectpacking.widthApproximation.targetWidth", "101",
		"elk.spacing.nodeNode", "7",
	), []map[string]any{
		semanticNode("n0", 69, 49, nil), semanticNode("n1", 31, 39, nil),
		semanticNode("n2", 40, 42, nil), semanticNode("n3", 73, 63, nil),
		semanticNode("n4", 40, 49, nil), semanticNode("n5", 46, 16, nil),
		semanticNode("n6", 55, 64, nil), semanticNode("n7", 68, 49, nil),
	}, nil)

	tryBox := semanticGraph("rectpacking", nil, []map[string]any{
		semanticNode("a", 20, 30, nil), semanticNode("b", 25, 30, nil),
		semanticNode("c", 30, 30, nil), semanticNode("d", 35, 30, nil),
	}, nil)

	return []releaseSemanticCase{
		{
			name: "rectpacking-forced-new-row", claim: "a node can force a new rectangle-packing row",
			upstream: "ELK 20a09e44c rectpacking restructure, inNewRow", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("continuous", base,
					semanticRootOption("elk.rectpacking.widthApproximation.strategy", "TARGET_WIDTH"),
					semanticRootOption("elk.rectpacking.widthApproximation.targetWidth", "120"),
					semanticElementOption("b", "elk.rectpacking.inNewRow", "false")),
				semanticVariant("new-row", base,
					semanticRootOption("elk.rectpacking.widthApproximation.strategy", "TARGET_WIDTH"),
					semanticRootOption("elk.rectpacking.widthApproximation.targetWidth", "120"),
					semanticElementOption("b", "elk.rectpacking.inNewRow", "true")),
			},
		},
		{
			name: "rectpacking-compaction-iterations", claim: "rectangle packing can run repeated target-width compaction iterations",
			upstream: "ELK 20a09e44c rectpacking compaction iterations", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("one", base,
					semanticRootOption("elk.rectpacking.packing.strategy", "COMPACTION"),
					semanticRootOption("elk.rectpacking.packing.compaction.iterations", "1")),
				semanticVariant("four", base,
					semanticRootOption("elk.rectpacking.packing.strategy", "COMPACTION"),
					semanticRootOption("elk.rectpacking.packing.compaction.iterations", "4")),
			},
		},
		{
			name: "rectpacking-row-height-reevaluation", claim: "compaction can reevaluate row heights when moving blocks",
			upstream: "ELK 20a09e44c rectpacking row-height reevaluation", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("off", rowHeight, semanticRootOption("elk.rectpacking.packing.compaction.rowHeightReevaluation", "false")),
				semanticVariant("on", rowHeight, semanticRootOption("elk.rectpacking.packing.compaction.rowHeightReevaluation", "true")),
			},
		},
		{
			name: "rectpacking-try-box", claim: "trybox can select the core Box provider for compatible unconnected inputs",
			upstream: "ELK 20a09e44c rectpacking trybox path", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("rectpacking", tryBox, semanticRootOption("elk.rectpacking.trybox", "false")),
				semanticVariant("try-box", tryBox, semanticRootOption("elk.rectpacking.trybox", "true")),
			},
		},
		{
			name: "rectpacking-last-place-shift", claim: "GREEDY width approximation can include or omit the final placement shift",
			upstream: "ELK 20a09e44c rectpacking renamed lastPlaceShift", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("off", base,
					semanticRootOption("elk.rectpacking.widthApproximation.strategy", "GREEDY"),
					semanticRootOption("elk.rectpacking.widthApproximation.lastPlaceShift", "false")),
				semanticVariant("on", base,
					semanticRootOption("elk.rectpacking.widthApproximation.strategy", "GREEDY"),
					semanticRootOption("elk.rectpacking.widthApproximation.lastPlaceShift", "true")),
			},
		},
		{
			name: "rectpacking-optimization-goal", claim: "GREEDY width approximation can optimize area or scale",
			upstream: "ELK 20a09e44c rectpacking renamed optimizationGoal", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("area", base,
					semanticRootOption("elk.rectpacking.widthApproximation.strategy", "GREEDY"),
					semanticRootOption("elk.rectpacking.widthApproximation.optimizationGoal", "AREA_DRIVEN")),
				semanticVariant("scale", base,
					semanticRootOption("elk.rectpacking.widthApproximation.strategy", "GREEDY"),
					semanticRootOption("elk.rectpacking.widthApproximation.optimizationGoal", "MAX_SCALE_DRIVEN")),
			},
		},
	}
}
func layeredExtendedSemanticCases() []releaseSemanticCase {
	inLayer := semanticGraph("layered", semanticOptions(
		"elk.direction", "RIGHT",
		"elk.layered.crossingMinimization.strategy", "LAYER_SWEEP",
	), []map[string]any{
		semanticNode("a", 30, 30, nil), semanticNode("b", 30, 30, nil),
		semanticNode("c", 30, 30, nil), semanticNode("d", 30, 30, nil),
	}, []map[string]any{
		semanticEdge("ac", "a", "c", nil), semanticEdge("bc", "b", "c", nil),
		semanticEdge("ad", "a", "d", nil), semanticEdge("bd", "b", "d", nil),
	})

	unzip := semanticGraph("layered", semanticOptions(
		"elk.direction", "RIGHT",
		"elk.layered.considerModelOrder.strategy", "PREFER_EDGES",
		"elk.layered.layerUnzipping.strategy", "ALTERNATING",
	), []map[string]any{
		semanticNode("source", 30, 30, nil),
		semanticNode("middle-1", 30, 30, semanticOptions(
			"elk.layered.layerUnzipping.layerSplit", "2",
			"elk.layered.layerUnzipping.resetOnLongEdges", "true",
			"elk.layered.layerUnzipping.minimizeEdgeLength", "false")),
		semanticNode("middle-2", 30, 30, nil),
		semanticNode("middle-3", 30, 30, nil),
		semanticNode("target", 30, 30, nil),
	}, []map[string]any{
		semanticEdge("s1", "source", "middle-1", nil), semanticEdge("s2", "source", "middle-2", nil),
		semanticEdge("s3", "source", "middle-3", nil), semanticEdge("t1", "middle-1", "target", nil),
		semanticEdge("t2", "middle-2", "target", nil), semanticEdge("t3", "middle-3", "target", nil),
		semanticEdge("long", "source", "target", nil),
	})

	resetNodes := []map[string]any{
		semanticNode("s", 46, 32, nil), semanticNode("a", 22, 27, nil),
		semanticNode("b", 21, 42, nil), semanticNode("c", 45, 49, nil),
		semanticNode("d", 29, 43, nil), semanticNode("e", 23, 24, nil),
		semanticNode("f", 29, 48, nil), semanticNode("t", 21, 40, nil),
	}
	for _, node := range resetNodes {
		node["layoutOptions"] = semanticOptions(
			"elk.layered.layerUnzipping.layerSplit", "2",
			"elk.layered.layerUnzipping.resetOnLongEdges", "true")
	}
	resetGraph := semanticGraph("layered", semanticOptions(
		"elk.direction", "RIGHT",
		"elk.layered.layerUnzipping.strategy", "ALTERNATING",
		"elk.layered.considerModelOrder.strategy", "PREFER_EDGES",
	), resetNodes, []map[string]any{
		semanticEdge("e0", "s", "a", nil), semanticEdge("e1", "s", "b", nil),
		semanticEdge("e2", "s", "c", nil), semanticEdge("e3", "s", "e", nil),
		semanticEdge("e4", "s", "f", nil), semanticEdge("e5", "a", "d", nil),
		semanticEdge("e6", "a", "f", nil), semanticEdge("e7", "a", "t", nil),
		semanticEdge("e8", "c", "t", nil), semanticEdge("e9", "d", "e", nil),
		semanticEdge("e10", "d", "f", nil), semanticEdge("e11", "d", "t", nil),
		semanticEdge("e12", "e", "t", nil), semanticEdge("long", "s", "t", nil),
	})
	setReset := func(value string) semanticEdit {
		return func(graph map[string]any) {
			for _, id := range []string{"s", "a", "b", "c", "d", "e", "f", "t"} {
				semanticElementOption(id, "elk.layered.layerUnzipping.resetOnLongEdges", value)(graph)
			}
		}
	}

	flexA := semanticNode("a", 50, 30, semanticOptions("elk.portConstraints", "FIXED_ORDER"))
	flexA["ports"] = []map[string]any{
		{"id": "a1", "width": 4.0, "height": 4.0, "layoutOptions": semanticOptions("elk.port.side", "EAST")},
		{"id": "a2", "width": 4.0, "height": 4.0, "layoutOptions": semanticOptions("elk.port.side", "EAST")},
		{"id": "a3", "width": 4.0, "height": 4.0, "layoutOptions": semanticOptions("elk.port.side", "EAST")},
	}
	flexB := semanticNode("b", 45, 80, semanticOptions("elk.portConstraints", "FIXED_ORDER"))
	flexB["ports"] = []map[string]any{
		{"id": "b1", "width": 4.0, "height": 4.0, "layoutOptions": semanticOptions("elk.port.side", "WEST")},
		{"id": "b2", "width": 4.0, "height": 4.0, "layoutOptions": semanticOptions("elk.port.side", "WEST")},
		{"id": "b3", "width": 4.0, "height": 4.0, "layoutOptions": semanticOptions("elk.port.side", "WEST")},
	}
	flex := semanticGraph("layered", semanticOptions(
		"elk.layered.nodePlacement.strategy", "NETWORK_SIMPLEX",
		"elk.layered.nodePlacement.networkSimplex.nodeFlexibility.default", "NODE_SIZE",
		"elk.edgeRouting", "ORTHOGONAL",
		"elk.randomSeed", "17",
	), []map[string]any{flexA, flexB}, []map[string]any{
		semanticEdge("e1", "a1", "b3", nil),
		semanticEdge("e2", "a2", "b2", nil),
		semanticEdge("e3", "a3", "b1", nil),
	})

	cbGroup := semanticGraph("layered", semanticOptions(
		"elk.direction", "RIGHT",
		"elk.layered.cycleBreaking.strategy", "DFS_NODE_ORDER",
		"elk.layered.considerModelOrder.strategy", "NODES_AND_EDGES",
	), []map[string]any{
		semanticNode("cb-a", 30, 30, semanticOptions("elk.layered.considerModelOrder.groupModelOrder.cycleBreakingId", "4")),
		semanticNode("cb-b", 30, 30, semanticOptions("elk.layered.considerModelOrder.groupModelOrder.cycleBreakingId", "3")),
		semanticNode("cb-c", 30, 30, semanticOptions("elk.layered.considerModelOrder.groupModelOrder.cycleBreakingId", "2")),
		semanticNode("cb-d", 30, 30, semanticOptions("elk.layered.considerModelOrder.groupModelOrder.cycleBreakingId", "1")),
	}, []map[string]any{
		semanticEdge("cb-ac", "cb-a", "cb-c", nil), semanticEdge("cb-ba", "cb-b", "cb-a", nil),
		semanticEdge("cb-bc", "cb-b", "cb-c", nil), semanticEdge("cb-ca", "cb-c", "cb-a", nil),
	})

	cmIDs := []string{"cm-a", "cm-b", "cm-c", "cm-d", "cm-w", "cm-x", "cm-y", "cm-z"}
	cmNodes := make([]map[string]any, 0, len(cmIDs))
	for index, id := range cmIDs {
		groupID := "2"
		if index%4 >= 2 {
			groupID = "1"
		}
		cmNodes = append(cmNodes, semanticNode(id, 30, 30, semanticOptions(
			"elk.layered.considerModelOrder.groupModelOrder.crossingMinimizationId", groupID)))
	}
	cmGroup := semanticGraph("layered", semanticOptions(
		"elk.direction", "RIGHT",
		"elk.layered.considerModelOrder.strategy", "NODES_AND_EDGES",
		"elk.layered.crossingMinimization.strategy", "LAYER_SWEEP",
		"elk.layered.thoroughness", "3",
	), cmNodes, []map[string]any{
		semanticEdge("cm-e0", "cm-a", "cm-w", nil),
		semanticEdge("cm-e1", "cm-b", "cm-y", nil),
		semanticEdge("cm-e2", "cm-b", "cm-z", nil),
		semanticEdge("cm-e3", "cm-c", "cm-w", nil),
		semanticEdge("cm-e4", "cm-c", "cm-y", nil),
		semanticEdge("cm-e5", "cm-d", "cm-z", nil),
	})

	component := semanticGraph("layered", semanticOptions(
		"elk.direction", "RIGHT",
		"elk.separateConnectedComponents", "true",
		"elk.aspectRatio", "1000",
		"elk.layered.considerModelOrder.components", "GROUP_MODEL_ORDER",
	), []map[string]any{
		semanticNode("group-a1", 30, 30, nil), semanticNode("group-a2", 30, 30, nil),
		semanticNode("group-b1", 30, 30, nil), semanticNode("group-b2", 30, 30, nil),
		semanticNode("group-c1", 30, 30, nil), semanticNode("group-c2", 30, 30, nil),
	}, []map[string]any{
		semanticEdge("group-a", "group-a1", "group-a2", nil),
		semanticEdge("group-b", "group-b1", "group-b2", nil),
		semanticEdge("group-c", "group-c1", "group-c2", nil),
	})
	setComponentGroups := func(a, b, c string) semanticEdit {
		return func(graph map[string]any) {
			for _, pair := range []struct{ prefix, value string }{{"a", a}, {"b", b}, {"c", c}} {
				for _, suffix := range []string{"1", "2"} {
					semanticElementOption("group-"+pair.prefix+suffix,
						"elk.layered.considerModelOrder.groupModelOrder.componentGroupId", pair.value)(graph)
				}
			}
		}
	}

	preferredSource := layeredPreferredCycleGraph([][2]string{
		{"a", "e"}, {"b", "e"}, {"d", "a"}, {"e", "a"}, {"e", "b"}, {"e", "d"},
	})
	preferredTarget := layeredPreferredCycleGraph([][2]string{
		{"a", "c"}, {"c", "d"}, {"c", "e"}, {"d", "c"}, {"e", "d"},
	})

	return []releaseSemanticCase{
		{
			name: "layered-service-in-layer-constraints", claim: "inLayerPredOf and inLayerSuccOf are InteractiveLayeredGraphVisitor service annotations, not direct LayoutJSON inputs",
			upstream: "ELK InteractiveLayeredGraphVisitor and Layered.melk", oracleNoOp: true,
			variants: []releaseSemanticVariant{
				semanticVariant("unset", inLayer),
				semanticVariant("predecessor", inLayer, semanticElementOption("b", "elk.layered.crossingMinimization.inLayerPredOf", "a")),
				semanticVariant("successor", inLayer, semanticElementOption("a", "elk.layered.crossingMinimization.inLayerSuccOf", "b")),
			},
		},
		{
			name: "layered-layer-split-value", claim: "ALTERNATING unzipping honors a per-layer split count larger than the default",
			upstream: "ELK ad04950f8 and follow-up AlternatingLayerUnzipper changes", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("split-two", unzip, semanticElementOption("middle-1", "elk.layered.layerUnzipping.layerSplit", "2")),
				semanticVariant("split-three", unzip, semanticElementOption("middle-1", "elk.layered.layerUnzipping.layerSplit", "3")),
			},
		},
		{
			name: "layered-layer-split-long-edge-reset", claim: "ALTERNATING unzipping can retain alternation across long-edge dummies",
			upstream: "ELK AlternatingLayerUnzipper resetOnLongEdges", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("reset", resetGraph, setReset("true")),
				semanticVariant("retain", resetGraph, setReset("false")),
			},
		},
		{
			name: "layered-layer-split-minimize-edge-length", claim: "ALTERNATING unzipping can choose shifts that minimize introduced edge length",
			upstream: "ELK AlternatingLayerUnzipper minimizeEdgeLength", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("off", unzip, semanticElementOption("middle-1", "elk.layered.layerUnzipping.minimizeEdgeLength", "false")),
				semanticVariant("on", unzip, semanticElementOption("middle-1", "elk.layered.layerUnzipping.minimizeEdgeLength", "true")),
			},
		},
		{
			name: "layered-network-simplex-recompute", claim: "network-simplex node flexibility can rerun a selected node-placement strategy using the flexible sizes",
			upstream: "ELK 38d0a9183 NodePlacementRepeater and recomputeNodePlacement", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("simple", flex, semanticRootOption("elk.layered.nodePlacement.networkSimplex.nodeFlexibility.recomputeNodePlacement", "SIMPLE")),
				semanticVariant("brandes-koepf", flex, semanticRootOption("elk.layered.nodePlacement.networkSimplex.nodeFlexibility.recomputeNodePlacement", "BRANDES_KOEPF")),
			},
		},
		{
			name: "layered-cb-model-order-mode-equivalence", claim: "DFS_NODE_ORDER treats MODEL_ORDER and ONLY_WITHIN_GROUP identically; only ENFORCED activates group IDs",
			upstream: "ELK DFSNodeOrderCycleBreaker groupModelOrder boolean", oracleNoOp: true,
			variants: []releaseSemanticVariant{
				semanticVariant("within", cbGroup, semanticRootOption("elk.layered.considerModelOrder.groupModelOrder.cbGroupOrderStrategy", "ONLY_WITHIN_GROUP")),
				semanticVariant("model-order", cbGroup, semanticRootOption("elk.layered.considerModelOrder.groupModelOrder.cbGroupOrderStrategy", "MODEL_ORDER")),
			},
		},
		{
			name: "layered-crossing-minimization-groups", claim: "crossingMinimizationId and ENFORCED group order can override input model order",
			upstream: "ELK CMGroupModelOrderCalculator and ModelOrderBarycenterHeuristic", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("model-order", cmGroup, semanticRootOption("elk.layered.considerModelOrder.groupModelOrder.cmGroupOrderStrategy", "MODEL_ORDER")),
				semanticVariant("enforced", cmGroup, semanticRootOption("elk.layered.considerModelOrder.groupModelOrder.cmGroupOrderStrategy", "ENFORCED")),
			},
		},
		{
			name: "layered-unused-component-group-id", claim: "componentGroupId is exposed in metadata but has no consumer in ELK 0.12 component placement",
			upstream: "ELK Layered.melk declaration; no 0.12 implementation read", oracleNoOp: true,
			variants: []releaseSemanticVariant{
				semanticVariant("ascending", component, setComponentGroups("1", "2", "3")),
				semanticVariant("descending", component, setComponentGroups("3", "2", "1")),
			},
		},
		{
			name: "layered-cycle-preferred-source", claim: "SCC_NODE_TYPE can prefer a cycle-breaking group as an SCC source",
			upstream: "ELK SCCNodeTypeCycleBreaker cbPreferredSourceId", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("group-one", preferredSource, semanticRootOption("elk.layered.considerModelOrder.groupModelOrder.cbPreferredSourceId", "1")),
				semanticVariant("group-five", preferredSource, semanticRootOption("elk.layered.considerModelOrder.groupModelOrder.cbPreferredSourceId", "5")),
			},
		},
		{
			name: "layered-cycle-preferred-target", claim: "SCC_NODE_TYPE can prefer a cycle-breaking group as an SCC target",
			upstream: "ELK SCCNodeTypeCycleBreaker cbPreferredTargetId", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("group-one", preferredTarget, semanticRootOption("elk.layered.considerModelOrder.groupModelOrder.cbPreferredTargetId", "1")),
				semanticVariant("group-five", preferredTarget, semanticRootOption("elk.layered.considerModelOrder.groupModelOrder.cbPreferredTargetId", "5")),
			},
		},
	}
}

func layeredPreferredCycleGraph(edges [][2]string) map[string]any {
	children := make([]map[string]any, 0, 5)
	for index, id := range []string{"a", "b", "c", "d", "e"} {
		children = append(children, semanticNode(id, 30, 30, semanticOptions(
			"elk.layered.considerModelOrder.groupModelOrder.cycleBreakingId", string(rune('1'+index)))))
	}
	semanticEdges := make([]map[string]any, 0, len(edges))
	for index, edge := range edges {
		semanticEdges = append(semanticEdges, semanticEdge("preferred-"+string(rune('a'+index)), edge[0], edge[1], nil))
	}
	return semanticGraph("layered", semanticOptions(
		"elk.direction", "RIGHT",
		"elk.layered.cycleBreaking.strategy", "SCC_NODE_TYPE",
		"elk.layered.considerModelOrder.strategy", "NODES_AND_EDGES",
	), children, semanticEdges)
}
