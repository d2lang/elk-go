// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0/.
//
// SPDX-License-Identifier: EPL-2.0

package mrtree_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/d2lang/elk-go/graphjson"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/mrtree"
	"github.com/d2lang/elk-go/internal/oracle"
)

// Official elkjs 0.12.0 and the native port differ only in the final binary
// rounding of a routed coordinate in eight option-matrix cases. The largest
// observed delta is 2.84217094304e-14; graph structure and routing are exact.
var mrtreeOracleULPTolerance = oracle.Tolerance{Absolute: 3e-14}

func TestELKJS0120OracleSimpleTree(t *testing.T) {
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Skip(err)
	}
	input := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"mrtree"},"children":[{"id":"a","width":30,"height":20},{"id":"b","width":40,"height":10},{"id":"c","width":20,"height":30}],"edges":[{"id":"e1","sources":["a"],"targets":["b"]},{"id":"e2","sources":["a"],"targets":["c"]}]}`)
	if err := oracle.Differential(context.Background(), runner, input, mrtreeLayoutJSON, oracle.ExactTolerance); err != nil {
		t.Fatal(err)
	}
}

func TestELKJS0120OracleTreeOptionsAndComponents(t *testing.T) {
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Skip(err)
	}
	fixtures := map[string]string{
		"deep variable-size tree":       `{"id":"root","layoutOptions":{"elk.algorithm":"mrtree","elk.padding":"[top=7,left=11,bottom=13,right=17]","elk.spacing.nodeNode":9},"children":[{"id":"a","width":23,"height":17},{"id":"b","width":31,"height":11},{"id":"c","width":13,"height":29},{"id":"d","width":41,"height":7},{"id":"e","width":19,"height":21}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"ac","sources":["a"],"targets":["c"]},{"id":"bd","sources":["b"],"targets":["d"]},{"id":"be","sources":["b"],"targets":["e"]}]}`,
		"disconnected priority packing": `{"id":"root","layoutOptions":{"elk.algorithm":"mrtree","elk.spacing.nodeNode":12},"children":[{"id":"a","width":20,"height":20,"layoutOptions":{"elk.priority":1}},{"id":"b","width":30,"height":10},{"id":"c","width":15,"height":25,"layoutOptions":{"elk.priority":9}},{"id":"d","width":22,"height":18},{"id":"e","width":11,"height":13}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"cd","sources":["c"],"targets":["d"]}]}`,
		"breadth-first treeification":   `{"id":"root","layoutOptions":{"elk.algorithm":"mrtree","elk.mrtree.searchOrder":"BFS"},"children":[{"id":"a","width":20,"height":20},{"id":"b","width":20,"height":20},{"id":"c","width":20,"height":20},{"id":"d","width":20,"height":20}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"ac","sources":["a"],"targets":["c"]},{"id":"bd","sources":["b"],"targets":["d"]}]}`,
		"right direction":               `{"id":"root","layoutOptions":{"elk.algorithm":"mrtree","elk.direction":"RIGHT"},"children":[{"id":"a","width":30,"height":20},{"id":"b","width":40,"height":10},{"id":"c","width":20,"height":30}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"ac","sources":["a"],"targets":["c"]}]}`,
		"up direction":                  `{"id":"root","layoutOptions":{"elk.algorithm":"mrtree","elk.direction":"UP"},"children":[{"id":"a","width":30,"height":20},{"id":"b","width":40,"height":10},{"id":"c","width":20,"height":30}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"ac","sources":["a"],"targets":["c"]}]}`,
		"fan weighting":                 `{"id":"root","layoutOptions":{"elk.algorithm":"mrtree","elk.mrtree.weighting":"FAN"},"children":[{"id":"a","width":20,"height":20},{"id":"b","width":20,"height":20},{"id":"c","width":20,"height":20},{"id":"d","width":20,"height":20},{"id":"e","width":20,"height":20}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"ac","sources":["a"],"targets":["c"]},{"id":"bd","sources":["b"],"targets":["d"]},{"id":"be","sources":["b"],"targets":["e"]}]}`,
		"constraint weighting":          `{"id":"root","layoutOptions":{"elk.algorithm":"mrtree","elk.mrtree.weighting":"CONSTRAINT"},"children":[{"id":"a","width":20,"height":20},{"id":"b","width":20,"height":20,"layoutOptions":{"elk.mrtree.positionConstraint":"1"}},{"id":"c","width":20,"height":20,"layoutOptions":{"elk.mrtree.positionConstraint":"0"}}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"ac","sources":["a"],"targets":["c"]}]}`,
		"middle routing":                `{"id":"root","layoutOptions":{"elk.algorithm":"mrtree","elk.mrtree.edgeRoutingMode":"MIDDLE_TO_MIDDLE"},"children":[{"id":"a","width":30,"height":20},{"id":"b","width":40,"height":10},{"id":"c","width":20,"height":30}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"ac","sources":["a"],"targets":["c"]}]}`,
		"left direction":                `{"id":"root","layoutOptions":{"elk.algorithm":"mrtree","elk.direction":"LEFT"},"children":[{"id":"a","width":30,"height":20},{"id":"b","width":40,"height":10},{"id":"c","width":20,"height":30}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"ac","sources":["a"],"targets":["c"]}]}`,
		"descendants weighting":         `{"id":"root","layoutOptions":{"elk.algorithm":"mrtree","elk.mrtree.weighting":"DESCENDANTS"},"children":[{"id":"a","width":20,"height":20},{"id":"b","width":20,"height":20},{"id":"c","width":20,"height":20},{"id":"d","width":20,"height":20},{"id":"e","width":20,"height":20}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"ac","sources":["a"],"targets":["c"]},{"id":"bd","sources":["b"],"targets":["d"]},{"id":"be","sources":["b"],"targets":["e"]}]}`,
		"fixed graph size":              `{"id":"root","width":400,"height":300,"layoutOptions":{"elk.algorithm":"mrtree","elk.nodeSize.fixedGraphSize":"true"},"children":[{"id":"a","width":30,"height":20},{"id":"b","width":40,"height":10}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]}]}`,
		"compaction":                    `{"id":"root","layoutOptions":{"elk.algorithm":"mrtree","elk.mrtree.compaction":"true"},"children":[{"id":"n0","width":58,"height":37},{"id":"n1","width":48,"height":15},{"id":"n2","width":42,"height":17},{"id":"n3","width":46,"height":11},{"id":"n4","width":18,"height":45},{"id":"n5","width":22,"height":31},{"id":"n6","width":40,"height":25},{"id":"n7","width":46,"height":19}],"edges":[{"id":"e1","sources":["n0"],"targets":["n1"]},{"id":"e2","sources":["n1"],"targets":["n2"]},{"id":"e3","sources":["n0"],"targets":["n3"]},{"id":"e4","sources":["n1"],"targets":["n4"]},{"id":"e5","sources":["n1"],"targets":["n5"]},{"id":"e6","sources":["n3"],"targets":["n6"]},{"id":"e7","sources":["n2"],"targets":["n7"]}]}`,
		"multi-level and cycle routes":  `{"id":"root","layoutOptions":{"elk.algorithm":"mrtree"},"children":[{"id":"a","width":30,"height":20},{"id":"b","width":40,"height":10},{"id":"c","width":20,"height":30},{"id":"d","width":15,"height":12}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"ac","sources":["a"],"targets":["c"]},{"id":"bd","sources":["b"],"targets":["d"]},{"id":"ad","sources":["a"],"targets":["d"]},{"id":"da","sources":["d"],"targets":["a"]}]}`,
	}
	for name, input := range fixtures {
		t.Run(name, func(t *testing.T) {
			if err := oracle.Differential(context.Background(), runner, []byte(input), mrtreeLayoutJSON, mrtreeOracleULPTolerance); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func mrtreeLayoutJSON(input []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()
	var value graphjson.Graph
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	options := core.NewOptionRegistry()
	mrtree.RegisterOptions(options)
	algorithms := core.NewAlgorithmRegistry()
	if err := mrtree.Register(algorithms); err != nil {
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
