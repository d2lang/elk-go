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
	"context"
	"encoding/json"
	"testing"

	"github.com/d2lang/elk-go/graphjson"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/force"
	"github.com/d2lang/elk-go/internal/oracle"
)

func TestELKJS0120ForceOracle(t *testing.T) {
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Skip(err)
	}
	fixtures := []string{
		`{"id":"root","layoutOptions":{"elk.algorithm":"force","elk.randomSeed":7},"children":[{"id":"a","width":30,"height":20},{"id":"b","width":40,"height":30},{"id":"c","width":20,"height":25}],"edges":[{"id":"e1","sources":["a"],"targets":["b"]},{"id":"e2","sources":["b"],"targets":["c"]}]}`,
		`{"id":"root","layoutOptions":{"elk.algorithm":"force","elk.randomSeed":7,"elk.force.iterations":20},"children":[{"id":"a","width":30,"height":20},{"id":"b","width":40,"height":30}],"edges":[{"id":"e","sources":["a"],"targets":["b"],"layoutOptions":{"elk.force.repulsivePower":2}}]}`,
	}
	for index, input := range fixtures {
		if err := oracle.Differential(context.Background(), runner, []byte(input), forceLayoutJSON, oracle.ExactTolerance); err != nil {
			t.Fatalf("fixture %d: %v", index, err)
		}
	}
}

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
// the Java RNG, force-iteration order, and routed edge geometry.
func TestOfficialELKJS0120Force(t *testing.T) {
	input := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"force"},"children":[{"id":"a","width":20,"height":20},{"id":"b","width":20,"height":20}],"edges":[{"id":"e","sources":["a"],"targets":["b"]}]}`)
	want := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"force"},"children":[{"id":"a","width":20,"height":20,"x":96.89408131207513,"y":56.934549690384046},{"id":"b","width":20,"height":20,"x":50,"y":50}],"edges":[{"id":"e","sources":["a"],"targets":["b"],"sections":[{"id":"e_s0","startPoint":{"x":96.89408131207513,"y":65.45578104914863},"endPoint":{"x":70,"y":61.47876864123541}}],"container":"root"}],"x":0,"y":0,"width":166.89408131207512,"height":126.93454969038405}`)
	got, err := forceLayoutJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := oracle.CompareJSON(want, got, oracle.ExactTolerance); err != nil {
		t.Fatal(err)
	}
}

func TestOfficialELKJS0120ForceComponentsLabelsAndOptions(t *testing.T) {
	input := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"force","elk.randomSeed":7,"elk.force.model":"EADES","elk.force.iterations":20,"elk.force.repulsion":4,"elk.spacing.nodeNode":30,"elk.aspectRatio":1.2,"elk.padding":"[top=11,left=12,bottom=13,right=14]"},"children":[{"id":"a","width":30,"height":20,"labels":[{"text":"A","width":8,"height":6,"x":2,"y":3}]},{"id":"b","width":20,"height":40},{"id":"c","width":15,"height":15}],"edges":[{"id":"e","sources":["a"],"targets":["b"],"layoutOptions":{"elk.priority":2,"elk.force.repulsivePower":1},"labels":[{"text":"edge","width":16,"height":8,"layoutOptions":{"elk.edgeLabels.inline":true}}]}]}`)
	want := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"force","elk.randomSeed":7,"elk.force.model":"EADES","elk.force.iterations":20,"elk.force.repulsion":4,"elk.spacing.nodeNode":30,"elk.aspectRatio":1.2,"elk.padding":"[top=11,left=12,bottom=13,right=14]"},"children":[{"id":"a","width":30,"height":20,"labels":[{"text":"A","width":8,"height":6,"x":2,"y":3}],"x":305.2523802809012,"y":11},{"id":"b","width":20,"height":40,"x":12,"y":197.27096603655926},{"id":"c","width":15,"height":15,"x":12.000000000000014,"y":267.27096603655923}],"edges":[{"id":"e","sources":["a"],"targets":["b"],"layoutOptions":{"elk.priority":2,"elk.force.repulsivePower":1},"labels":[{"text":"edge","width":16,"height":8,"layoutOptions":{"elk.edgeLabels.inline":true},"x":-92.87379833421913,"y":-63.74999920041512}],"sections":[{"id":"e_s0","startPoint":{"x":305.2523802809012,"y":30.871051113743327},"endPoint":{"x":32,"y":210.6902652940637},"bendPoints":[{"x":427.12620166578085,"y":298.02090068719446}]}],"container":"root"}],"x":0,"y":0,"width":349.2523802809012,"height":295.27096603655923}`)
	got, err := forceLayoutJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := oracle.CompareJSON(want, got, oracle.ExactTolerance); err != nil {
		t.Fatal(err)
	}
}
