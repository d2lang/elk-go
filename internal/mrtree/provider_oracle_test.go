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

func TestFrozenOracleSimpleTree(t *testing.T) {
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Skip(err)
	}
	input := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"mrtree"},"children":[{"id":"a","width":30,"height":20},{"id":"b","width":40,"height":10},{"id":"c","width":20,"height":30}],"edges":[{"id":"e1","sources":["a"],"targets":["b"]},{"id":"e2","sources":["a"],"targets":["c"]}]}`)
	if err := oracle.Differential(context.Background(), runner, input, mrtreeLayoutJSON, oracle.DefaultTolerance); err != nil {
		t.Fatal(err)
	}
}

func TestFrozenOracleTreeOptionsAndComponents(t *testing.T) {
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Skip(err)
	}
	fixtures := map[string]string{
		"deep variable-size tree":       `{"id":"root","layoutOptions":{"elk.algorithm":"mrtree","elk.padding":"[top=7,left=11,bottom=13,right=17]","elk.spacing.nodeNode":9},"children":[{"id":"a","width":23,"height":17},{"id":"b","width":31,"height":11},{"id":"c","width":13,"height":29},{"id":"d","width":41,"height":7},{"id":"e","width":19,"height":21}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"ac","sources":["a"],"targets":["c"]},{"id":"bd","sources":["b"],"targets":["d"]},{"id":"be","sources":["b"],"targets":["e"]}]}`,
		"disconnected priority packing": `{"id":"root","layoutOptions":{"elk.algorithm":"mrtree","elk.spacing.nodeNode":12},"children":[{"id":"a","width":20,"height":20,"layoutOptions":{"elk.priority":1}},{"id":"b","width":30,"height":10},{"id":"c","width":15,"height":25,"layoutOptions":{"elk.priority":9}},{"id":"d","width":22,"height":18},{"id":"e","width":11,"height":13}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"cd","sources":["c"],"targets":["d"]}]}`,
		"breadth-first treeification":   `{"id":"root","layoutOptions":{"elk.algorithm":"mrtree","elk.mrtree.searchOrder":"BFS"},"children":[{"id":"a","width":20,"height":20},{"id":"b","width":20,"height":20},{"id":"c","width":20,"height":20},{"id":"d","width":20,"height":20}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"ac","sources":["a"],"targets":["c"]},{"id":"bd","sources":["b"],"targets":["d"]}]}`,
	}
	for name, input := range fixtures {
		t.Run(name, func(t *testing.T) {
			if err := oracle.Differential(context.Background(), runner, []byte(input), mrtreeLayoutJSON, oracle.DefaultTolerance); err != nil {
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
