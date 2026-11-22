// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package stress_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/d2lang/elk-go/graphjson"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/force"
	"github.com/d2lang/elk-go/internal/oracle"
	"github.com/d2lang/elk-go/internal/stress"
)

func TestELKJS0120StressOracle(t *testing.T) {
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Skip(err)
	}
	input := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"stress","elk.randomSeed":7},"children":[{"id":"a","width":30,"height":20},{"id":"b","width":40,"height":30},{"id":"c","width":20,"height":25}],"edges":[{"id":"e1","sources":["a"],"targets":["b"]},{"id":"e2","sources":["b"],"targets":["c"]}]}`)
	if err := oracle.Differential(context.Background(), runner, input, stressLayoutJSON, oracle.ExactTolerance); err != nil {
		t.Fatal(err)
	}
}

func stressLayoutJSON(input []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()
	var value graphjson.Graph
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	options := core.NewOptionRegistry()
	force.RegisterOptions(options)
	stress.RegisterOptions(options)
	algorithms := core.NewAlgorithmRegistry()
	if err := stress.Register(algorithms); err != nil {
		return nil, err
	}
	codec := core.NewJSONCodec(options)
	model, err := codec.Import(&value)
	if err != nil {
		return nil, err
	}
	if err := core.NewEngine(options, algorithms).Layout(model); err != nil {
		return nil, err
	}
	if err := codec.TransferLayout(model); err != nil {
		return nil, err
	}
	return json.Marshal(&value)
}

// Expected output was captured from the official elkjs 0.12.0 asset and guards
// the 0.12 force initialization plus majorization order.
func TestOfficialELKJS0120Stress(t *testing.T) {
	input := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"stress"},"children":[{"id":"a","width":20,"height":20},{"id":"b","width":20,"height":20}],"edges":[{"id":"e","sources":["a"],"targets":["b"]}]}`)
	want := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"stress"},"children":[{"id":"a","width":20,"height":20,"x":148.92423315100032,"y":64.62860538419602},{"id":"b","width":20,"height":20,"x":50,"y":50}],"edges":[{"id":"e","sources":["a"],"targets":["b"],"sections":[{"id":"e_s0","startPoint":{"x":148.92423315100032,"y":73.1498367429606},"endPoint":{"x":70,"y":61.47876864123542}}],"container":"root"}],"x":0,"y":0,"width":218.92423315100032,"height":134.62860538419602}`)
	got, err := stressLayoutJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := oracle.CompareJSON(want, got, oracle.ExactTolerance); err != nil {
		t.Fatal(err)
	}
}

func TestOfficialELKJS0120StressComponentsLabelsAndOptions(t *testing.T) {
	input := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"stress","elk.randomSeed":9,"elk.stress.desiredEdgeLength":70,"elk.stress.dimension":"X","elk.stress.epsilon":0.0001,"elk.stress.iterationLimit":10,"elk.padding":"[top=9,left=10,bottom=11,right=12]"},"children":[{"id":"a","width":20,"height":10,"layoutOptions":{"elk.stress.fixed":true}},{"id":"b","width":30,"height":20},{"id":"c","width":12,"height":12}],"edges":[{"id":"e","sources":["a"],"targets":["b"],"layoutOptions":{"elk.stress.desiredEdgeLength":40},"labels":[{"text":"L","width":10,"height":5}]}]}`)
	want := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"stress","elk.randomSeed":9,"elk.stress.desiredEdgeLength":70,"elk.stress.dimension":"X","elk.stress.epsilon":0.0001,"elk.stress.iterationLimit":10,"elk.padding":"[top=9,left=10,bottom=11,right=12]"},"children":[{"id":"a","width":20,"height":10,"layoutOptions":{"elk.stress.fixed":true},"x":14.297173438348597,"y":65.75164786892884},{"id":"b","width":30,"height":20,"x":10,"y":9},{"id":"c","width":12,"height":12,"x":10,"y":155.75164786892884}],"edges":[{"id":"e","sources":["a"],"targets":["b"],"layoutOptions":{"elk.stress.desiredEdgeLength":40},"labels":[{"text":"L","width":10,"height":5,"x":29.6485867191743,"y":49.87582393446442}],"sections":[{"id":"e_s0","startPoint":{"x":24.365077224082285,"y":65.75164786892884},"endPoint":{"x":24.864192428532625,"y":29}}],"container":"root"}],"x":0,"y":0,"width":52,"height":178.75164786892884}`)
	got, err := stressLayoutJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := oracle.CompareJSON(want, got, oracle.ExactTolerance); err != nil {
		t.Fatal(err)
	}
}
