// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package elk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestLayoutJSONValidatesGraphReferences(t *testing.T) {
	_, err := LayoutJSON([]byte(`{"id":"root","children":[{"id":"a"}],"edges":[{"id":"e","sources":["missing"],"targets":["a"]}]}`))
	if err == nil || !strings.Contains(err.Error(), "referenced shape does not exist") {
		t.Fatalf("error = %v", err)
	}
}

func TestLayoutJSONRejectsTrailingValue(t *testing.T) {
	_, err := LayoutJSON([]byte(`{"id":"root"} {"trailing":true}`))
	if err == nil || !strings.Contains(err.Error(), "unexpected trailing JSON value") {
		t.Fatalf("error = %v", err)
	}
}

func TestLayoutEmptyGraph(t *testing.T) {
	graph := &Graph{ID: StringID("root"), Children: []*Node{}, Edges: []*Edge{}}
	if err := Layout(graph); err != nil {
		t.Fatal(err)
	}
	if graph.X == nil || graph.Y == nil || graph.Width == nil || graph.Height == nil {
		t.Fatal("layout coordinates were not transferred")
	}
}

func TestLayoutJSONConcurrentDeterminism(t *testing.T) {
	input := []byte(`{
		"id":"root",
		"layoutOptions":{
			"elk.algorithm":"layered",
			"elk.direction":"DOWN",
			"elk.hierarchyHandling":"INCLUDE_CHILDREN",
			"elk.layered.considerModelOrder.strategy":"NODES_AND_EDGES",
			"elk.layered.cycleBreaking.strategy":"GREEDY_MODEL_ORDER"
		},
		"children":[
			{"id":"a","width":60,"height":30},
			{"id":"b","width":70,"height":35},
			{"id":"c","width":50,"height":25}
		],
		"edges":[
			{"id":"e1","sources":["a"],"targets":["b"]},
			{"id":"e2","sources":["a"],"targets":["c"]},
			{"id":"e3","sources":["c"],"targets":["a"]}
		]
	}`)
	baseline, err := LayoutJSON(input)
	if err != nil {
		t.Fatal(err)
	}

	const workers = 32
	errors := make(chan error, workers)
	var wait sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		worker := worker
		wait.Add(1)
		go func() {
			defer wait.Done()
			for iteration := 0; iteration < 4; iteration++ {
				output, err := LayoutJSON(input)
				if err != nil {
					errors <- fmt.Errorf("worker %d iteration %d: %w", worker, iteration, err)
					return
				}
				if !bytes.Equal(output, baseline) {
					errors <- fmt.Errorf("worker %d iteration %d produced nondeterministic output", worker, iteration)
					return
				}
			}
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

func TestLayoutJSONResolvesAliasesInJSONMemberOrder(t *testing.T) {
	const canonical = "org.eclipse.elk.layered.spacing.nodeNodeBetweenLayers"
	const legacy = "spacing.nodeNodeBetweenLayers"
	tests := []struct {
		name        string
		firstKey    string
		firstValue  int
		secondKey   string
		secondValue int
		wantX       float64
	}{
		{"legacy-last", canonical, 20, legacy, 100, 122},
		{"canonical-last", legacy, 100, canonical, 20, 42},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			input := []byte(fmt.Sprintf(`{
				"id":"root",
				"layoutOptions":{"elk.algorithm":"layered",%q:%q,%q:%q},
				"children":[{"id":"a","width":10,"height":10},{"id":"b","width":10,"height":10}],
				"edges":[{"id":"e","sources":["a"],"targets":["b"]}]
			}`, test.firstKey, fmt.Sprint(test.firstValue), test.secondKey, fmt.Sprint(test.secondValue)))
			baseline, err := LayoutJSON(input)
			if err != nil {
				t.Fatal(err)
			}
			if got, err := outputNodeX(baseline, "b"); err != nil || got != test.wantX {
				t.Fatalf("b.x = %v, %v; want %v", got, err, test.wantX)
			}

			const workers = 16
			errors := make(chan error, workers)
			var wait sync.WaitGroup
			for worker := 0; worker < workers; worker++ {
				wait.Add(1)
				go func() {
					defer wait.Done()
					for iteration := 0; iteration < 8; iteration++ {
						output, err := LayoutJSON(input)
						if err != nil {
							errors <- err
							return
						}
						if !bytes.Equal(output, baseline) {
							errors <- fmt.Errorf("iteration %d produced nondeterministic output", iteration)
							return
						}
					}
				}()
			}
			wait.Wait()
			close(errors)
			for err := range errors {
				t.Error(err)
			}
		})
	}
}

func TestLayoutProgrammaticAliasesPreferCanonicalSpelling(t *testing.T) {
	const canonical = "org.eclipse.elk.layered.spacing.nodeNodeBetweenLayers"
	const legacy = "spacing.nodeNodeBetweenLayers"
	for iteration := 0; iteration < 128; iteration++ {
		graph := &Graph{
			ID: StringID("root"),
			LayoutOptions: LayoutOptions{
				"elk.algorithm": "layered",
				legacy:          "100",
				canonical:       "20",
			},
			Children: []*Node{
				{ID: StringID("a"), Width: Float64(10), Height: Float64(10)},
				{ID: StringID("b"), Width: Float64(10), Height: Float64(10)},
			},
			Edges: []*Edge{{
				ID:      StringID("e"),
				Sources: []ID{StringID("a")},
				Targets: []ID{StringID("b")},
			}},
		}
		if err := Layout(graph); err != nil {
			t.Fatal(err)
		}
		if graph.Children[1].X == nil || *graph.Children[1].X != 42 {
			t.Fatalf("iteration %d: b.x = %#v, want canonical spacing result 42", iteration, graph.Children[1].X)
		}
	}
}

func TestLayoutReplacesStaleEdgeSections(t *testing.T) {
	input := []byte(`{
		"id":"root",
		"layoutOptions":{"elk.algorithm":"layered"},
		"children":[{"id":"a","width":10,"height":10},{"id":"b","width":10,"height":10}],
		"edges":[{"id":"e","sources":["a"],"targets":["b"],"sections":[
			{"id":"old1","startPoint":{"x":1,"y":2},"endPoint":{"x":3,"y":4},"marker":"keep"},
			{"id":"old2","startPoint":{"x":5,"y":6},"endPoint":{"x":7,"y":8},"marker":"drop"}
		]}]
	}`)
	output, err := LayoutJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	var graph Graph
	if err := json.Unmarshal(output, &graph); err != nil {
		t.Fatal(err)
	}
	if got := len(graph.Edges[0].Sections); got != 1 {
		t.Fatalf("section count = %d, want 1: %s", got, output)
	}
	section := graph.Edges[0].Sections[0]
	if section.ID.String() != "old1" {
		t.Fatalf("section id = %q, want old1", section.ID.String())
	}
	if got := string(section.AdditionalFields["marker"]); got != `"keep"` {
		t.Fatalf("first section extension = %s, want preserved marker", got)
	}
}

func TestLayoutPlacesLayeredNodeLabels(t *testing.T) {
	output, err := LayoutJSON([]byte(`{
		"id":"root",
		"layoutOptions":{"elk.algorithm":"layered"},
		"children":[{
			"id":"a","width":40,"height":40,
			"layoutOptions":{"org.eclipse.elk.nodeLabels.placement":"INSIDE H_CENTER V_CENTER"},
			"labels":[{"id":"l","text":"x","width":14,"height":6}]
		}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	var graph Graph
	if err := json.Unmarshal(output, &graph); err != nil {
		t.Fatal(err)
	}
	label := graph.Children[0].Labels[0]
	if label.X == nil || label.Y == nil || *label.X != 13 || *label.Y != 17 {
		t.Fatalf("node label position = (%v, %v), want (13, 17): %s", label.X, label.Y, output)
	}
}

func TestLayoutPlacesLayeredPortLabels(t *testing.T) {
	output, err := LayoutJSON([]byte(`{
		"id":"root",
		"layoutOptions":{"elk.algorithm":"layered"},
		"children":[{
			"id":"a","width":100,"height":100,
			"layoutOptions":{"elk.portConstraints":"FIXED_SIDE"},
			"ports":[{
				"id":"p","width":10,"height":10,
				"layoutOptions":{"elk.port.side":"EAST"},
				"labels":[
					{"id":"l1","text":"one","width":20,"height":10},
					{"id":"l2","text":"two","width":30,"height":10}
				]
			}]
		}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	var graph Graph
	if err := json.Unmarshal(output, &graph); err != nil {
		t.Fatal(err)
	}
	port := graph.Children[0].Ports[0]
	if port.X == nil || port.Y == nil || *port.X != 100 || *port.Y != 45 {
		t.Fatalf("port position = (%v, %v), want (100, 45): %s", port.X, port.Y, output)
	}
	wantY := []float64{11, 21}
	for index, label := range port.Labels {
		if label.X == nil || label.Y == nil || *label.X != 11 || *label.Y != wantY[index] {
			t.Fatalf("port label %d position = (%v, %v), want (11, %v): %s",
				index, label.X, label.Y, wantY[index], output)
		}
	}
}

func outputNodeX(output []byte, identifier string) (float64, error) {
	var graph Graph
	if err := json.Unmarshal(output, &graph); err != nil {
		return 0, err
	}
	for _, child := range graph.Children {
		if child != nil && child.ID.String() == identifier && child.X != nil {
			return *child.X, nil
		}
	}
	return 0, fmt.Errorf("node %q has no x coordinate", identifier)
}
