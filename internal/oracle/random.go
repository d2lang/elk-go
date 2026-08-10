// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0/.
//
// SPDX-License-Identifier: EPL-2.0

package oracle

import (
	"encoding/json"
	"fmt"
)

// RandomCase is deliberately described by small scalar parameters so a Go
// fuzz failure can shrink to a compact graph and be reproduced exactly.
type RandomCase struct {
	Seed      uint64
	NodeCount uint8
	EdgeCount uint8
	// D2Options adds the exact option profile and leaf-node self-loop option
	// emitted by D2's ELK adapter.
	D2Options bool
}

type generatedGraph struct {
	ID            string            `json:"id"`
	LayoutOptions map[string]string `json:"layoutOptions"`
	Children      []generatedNode   `json:"children"`
	Edges         []generatedEdge   `json:"edges"`
}

type generatedNode struct {
	ID            string            `json:"id"`
	Width         float64           `json:"width"`
	Height        float64           `json:"height"`
	Labels        []generatedLabel  `json:"labels,omitempty"`
	LayoutOptions map[string]string `json:"layoutOptions,omitempty"`
}

type generatedLabel struct {
	Text   string  `json:"text"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type generatedEdge struct {
	ID      string   `json:"id"`
	Sources []string `json:"sources"`
	Targets []string `json:"targets"`
}

// JSON returns a deterministic layered ELK graph. Counts are bounded to keep
// fuzz cases cheap; zero counts are promoted to one node and zero edges.
// D2Options adds D2's exact adapter profile so generated cases exercise that
// compatibility contract in addition to the captured E2E corpus.
func (randomCase RandomCase) JSON() ([]byte, error) {
	nodeCount := int(randomCase.NodeCount)%16 + 1
	edgeCount := int(randomCase.EdgeCount) % 33
	random := splitMix64{state: randomCase.Seed}
	directions := [...]string{"DOWN", "RIGHT", "UP", "LEFT"}
	graph := generatedGraph{
		ID: "root",
		LayoutOptions: map[string]string{
			"elk.algorithm": "layered",
			"elk.direction": directions[random.next()%uint64(len(directions))],
		},
		Children: make([]generatedNode, nodeCount),
		Edges:    make([]generatedEdge, edgeCount),
	}
	if randomCase.D2Options {
		d2Options := map[string]string{
			"elk.hierarchyHandling":                       "INCLUDE_CHILDREN",
			"elk.layered.thoroughness":                    "8",
			"elk.layered.spacing.edgeEdgeBetweenLayers":   "50",
			"elk.spacing.edgeNode":                        "40",
			"elk.layered.nodePlacement.bk.fixedAlignment": "BALANCED",
			"elk.layered.considerModelOrder.strategy":     "NODES_AND_EDGES",
			"elk.layered.cycleBreaking.strategy":          "GREEDY_MODEL_ORDER",
			"elk.nodeSize.constraints":                    "MINIMUM_SIZE",
			"spacing.nodeNodeBetweenLayers":               "70",
			"spacing.edgeNodeBetweenLayers":               "40",
			"elk.spacing.nodeSelfLoop":                    "50",
		}
		for key, value := range d2Options {
			graph.LayoutOptions[key] = value
		}
	}
	for index := range graph.Children {
		graph.Children[index] = generatedNode{
			ID:     fmt.Sprintf("n%d", index),
			Width:  float64(20 + random.next()%101),
			Height: float64(15 + random.next()%76),
		}
		if randomCase.D2Options {
			graph.Children[index].LayoutOptions = map[string]string{
				"elk.layered.edgeRouting.selfLoopDistribution": "EQUALLY",
			}
		}
		if random.next()%4 == 0 {
			graph.Children[index].Labels = []generatedLabel{{
				Text:   fmt.Sprintf("node %d", index),
				Width:  float64(25 + random.next()%61),
				Height: float64(10 + random.next()%21),
			}}
		}
	}
	for index := range graph.Edges {
		source := random.next() % uint64(nodeCount)
		target := random.next() % uint64(nodeCount)
		graph.Edges[index] = generatedEdge{
			ID:      fmt.Sprintf("e%d", index),
			Sources: []string{fmt.Sprintf("n%d", source)},
			Targets: []string{fmt.Sprintf("n%d", target)},
		}
	}
	return json.Marshal(graph)
}

// splitMix64 is tiny, fully specified, and independent of math/rand's evolving
// implementation. This makes a saved seed stable across Go releases.
type splitMix64 struct {
	state uint64
}

func (random *splitMix64) next() uint64 {
	random.state += 0x9e3779b97f4a7c15
	value := random.state
	value = (value ^ value>>30) * 0xbf58476d1ce4e5b9
	value = (value ^ value>>27) * 0x94d049bb133111eb
	return value ^ value>>31
}
