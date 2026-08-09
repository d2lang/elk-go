// Copyright (c) 2017 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package spore_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"testing"

	"github.com/d2lang/elk-go/graphjson"
	"github.com/d2lang/elk-go/internal/basic"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/oracle"
	"github.com/d2lang/elk-go/internal/spore"
)

func sporeLayoutJSON(input []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()
	var value graphjson.Graph
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	options := core.NewOptionRegistry()
	basic.RegisterOptions(options)
	spore.RegisterOptions(options)
	algorithms := core.NewAlgorithmRegistry()
	if err := basic.Register(algorithms); err != nil {
		return nil, err
	}
	if err := spore.Register(algorithms); err != nil {
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

func TestFrozenELKJS082SporeCompaction(t *testing.T) {
	input := []byte(`{"id":"root","x":0,"y":0,"width":0,"height":0,"layoutOptions":{"elk.algorithm":"sporeCompaction","elk.padding":"[top=8,left=8,bottom=8,right=8]","elk.spacing.nodeNode":8,"elk.processingOrder.treeConstruction":"MINIMUM_SPANNING_TREE","elk.processingOrder.spanningTreeCostFunction":"CIRCLE_UNDERLAP","elk.processingOrder.rootSelection":"FIXED","elk.processingOrder.preferredRoot":"a","elk.compaction.orthogonal":false},"children":[{"id":"a","x":0,"y":0,"width":10,"height":10},{"id":"b","x":100,"y":0,"width":10,"height":10}],"edges":[{"id":"e","sources":["a"],"targets":["b"]}]}`)
	want := []byte(`{"id":"root","x":0,"y":0,"width":44,"height":26,"layoutOptions":{"elk.algorithm":"sporeCompaction","elk.padding":"[top=8,left=8,bottom=8,right=8]","elk.spacing.nodeNode":8,"elk.processingOrder.treeConstruction":"MINIMUM_SPANNING_TREE","elk.processingOrder.spanningTreeCostFunction":"CIRCLE_UNDERLAP","elk.processingOrder.rootSelection":"FIXED","elk.processingOrder.preferredRoot":"a","elk.compaction.orthogonal":false},"children":[{"id":"a","x":8,"y":8,"width":10,"height":10},{"id":"b","x":26,"y":8,"width":10,"height":10}],"edges":[{"id":"e","sources":["a"],"targets":["b"],"sections":[{"id":"e_s0","startPoint":{"x":18,"y":13},"endPoint":{"x":26,"y":13}}],"container":"root"}]}`)
	got, err := sporeLayoutJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := oracle.CompareJSON(want, got, oracle.DefaultTolerance); err != nil {
		t.Fatal(err)
	}
}

func TestFrozenELKJS082SporeOverlap(t *testing.T) {
	input := []byte(`{"id":"root","x":0,"y":0,"width":120,"height":100,"layoutOptions":{"elk.algorithm":"sporeOverlap","elk.padding":"[top=8,left=8,bottom=8,right=8]","elk.spacing.nodeNode":8,"elk.overlapRemoval.maxIterations":64,"elk.overlapRemoval.runScanline":true},"children":[{"id":"a","x":0,"y":0,"width":20,"height":20},{"id":"b","x":10,"y":5,"width":30,"height":10},{"id":"c","x":60,"y":40,"width":15,"height":15}],"edges":[{"id":"e","sources":["a"],"targets":["b"]}]}`)
	want := []byte(`{"id":"root","x":0,"y":0,"width":109,"height":71,"layoutOptions":{"elk.algorithm":"sporeOverlap","elk.padding":"[top=8,left=8,bottom=8,right=8]","elk.spacing.nodeNode":8,"elk.overlapRemoval.maxIterations":64,"elk.overlapRemoval.runScanline":true},"children":[{"id":"a","x":8,"y":8,"width":20,"height":20},{"id":"b","x":36,"y":13,"width":30,"height":10},{"id":"c","x":86,"y":48,"width":15,"height":15}],"edges":[{"id":"e","sources":["a"],"targets":["b"],"sections":[{"id":"e_s0","startPoint":{"x":28,"y":18},"endPoint":{"x":36,"y":18}}],"container":"root"}]}`)
	got, err := sporeLayoutJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := oracle.CompareJSON(want, got, oracle.DefaultTolerance); err != nil {
		t.Fatal(err)
	}
}

func TestLiveELKJS082SporeParity(t *testing.T) {
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Skip(err)
	}
	fixtures := []string{
		`{"id":"root","width":200,"height":150,"layoutOptions":{"elk.algorithm":"sporeCompaction","elk.processingOrder.treeConstruction":"MAXIMUM_SPANNING_TREE","elk.processingOrder.spanningTreeCostFunction":"RECTANGLE_UNDERLAP","elk.processingOrder.rootSelection":"FIXED","elk.processingOrder.preferredRoot":"b","elk.compaction.orthogonal":true,"elk.spacing.nodeNode":9,"elk.padding":"[top=3,left=5,bottom=7,right=11]"},"children":[{"id":"a","x":5,"y":10,"width":20,"height":30},{"id":"b","x":90,"y":15,"width":35,"height":15},{"id":"c","x":35,"y":100,"width":15,"height":25},{"id":"d","x":150,"y":80,"width":25,"height":20}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"bc","sources":["b"],"targets":["c"]}]}`,
		`{"id":"root","width":120,"height":100,"layoutOptions":{"elk.algorithm":"sporeOverlap","elk.overlapRemoval.runScanline":true,"elk.overlapRemoval.maxIterations":12,"elk.spacing.nodeNode":6,"elk.padding":"[top=4,left=7,bottom=9,right=3]"},"children":[{"id":"a","x":0,"y":0,"width":20,"height":20},{"id":"b","x":10,"y":5,"width":30,"height":10},{"id":"c","x":60,"y":40,"width":15,"height":15}],"edges":[{"id":"e","sources":["a"],"targets":["b"]}]}`,
		`{"id":"root","width":180,"height":120,"layoutOptions":{"elk.algorithm":"sporeCompaction","elk.underlyingLayoutAlgorithm":"fixed","elk.spacing.nodeNode":4,"elk.padding":"[top=2,left=3,bottom=5,right=7]"},"children":[{"id":"a","width":20,"height":15,"layoutOptions":{"elk.position":"(11,13)"},"labels":[{"id":"al","x":-9,"y":2,"width":7,"height":6}]},{"id":"b","width":25,"height":17,"layoutOptions":{"elk.position":"(109,71)"},"ports":[{"id":"bp","x":-4,"y":5,"width":4,"height":5,"labels":[{"id":"bpl","x":-8,"y":1,"width":7,"height":3}]}]}],"edges":[{"id":"e","sources":["a"],"targets":["bp"]}]}`,
		`{"id":"root","width":180,"height":180,"layoutOptions":{"elk.algorithm":"sporeCompaction","elk.processingOrder.spanningTreeCostFunction":"CENTER_DISTANCE","elk.processingOrder.rootSelection":"FIXED","elk.processingOrder.preferredRoot":"a"},"children":[{"id":"a","x":0,"y":0,"width":20,"height":20},{"id":"b","x":100,"y":0,"width":20,"height":20},{"id":"c","x":100,"y":100,"width":20,"height":20},{"id":"d","x":0,"y":100,"width":20,"height":20}]}`,
		`{"id":"root","width":240,"height":80,"layoutOptions":{"elk.algorithm":"sporeCompaction","elk.processingOrder.spanningTreeCostFunction":"CENTER_DISTANCE","elk.processingOrder.rootSelection":"FIXED","elk.processingOrder.preferredRoot":"c"},"children":[{"id":"a","x":0,"y":20,"width":20,"height":20},{"id":"b","x":60,"y":20,"width":20,"height":20},{"id":"c","x":120,"y":20,"width":20,"height":20},{"id":"d","x":180,"y":20,"width":20,"height":20}]}`,
	}
	for _, input := range fixtures {
		if err := oracle.Differential(context.Background(), runner, []byte(input), sporeLayoutJSON, oracle.DefaultTolerance); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLiveELKJS082SporeOptionMatrix(t *testing.T) {
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Skip(err)
	}
	costs := []string{
		"CENTER_DISTANCE",
		"CIRCLE_UNDERLAP",
		"RECTANGLE_UNDERLAP",
		"INVERTED_OVERLAP",
		"MINIMUM_ROOT_DISTANCE",
	}
	for _, tree := range []string{"MINIMUM_SPANNING_TREE", "MAXIMUM_SPANNING_TREE"} {
		for _, cost := range costs {
			for _, orthogonal := range []bool{false, true} {
				name := fmt.Sprintf("%s/%s/orthogonal=%t", tree, cost, orthogonal)
				t.Run(name, func(t *testing.T) {
					input := []byte(fmt.Sprintf(`{
						"id":"root","x":3,"y":5,"width":240,"height":190,
						"layoutOptions":{
							"elk.algorithm":"sporeCompaction",
							"elk.processingOrder.treeConstruction":%q,
							"elk.processingOrder.spanningTreeCostFunction":%q,
							"elk.processingOrder.rootSelection":"FIXED",
							"elk.processingOrder.preferredRoot":"c",
							"elk.compaction.orthogonal":%t,
							"elk.spacing.nodeNode":7,
							"elk.padding":"[top=2,left=3,bottom=5,right=7]"
						},
						"children":[
							{"id":"a","x":7,"y":11,"width":21,"height":17},
							{"id":"b","x":111,"y":19,"width":33,"height":12},
							{"id":"c","x":48,"y":91,"width":16,"height":29},
							{"id":"d","x":178,"y":73,"width":27,"height":22},
							{"id":"e","x":89,"y":151,"width":19,"height":14},
							{"id":"f","x":208,"y":143,"width":12,"height":31}
						],
						"edges":[
							{"id":"ab","sources":["a"],"targets":["b"]},
							{"id":"bc","sources":["b"],"targets":["c"]},
							{"id":"de","sources":["d"],"targets":["e"]}
						]
					}`, tree, cost, orthogonal))
					if err := oracle.Differential(context.Background(), runner, input, sporeLayoutJSON, oracle.DefaultTolerance); err != nil {
						t.Fatalf("%v\ninput: %s", err, input)
					}
				})
			}
		}
	}

	for _, scanline := range []bool{false, true} {
		for _, iterations := range []int{1, 4, 12} {
			name := fmt.Sprintf("overlap/scanline=%t/iterations=%d", scanline, iterations)
			t.Run(name, func(t *testing.T) {
				input := []byte(fmt.Sprintf(`{
					"id":"root","x":2,"y":3,"width":190,"height":150,
					"layoutOptions":{
						"elk.algorithm":"sporeOverlap",
						"elk.overlapRemoval.runScanline":%t,
						"elk.overlapRemoval.maxIterations":%d,
						"elk.spacing.nodeNode":5,
						"elk.padding":"[top=3,left=7,bottom=11,right=13]"
					},
					"children":[
						{"id":"a","x":4,"y":8,"width":60,"height":25},
						{"id":"b","x":33,"y":17,"width":38,"height":42},
						{"id":"c","x":62,"y":45,"width":51,"height":19},
						{"id":"d","x":99,"y":31,"width":30,"height":53},
						{"id":"e","x":141,"y":96,"width":22,"height":28}
					]
				}`, scanline, iterations))
				if err := oracle.Differential(context.Background(), runner, input, sporeLayoutJSON, oracle.DefaultTolerance); err != nil {
					t.Fatalf("%v\ninput: %s", err, input)
				}
			})
		}
	}
}

func TestCoincidentOverlapInputIsSeparatedAndFinite(t *testing.T) {
	input := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"sporeOverlap"},"children":[{"id":"a","x":0,"y":0,"width":20,"height":20},{"id":"b","x":0,"y":0,"width":20,"height":20},{"id":"c","x":0,"y":0,"width":20,"height":20},{"id":"d","x":0,"y":0,"width":20,"height":20}]}`)
	got, err := sporeLayoutJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	var output graphjson.Graph
	if err := json.Unmarshal(got, &output); err != nil {
		t.Fatal(err)
	}
	for index, child := range output.Children {
		if child.X == nil || child.Y == nil || math.IsNaN(*child.X) || math.IsNaN(*child.Y) || math.IsInf(*child.X, 0) || math.IsInf(*child.Y, 0) {
			t.Fatalf("child %d has a non-finite position", index)
		}
		for _, other := range output.Children[index+1:] {
			if *child.X < *other.X+*other.Width && *child.X+*child.Width > *other.X &&
				*child.Y < *other.Y+*other.Height && *child.Y+*child.Height > *other.Y {
				t.Fatalf("children %v and %v still overlap", child.ID, other.ID)
			}
		}
	}
}

func TestProvidersAreConcurrent(t *testing.T) {
	input := []byte(`{"id":"root","width":160,"height":120,"layoutOptions":{"elk.algorithm":"sporeCompaction"},"children":[{"id":"a","x":2,"y":3,"width":20,"height":20},{"id":"b","x":90,"y":7,"width":30,"height":15},{"id":"c","x":35,"y":78,"width":15,"height":27}]}`)
	const workers = 24
	var wait sync.WaitGroup
	errors := make(chan error, workers)
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := sporeLayoutJSON(input)
			errors <- err
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
}
