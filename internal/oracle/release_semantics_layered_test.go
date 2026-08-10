// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package oracle

func buildLayeredSemanticCases() []releaseSemanticCase {
	var cases []releaseSemanticCase
	cases = append(cases, layeredOrderingSemanticCases()...)
	cases = append(cases, layeredLabelSemanticCases()...)
	cases = append(cases, layeredStructuralSemanticCases()...)
	return cases
}

func layeredOrderingSemanticCases() []releaseSemanticCase {
	modelOrder := semanticGraph("layered", semanticOptions(
		"elk.direction", "DOWN",
		"elk.layered.cycleBreaking.strategy", "GREEDY",
	), []map[string]any{
		semanticNode("first", 70, 40, nil), semanticNode("second", 70, 40, nil),
		semanticNode("third", 70, 40, nil), semanticNode("fourth", 70, 40, nil),
	}, []map[string]any{
		semanticEdge("e1", "first", "second", nil), semanticEdge("e2", "second", "third", nil),
		semanticEdge("e3", "third", "first", nil), semanticEdge("e4", "third", "fourth", nil),
	})

	server := semanticNode("server", 100, 80, semanticOptions("elk.portConstraints", "FIXED_SIDE"))
	server["ports"] = []map[string]any{
		{"id": "server.p1", "width": 4.0, "height": 4.0, "layoutOptions": semanticOptions("elk.port.side", "EAST")},
		{"id": "server.p2", "width": 4.0, "height": 4.0, "layoutOptions": semanticOptions("elk.port.side", "EAST")},
	}
	target := semanticNode("target", 100, 80, semanticOptions("elk.portConstraints", "FIXED_SIDE"))
	target["ports"] = []map[string]any{
		{"id": "target.q1", "width": 4.0, "height": 4.0, "layoutOptions": semanticOptions("elk.port.side", "WEST")},
		{"id": "target.q2", "width": 4.0, "height": 4.0, "layoutOptions": semanticOptions("elk.port.side", "WEST")},
	}
	portOrder := semanticGraph("layered", semanticOptions(
		"elk.direction", "RIGHT",
		"elk.layered.considerModelOrder.strategy", "NODES_AND_EDGES",
	), []map[string]any{server, target}, []map[string]any{
		semanticEdge("cross-2", "server.p2", "target.q1", nil),
		semanticEdge("cross-1", "server.p1", "target.q2", nil),
	})

	componentOrder := semanticGraph("layered", semanticOptions(
		"elk.direction", "RIGHT", "elk.separateConnectedComponents", "true", "elk.aspectRatio", "1000",
	), []map[string]any{
		semanticNode("a1", 30, 30, semanticOptions("elk.priority", "0")),
		semanticNode("a2", 30, 30, semanticOptions("elk.priority", "0")),
		semanticNode("b1", 30, 30, semanticOptions("elk.priority", "10")),
		semanticNode("b2", 30, 30, semanticOptions("elk.priority", "10")),
	}, []map[string]any{
		semanticEdge("a", "a1", "a2", nil), semanticEdge("b", "b1", "b2", nil),
	})

	groupOrder := semanticGraph("layered", semanticOptions(
		"elk.direction", "RIGHT",
		"elk.layered.considerModelOrder.strategy", "NODES_AND_EDGES",
		"elk.layered.cycleBreaking.strategy", "DFS_NODE_ORDER",
	), []map[string]any{
		semanticNode("a", 30, 30, semanticOptions("elk.layered.considerModelOrder.groupModelOrder.cycleBreakingId", "4")),
		semanticNode("b", 30, 30, semanticOptions("elk.layered.considerModelOrder.groupModelOrder.cycleBreakingId", "3")),
		semanticNode("c", 30, 30, semanticOptions("elk.layered.considerModelOrder.groupModelOrder.cycleBreakingId", "2")),
		semanticNode("d", 30, 30, semanticOptions("elk.layered.considerModelOrder.groupModelOrder.cycleBreakingId", "1")),
	}, []map[string]any{
		semanticEdge("ac", "a", "c", nil),
		semanticEdge("ba", "b", "a", nil),
		semanticEdge("bc", "b", "c", nil),
		semanticEdge("ca", "c", "a", nil),
	})

	return []releaseSemanticCase{
		{
			name: "layered-model-order", claim: "Layered model-order strategy participates in cycle breaking and crossing minimization",
			upstream: "ELK 568fa0325, 4b7235968 and later model-order fixes", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("none", modelOrder, semanticRootOption("elk.layered.considerModelOrder.strategy", "NONE")),
				semanticVariant("nodes-and-edges", modelOrder, semanticRootOption("elk.layered.considerModelOrder.strategy", "NODES_AND_EDGES")),
			},
		},
		{
			name: "layered-port-model-order", claim: "Layered can preserve explicit port model order instead of deriving it from edge order",
			upstream: "ELK fae54ae20 Port Model Order (#895) and 6629d35ac port sorting fix (#1067)", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("edge-derived", portOrder, semanticRootOption("elk.layered.considerModelOrder.portModelOrder", "false")),
				semanticVariant("port-model", portOrder, semanticRootOption("elk.layered.considerModelOrder.portModelOrder", "true")),
			},
		},
		{
			name: "layered-component-model-order", claim: "Layered can order disconnected components by model order instead of cumulative priority",
			upstream: "ELK f701890e1 component model-order row placer (#876) and later component fixes", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("priority", componentOrder, semanticRootOption("elk.layered.considerModelOrder.components", "NONE")),
				semanticVariant("model-order", componentOrder, semanticRootOption("elk.layered.considerModelOrder.components", "MODEL_ORDER")),
			},
		},
		{
			name: "layered-group-model-order", claim: "Layered cycle breaking can enforce relative ordering between model-order groups",
			upstream: "ELK 30b043510, 9319293e6 and follow-up group model-order strategy commits", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("within-groups", groupOrder, semanticRootOption("elk.layered.considerModelOrder.groupModelOrder.cbGroupOrderStrategy", "ONLY_WITHIN_GROUP")),
				semanticVariant("enforced-groups", groupOrder, semanticRootOption("elk.layered.considerModelOrder.groupModelOrder.cbGroupOrderStrategy", "ENFORCED")),
			},
		},
	}
}
func layeredLabelSemanticCases() []releaseSemanticCase {
	tailEdge := semanticEdge("edge", "a", "b", nil)
	tailEdge["labels"] = []map[string]any{{
		"id": "tail-label", "text": "tail", "width": 48.0, "height": 16.0,
		"layoutOptions": semanticOptions("elk.edgeLabels.placement", "TAIL"),
	}}
	tailLabel := semanticGraph("layered", semanticOptions(
		"elk.direction", "RIGHT", "elk.edgeRouting", "ORTHOGONAL",
	), []map[string]any{
		semanticNode("a", 50, 30, nil), semanticNode("b", 50, 30, nil),
	}, []map[string]any{tailEdge})

	inlineEdge := semanticEdge("edge", "a", "b", nil)
	inlineEdge["labels"] = []map[string]any{{
		"id": "center-label", "text": "inline", "width": 64.0, "height": 18.0,
		"layoutOptions": semanticOptions("elk.edgeLabels.placement", "CENTER"),
	}}
	inlineLabel := semanticGraph("layered", semanticOptions(
		"elk.direction", "RIGHT", "elk.edgeRouting", "ORTHOGONAL",
	), []map[string]any{
		semanticNode("a", 50, 30, nil), semanticNode("b", 50, 30, nil),
	}, []map[string]any{inlineEdge})

	loopNode := semanticNode("state", 100, 70, semanticOptions("elk.portConstraints", "FIXED_POS"))
	loopNode["ports"] = []map[string]any{
		{"id": "state.east", "x": 98.0, "y": 18.0, "width": 4.0, "height": 4.0, "layoutOptions": semanticOptions("elk.port.side", "EAST")},
		{"id": "state.west", "x": -2.0, "y": 48.0, "width": 4.0, "height": 4.0, "layoutOptions": semanticOptions("elk.port.side", "WEST")},
	}
	loopEdge := semanticEdge("loop", "state.east", "state.west", nil)
	loopEdge["labels"] = []map[string]any{{
		"id": "loop-label", "text": "retry", "width": 52.0, "height": 18.0,
		"layoutOptions": semanticOptions("elk.edgeLabels.placement", "CENTER", "elk.edgeLabels.inline", "true"),
	}}
	selfLoopLabel := semanticGraph("layered", semanticOptions(
		"elk.direction", "RIGHT", "elk.edgeRouting", "ORTHOGONAL", "elk.spacing.nodeSelfLoop", "35",
	), []map[string]any{loopNode, semanticNode("next", 60, 40, nil)}, []map[string]any{
		loopEdge, semanticEdge("advance", "state", "next", nil),
	})

	return []releaseSemanticCase{
		{
			name: "layered-tail-edge-label", claim: "Layered places a TAIL edge label relative to the edge source rather than target",
			upstream: "ELK b85ad1c16 Wrong tail edge label (#906)", observable: false,
			variants: []releaseSemanticVariant{semanticVariant("tail", tailLabel)},
		},
		{
			name: "layered-inline-center-label", claim: "Layered centers inline center labels directly on their edge",
			upstream: "ELK 0dfc1d0de layered: Center inline edge labels (#852)", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("external", inlineLabel, semanticElementOption("center-label", "elk.edgeLabels.inline", "false")),
				semanticVariant("inline", inlineLabel, semanticElementOption("center-label", "elk.edgeLabels.inline", "true")),
			},
		},
		{
			name: "layered-inline-opposing-self-loop-label", claim: "Layered routes inline labels on self-loops whose ports are on opposing node sides",
			upstream: "ELK 72d245ab6 Inline edge labels on self-loops with opposing sides (#1158)", observable: false,
			variants: []releaseSemanticVariant{semanticVariant("opposing-ports", selfLoopLabel)},
		},
	}
}
func layeredStructuralSemanticCases() []releaseSemanticCase {
	partitioned := semanticGraph("layered", semanticOptions(
		"elk.direction", "RIGHT",
	), []map[string]any{
		semanticNode("partition-2", 40, 30, semanticOptions("elk.partitioning.partition", "2")),
		semanticNode("bridge-1", 40, 30, nil),
		semanticNode("bridge-2", 40, 30, nil),
		semanticNode("partition-0", 40, 30, semanticOptions("elk.partitioning.partition", "0")),
	}, []map[string]any{
		semanticEdge("to-bridge", "partition-2", "bridge-1", nil),
		semanticEdge("bridge", "bridge-1", "bridge-2", nil),
		semanticEdge("to-lower-partition", "bridge-2", "partition-0", nil),
	})

	unzip := semanticGraph("layered", semanticOptions(
		"elk.direction", "RIGHT",
		"elk.layered.considerModelOrder.strategy", "PREFER_EDGES",
	), []map[string]any{
		semanticNode("source", 30, 30, nil),
		semanticNode("middle-1", 30, 30, semanticOptions("elk.layered.layerUnzipping.layerSplit", "2")),
		semanticNode("middle-2", 30, 30, nil),
		semanticNode("middle-3", 30, 30, nil),
		semanticNode("target", 30, 30, nil),
	}, []map[string]any{
		semanticEdge("s1", "source", "middle-1", nil), semanticEdge("s2", "source", "middle-2", nil),
		semanticEdge("s3", "source", "middle-3", nil), semanticEdge("t1", "middle-1", "target", nil),
		semanticEdge("t2", "middle-2", "target", nil), semanticEdge("t3", "middle-3", "target", nil),
	})

	coffman := semanticGraph("layered", semanticOptions(
		"elk.direction", "RIGHT",
		"elk.layered.layering.strategy", "COFFMAN_GRAHAM",
		"elk.layered.layering.coffmanGraham.layerBound", "2",
		"elk.layered.cycleBreaking.strategy", "GREEDY",
	), []map[string]any{
		semanticNode("a", 30, 30, nil), semanticNode("b", 30, 30, nil),
		semanticNode("c", 30, 30, nil), semanticNode("d", 30, 30, nil),
		semanticNode("e", 30, 30, nil), semanticNode("f", 30, 30, nil),
	}, []map[string]any{
		semanticEdge("ad", "a", "d", nil), semanticEdge("bd", "b", "d", nil),
		semanticEdge("ae", "a", "e", nil), semanticEdge("be", "b", "e", nil),
		semanticEdge("cf", "c", "f", nil),
	})

	return []releaseSemanticCase{
		{
			name: "layered-partitioning", claim: "Layered respects partition order when partitioned nodes are connected only through unpartitioned nodes",
			upstream: "ELK fa9df471e Make partitioning consider not directly connected partitions", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("disabled", partitioned, semanticRootOption("elk.partitioning.activate", "false")),
				semanticVariant("enabled", partitioned, semanticRootOption("elk.partitioning.activate", "true")),
			},
		},
		{
			name: "layered-layer-splitting", claim: "Layered ALTERNATING unzipping splits a wide three-node layer into two layers",
			upstream: "ELK ad04950f8, 95417f0a3, dcbe972ab6 and b1aca40b5 layer-unzipping changes", observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("none", unzip, semanticRootOption("elk.layered.layerUnzipping.strategy", "NONE")),
				semanticVariant("alternating", unzip, semanticRootOption("elk.layered.layerUnzipping.strategy", "ALTERNATING")),
			},
		},
		{
			name: "layered-coffman-graham-id-tiebreak", claim: "Coffman-Graham resolves equal predecessor labels deterministically by internal node id",
			upstream: "ELK c84f1d9e9 CoffmanGrahamLayerer: Use node id as secondary order criterion (#1161)", observable: false,
			variants: []releaseSemanticVariant{semanticVariant("symmetric-dag", coffman)},
		},
	}
}
