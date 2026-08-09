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
	"encoding/json"
	"testing"

	"github.com/d2lang/elk-go/graphjson"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/force"
	"github.com/d2lang/elk-go/internal/oracle"
	"github.com/d2lang/elk-go/internal/stress"
)

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
	codec := core.NewCodec(options)
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

// Expected output was frozen from D2's embedded elkjs 0.8.2 asset
// (ELK v0.8.1) and guards force initialization plus majorization order.
func TestFrozenELKJS082Stress(t *testing.T) {
	input := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"stress"},"children":[{"id":"a","width":20,"height":20},{"id":"b","width":20,"height":20}],"edges":[{"id":"e","sources":["a"],"targets":["b"]}]}`)
	want := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"stress"},"children":[{"id":"a","width":20,"height":20,"x":148.92423315100032,"y":64.62860538419605},{"id":"b","width":20,"height":20,"x":50,"y":50}],"edges":[{"id":"e","sources":["a"],"targets":["b"],"sections":[{"id":"e_s0","startPoint":{"x":148.92423315100032,"y":73.14983674296063},"endPoint":{"x":70,"y":61.47876864123542}}],"container":"root"}],"x":0,"y":0,"width":218.92423315100032,"height":134.62860538419605}`)
	got, err := stressLayoutJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := oracle.CompareJSON(want, got, oracle.DefaultTolerance); err != nil {
		t.Fatal(err)
	}
}

func TestFrozenELKJS082StressComponentsLabelsAndOptions(t *testing.T) {
	input := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"stress","elk.randomSeed":9,"elk.stress.desiredEdgeLength":70,"elk.stress.dimension":"X","elk.stress.epsilon":0.0001,"elk.stress.iterationLimit":10,"elk.padding":"[top=9,left=10,bottom=11,right=12]"},"children":[{"id":"a","width":20,"height":10,"layoutOptions":{"elk.stress.fixed":true}},{"id":"b","width":30,"height":20},{"id":"c","width":12,"height":12}],"edges":[{"id":"e","sources":["a"],"targets":["b"],"layoutOptions":{"elk.stress.desiredEdgeLength":40},"labels":[{"text":"L","width":10,"height":5}]}]}`)
	want := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"stress","elk.randomSeed":9,"elk.stress.desiredEdgeLength":70,"elk.stress.dimension":"X","elk.stress.epsilon":0.0001,"elk.stress.iterationLimit":10,"elk.padding":"[top=9,left=10,bottom=11,right=12]"},"children":[{"id":"a","width":20,"height":10,"layoutOptions":{"elk.stress.fixed":true},"x":14.354829303752108,"y":65.91054197935557},{"id":"b","width":30,"height":20,"x":10,"y":9},{"id":"c","width":12,"height":12,"x":10,"y":155.91054197935557}],"edges":[{"id":"e","sources":["a"],"targets":["b"],"layoutOptions":{"elk.stress.desiredEdgeLength":40},"labels":[{"text":"L","width":10,"height":5,"x":29.677414651876056,"y":49.955270989677786}],"sections":[{"id":"e_s0","startPoint":{"x":24.41697185434467,"y":65.91054197935557},"endPoint":{"x":24.875714898814874,"y":29}}],"container":"root"}],"x":0,"y":0,"width":52,"height":178.91054197935557}`)
	got, err := stressLayoutJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := oracle.CompareJSON(want, got, oracle.DefaultTolerance); err != nil {
		t.Fatal(err)
	}
}
