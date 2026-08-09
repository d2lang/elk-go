// Copyright (c) 2018, 2020 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package rectpacking_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/d2lang/elk-go/graphjson"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/oracle"
	"github.com/d2lang/elk-go/internal/rectpacking"
)

func rectpackingLayoutJSON(input []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()
	var value graphjson.Graph
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	options := core.NewOptionRegistry()
	rectpacking.RegisterOptions(options)
	algorithms := core.NewAlgorithmRegistry()
	if err := rectpacking.Register(algorithms); err != nil {
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

func TestFrozenELKJS082RectPacking(t *testing.T) {
	input := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"rectpacking"},"children":[{"id":"a","width":30,"height":20},{"id":"b","width":40,"height":10},{"id":"c","width":20,"height":30}]}`)
	want := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"rectpacking"},"children":[{"id":"a","width":30,"height":20,"x":15,"y":15},{"id":"b","width":40,"height":10,"x":15,"y":50},{"id":"c","width":20,"height":30,"x":70,"y":15}],"x":0,"y":0,"width":105,"height":75}`)
	got, err := rectpackingLayoutJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := oracle.CompareJSON(want, got, oracle.DefaultTolerance); err != nil {
		t.Fatal(err)
	}
}

func TestFrozenOracleRectPackingOptions(t *testing.T) {
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Skip(err)
	}
	fixtures := map[string]string{
		"first iteration":            `{"id":"root","layoutOptions":{"elk.algorithm":"rectpacking","elk.rectpacking.onlyFirstIteration":true,"elk.rectpacking.optimizationGoal":"AREA_DRIVEN","elk.spacing.nodeNode":7,"elk.padding":"[top=3,left=5,bottom=11,right=13]"},"children":[{"id":"a","width":23,"height":17},{"id":"b","width":31,"height":11},{"id":"c","width":13,"height":29},{"id":"d","width":41,"height":7}]}`,
		"target width no compaction": `{"id":"root","layoutOptions":{"elk.algorithm":"rectpacking","elk.rectpacking.targetWidth":80,"elk.rectpacking.rowCompaction":false,"elk.spacing.nodeNode":9},"children":[{"id":"a","width":23,"height":17},{"id":"b","width":31,"height":11},{"id":"c","width":13,"height":29},{"id":"d","width":41,"height":7}]}`,
		"default mixed sizes":        `{"id":"root","layoutOptions":{"elk.algorithm":"rectpacking"},"children":[{"id":"a","width":12,"height":42},{"id":"b","width":33,"height":11},{"id":"c","width":24,"height":20},{"id":"d","width":18,"height":36},{"id":"e","width":50,"height":9}]}`,
	}
	for name, input := range fixtures {
		t.Run(name, func(t *testing.T) {
			if err := oracle.Differential(context.Background(), runner, []byte(input), rectpackingLayoutJSON, oracle.DefaultTolerance); err != nil {
				t.Fatal(err)
			}
		})
	}
}
