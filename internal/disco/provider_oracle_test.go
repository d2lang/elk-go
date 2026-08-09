// Copyright (c) 2017 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package disco_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/d2lang/elk-go/graphjson"
	"github.com/d2lang/elk-go/internal/basic"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/disco"
	"github.com/d2lang/elk-go/internal/oracle"
)

func discoLayoutJSON(input []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()
	var value graphjson.Graph
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	options := core.NewOptionRegistry()
	basic.RegisterOptions(options)
	disco.RegisterOptions(options)
	algorithms := core.NewAlgorithmRegistry()
	if err := basic.Register(algorithms); err != nil {
		return nil, err
	}
	if err := disco.Register(algorithms); err != nil {
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

func TestFrozenELKJS082DisCo(t *testing.T) {
	input := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"disco"},"children":[{"id":"a","width":30,"height":20},{"id":"b","width":40,"height":10},{"id":"c","width":20,"height":30}]}`)
	want := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"disco"},"children":[{"id":"a","width":30,"height":20,"x":55.74408889662569,"y":65.6229915791516},{"id":"b","width":40,"height":10,"x":55.6229915791516,"y":31.59177011894432},{"id":"c","width":20,"height":30,"x":11.955062071366594,"y":11.833964753892502}],"x":0,"y":0,"width":131.33585901557,"height":121.5780536505182}`)
	got, err := discoLayoutJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := oracle.CompareJSON(want, got, oracle.DefaultTolerance); err != nil {
		t.Fatal(err)
	}
}

func TestFrozenOracleDisCoComponentsAndOptions(t *testing.T) {
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Skip(err)
	}
	fixtures := map[string]string{
		"connected and isolated":   `{"id":"root","layoutOptions":{"elk.algorithm":"disco"},"children":[{"id":"a","width":30,"height":20},{"id":"b","width":40,"height":10},{"id":"c","width":20,"height":30}],"edges":[{"id":"e","sources":["a"],"targets":["b"]}]}`,
		"padding aspect spacing":   `{"id":"root","layoutOptions":{"elk.algorithm":"disco","elk.padding":"[top=7,left=11,bottom=13,right=17]","elk.aspectRatio":1.6,"elk.spacing.componentComponent":35},"children":[{"id":"a","width":23,"height":17,"x":4,"y":9},{"id":"b","width":31,"height":11,"x":71,"y":5},{"id":"c","width":13,"height":29,"x":2,"y":80},{"id":"d","width":41,"height":7,"x":105,"y":60}]}`,
		"traversal sort no fill":   `{"id":"root","layoutOptions":{"elk.algorithm":"disco","elk.polyomino.traversalStrategy":"SPIRAL","elk.polyomino.lowLevelSort":"BY_SIZE","elk.polyomino.fill":false},"children":[{"id":"a","width":12,"height":42},{"id":"b","width":33,"height":11},{"id":"c","width":24,"height":20},{"id":"d","width":18,"height":36}]}`,
		"ports labels routed edge": `{"id":"root","width":300,"height":200,"layoutOptions":{"elk.algorithm":"disco"},"labels":[{"text":"root","x":30,"y":10,"width":20,"height":8}],"ports":[{"id":"rp","x":300,"y":80,"width":5,"height":5}],"children":[{"id":"a","x":20,"y":30,"width":40,"height":30,"labels":[{"text":"A","x":5,"y":3,"width":11,"height":7}],"ports":[{"id":"pa","x":40,"y":10,"width":4,"height":4,"labels":[{"text":"P","x":2,"y":1,"width":5,"height":3}]}]},{"id":"b","x":130,"y":40,"width":35,"height":25},{"id":"c","x":10,"y":130,"width":20,"height":20}],"edges":[{"id":"e","sources":["pa"],"targets":["b"],"sections":[{"id":"s","startPoint":{"x":64,"y":42},"bendPoints":[{"x":90,"y":42},{"x":90,"y":52}],"endPoint":{"x":130,"y":52}}],"labels":[{"text":"edge","x":82,"y":35,"width":16,"height":6}]}]}`,
		"fixed component layout":   `{"id":"root","width":220,"height":160,"layoutOptions":{"elk.algorithm":"disco","elk.disco.componentCompaction.componentLayoutAlgorithm":"fixed"},"children":[{"id":"a","x":12,"y":18,"width":30,"height":20},{"id":"b","x":92,"y":22,"width":40,"height":10},{"id":"c","x":35,"y":100,"width":20,"height":30}],"edges":[{"id":"e","sources":["a"],"targets":["b"]}]}`,
	}
	for name, input := range fixtures {
		t.Run(name, func(t *testing.T) {
			want, err := runner.Layout(context.Background(), []byte(input))
			if err != nil {
				t.Fatal(err)
			}
			got, err := discoLayoutJSON([]byte(input))
			if err != nil {
				t.Fatal(err)
			}
			if err := oracle.CompareJSON(want, got, oracle.DefaultTolerance); err != nil {
				t.Fatalf("%v\nwant: %s\n got: %s", err, want, got)
			}
		})
	}
}
