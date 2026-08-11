// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0/.
//
// SPDX-License-Identifier: EPL-2.0

package oracle

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	elk "github.com/d2lang/elk-go"
)

// TestD2ContractExposedOptionMatrixParity varies each scalar layout field D2
// exposes to its ELK adapter. Every variant must match official elkjs 0.12.0
// exactly, and each pair must produce different geometry in the oracle so the
// test cannot pass merely because an option was ignored.
//
// Algorithm selection is varied across all eleven providers by
// TestCandidateProviderMatrixParity. The companion coverage test below pins
// the direction and graph-feature fixtures run by TestCandidateCuratedParity.
func TestD2ContractExposedOptionMatrixParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_PARITY") != "1" {
		t.Skip("set ELK_GO_ORACLE_PARITY=1 to run the D2 contract differential")
	}
	runner := DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	pairs := []struct {
		name     string
		variants [2]string
	}{
		{
			name: "nodeNodeBetweenLayers",
			variants: [2]string{
				`{"id":"root","layoutOptions":{"elk.algorithm":"layered","elk.direction":"RIGHT","spacing.nodeNodeBetweenLayers":"17"},"children":[{"id":"a","width":20,"height":20},{"id":"b","width":20,"height":20}],"edges":[{"id":"e","sources":["a"],"targets":["b"]}]}`,
				`{"id":"root","layoutOptions":{"elk.algorithm":"layered","elk.direction":"RIGHT","spacing.nodeNodeBetweenLayers":"93"},"children":[{"id":"a","width":20,"height":20},{"id":"b","width":20,"height":20}],"edges":[{"id":"e","sources":["a"],"targets":["b"]}]}`,
			},
		},
		{
			name: "padding",
			variants: [2]string{
				`{"id":"root","layoutOptions":{"elk.algorithm":"layered","elk.direction":"RIGHT","elk.padding":"[top=1,left=2,bottom=3,right=4]"},"children":[{"id":"a","width":20,"height":20},{"id":"b","width":20,"height":20}],"edges":[{"id":"e","sources":["a"],"targets":["b"]}]}`,
				`{"id":"root","layoutOptions":{"elk.algorithm":"layered","elk.direction":"RIGHT","elk.padding":"[top=11,left=7,bottom=5,right=13]"},"children":[{"id":"a","width":20,"height":20},{"id":"b","width":20,"height":20}],"edges":[{"id":"e","sources":["a"],"targets":["b"]}]}`,
			},
		},
		{
			name: "edgeNodeBetweenLayers",
			variants: [2]string{
				`{"id":"root","layoutOptions":{"elk.algorithm":"layered","elk.direction":"RIGHT","elk.edgeRouting":"ORTHOGONAL","spacing.edgeNodeBetweenLayers":"5"},"children":[{"id":"n0","width":20,"height":20},{"id":"n1","width":20,"height":20},{"id":"n2","width":20,"height":20},{"id":"n3","width":20,"height":20},{"id":"n4","width":20,"height":20}],"edges":[{"id":"e1","sources":["n0"],"targets":["n1"]},{"id":"e2","sources":["n0"],"targets":["n2"]},{"id":"e3","sources":["n0"],"targets":["n3"]},{"id":"e4","sources":["n0"],"targets":["n4"]}]}`,
				`{"id":"root","layoutOptions":{"elk.algorithm":"layered","elk.direction":"RIGHT","elk.edgeRouting":"ORTHOGONAL","spacing.edgeNodeBetweenLayers":"80"},"children":[{"id":"n0","width":20,"height":20},{"id":"n1","width":20,"height":20},{"id":"n2","width":20,"height":20},{"id":"n3","width":20,"height":20},{"id":"n4","width":20,"height":20}],"edges":[{"id":"e1","sources":["n0"],"targets":["n1"]},{"id":"e2","sources":["n0"],"targets":["n2"]},{"id":"e3","sources":["n0"],"targets":["n3"]},{"id":"e4","sources":["n0"],"targets":["n4"]}]}`,
			},
		},
		{
			name: "nodeSelfLoop",
			variants: [2]string{
				`{"id":"root","layoutOptions":{"elk.algorithm":"layered","elk.direction":"RIGHT","elk.spacing.nodeSelfLoop":"5"},"children":[{"id":"a","width":40,"height":30,"layoutOptions":{"elk.layered.edgeRouting.selfLoopDistribution":"EQUALLY"}}],"edges":[{"id":"e","sources":["a"],"targets":["a"]}]}`,
				`{"id":"root","layoutOptions":{"elk.algorithm":"layered","elk.direction":"RIGHT","elk.spacing.nodeSelfLoop":"61"},"children":[{"id":"a","width":40,"height":30,"layoutOptions":{"elk.layered.edgeRouting.selfLoopDistribution":"EQUALLY"}}],"edges":[{"id":"e","sources":["a"],"targets":["a"]}]}`,
			},
		},
	}

	for _, pair := range pairs {
		pair := pair
		t.Run(pair.name, func(t *testing.T) {
			var signatures [2]string
			for index, raw := range pair.variants {
				input := []byte(raw)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				want, err := runner.Layout(ctx, input)
				cancel()
				if err != nil {
					t.Fatalf("variant %d oracle: %v", index, err)
				}
				got, err := elk.LayoutJSON(input)
				if err != nil {
					t.Fatalf("variant %d candidate: %v", index, err)
				}
				if err := CompareJSON(want, got, ExactTolerance); err != nil {
					t.Fatalf("variant %d: %v\ninput: %s", index, err, input)
				}
				signatures[index], err = geometrySignature(want)
				if err != nil {
					t.Fatalf("variant %d geometry: %v", index, err)
				}
			}
			if signatures[0] == signatures[1] {
				t.Fatal("option variants produced identical oracle geometry; fixture does not exercise the field")
			}
		})
	}
}

// TestD2ContractCuratedCoverage makes the D2 feature contract explicit. Every
// listed fixture is run with ExactTolerance by TestCandidateCuratedParity.
func TestD2ContractCuratedCoverage(t *testing.T) {
	if len(OfficialAlgorithmSuffixes) != 11 {
		t.Fatalf("provider-selection matrix has %d algorithms, want 11", len(OfficialAlgorithmSuffixes))
	}
	paths, err := CuratedInputs()
	if err != nil {
		t.Fatal(err)
	}
	available := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		available[filepath.Base(path)] = struct{}{}
	}
	required := map[string][]string{
		"directions": {
			"01_simple_chain_down.json",
			"04_direction_right.json",
			"05_direction_up.json",
			"06_direction_left.json",
		},
		"compound":      {"09_compound.json"},
		"ports":         {"10_ports.json"},
		"inline-labels": {"03_labels.json"},
		"self-loops":    {"08_self_loop.json"},
	}
	for feature, fixtures := range required {
		for _, fixture := range fixtures {
			if _, ok := available[fixture]; !ok {
				t.Errorf("D2 contract feature %s is missing curated fixture %s", feature, fixture)
			}
		}
	}
}

func geometrySignature(raw []byte) (string, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	removeLayoutOptions(value)
	encoded, err := json.Marshal(value)
	return string(encoded), err
}

func removeLayoutOptions(value any) {
	switch value := value.(type) {
	case map[string]any:
		delete(value, "layoutOptions")
		for _, child := range value {
			removeLayoutOptions(child)
		}
	case []any:
		for _, child := range value {
			removeLayoutOptions(child)
		}
	}
}
