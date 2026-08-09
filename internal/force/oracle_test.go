// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package force_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/d2lang/elk-go/graphjson"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/force"
	"github.com/d2lang/elk-go/internal/oracle"
)

func forceLayoutJSON(input []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()
	var value graphjson.Graph
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	options := core.NewOptionRegistry()
	force.RegisterOptions(options)
	algorithms := core.NewAlgorithmRegistry()
	if err := force.Register(algorithms); err != nil {
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
// (ELK v0.8.1) and guards the Java RNG and force-iteration order.
func TestFrozenELKJS082Force(t *testing.T) {
	input := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"force"},"children":[{"id":"a","width":20,"height":20},{"id":"b","width":20,"height":20}],"edges":[{"id":"e","sources":["a"],"targets":["b"]}]}`)
	want := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"force"},"children":[{"id":"a","width":20,"height":20,"x":96.93970495736805,"y":56.94129637197986},{"id":"b","width":20,"height":20,"x":49.99999999999999,"y":50}],"edges":[{"id":"e","sources":["a"],"targets":["b"],"sections":[{"id":"e_s0","startPoint":{"x":96.93970495736805,"y":65.46252773074444},"endPoint":{"x":70,"y":61.47876864123542}}],"container":"root"}],"x":0,"y":0,"width":166.93970495736806,"height":126.94129637197986}`)
	got, err := forceLayoutJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := oracle.CompareJSON(want, got, oracle.DefaultTolerance); err != nil {
		t.Fatal(err)
	}
}

func TestFrozenELKJS082ForceComponentsLabelsAndOptions(t *testing.T) {
	input := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"force","elk.randomSeed":7,"elk.force.model":"EADES","elk.force.iterations":20,"elk.force.repulsion":4,"elk.spacing.nodeNode":30,"elk.aspectRatio":1.2,"elk.padding":"[top=11,left=12,bottom=13,right=14]"},"children":[{"id":"a","width":30,"height":20,"labels":[{"text":"A","width":8,"height":6,"x":2,"y":3}]},{"id":"b","width":20,"height":40},{"id":"c","width":15,"height":15}],"edges":[{"id":"e","sources":["a"],"targets":["b"],"layoutOptions":{"elk.priority":2,"elk.force.repulsivePower":1},"labels":[{"text":"edge","width":16,"height":8,"layoutOptions":{"elk.edgeLabels.inline":true}}]}]}`)
	want := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"force","elk.randomSeed":7,"elk.force.model":"EADES","elk.force.iterations":20,"elk.force.repulsion":4,"elk.spacing.nodeNode":30,"elk.aspectRatio":1.2,"elk.padding":"[top=11,left=12,bottom=13,right=14]"},"children":[{"id":"a","width":30,"height":20,"labels":[{"text":"A","width":8,"height":6,"x":2,"y":3}],"x":1262.1908810589239,"y":422.8577377478938},{"id":"b","width":20,"height":40,"x":12,"y":11},{"id":"c","width":15,"height":15,"x":12,"y":472.8577377478938}],"edges":[{"id":"e","sources":["a"],"targets":["b"],"layoutOptions":{"elk.priority":2,"elk.force.repulsivePower":1},"labels":[{"text":"edge","width":16,"height":8,"layoutOptions":{"elk.edgeLabels.inline":true},"x":641.5954405294619,"y":227.9288688739469}],"sections":[{"id":"e_s0","startPoint":{"x":1262.1908810589239,"y":428.05538763750013},"endPoint":{"x":32,"y":34.20156674026242}}],"container":"root"}],"x":0,"y":0,"width":1306.1908810589239,"height":500.8577377478938}`)
	got, err := forceLayoutJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := oracle.CompareJSON(want, got, oracle.DefaultTolerance); err != nil {
		t.Fatal(err)
	}
}
