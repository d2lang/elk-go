// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package elk

import (
	"bytes"
	"encoding/json"
	"testing"
)

// rebuildGraph copies a decoded graph into fresh values through the exported
// API only, as a Go caller builds one, keeping the decoded member order of
// every option map with SetLayoutOptionKeys.
func rebuildGraph(source *Node) *Node {
	if source == nil {
		return nil
	}
	node := &Node{
		ID:                 source.ID,
		LayoutOptions:      copyOptions(source.LayoutOptions),
		IndividualSpacings: copyOptions(source.IndividualSpacings),
		X:                  copyFloat(source.X),
		Y:                  copyFloat(source.Y),
		Width:              copyFloat(source.Width),
		Height:             copyFloat(source.Height),
		AdditionalFields:   source.AdditionalFields,
	}
	node.SetLayoutOptionKeys(source.LayoutOptionKeys())
	node.SetIndividualSpacingKeys(source.IndividualSpacingKeys())
	if source.Labels != nil {
		node.Labels = make([]*Label, len(source.Labels))
		for i, label := range source.Labels {
			node.Labels[i] = rebuildLabel(label)
		}
	}
	if source.Ports != nil {
		node.Ports = make([]*Port, len(source.Ports))
		for i, port := range source.Ports {
			if port == nil {
				continue
			}
			copied := &Port{
				ID: port.ID, LayoutOptions: copyOptions(port.LayoutOptions),
				X: copyFloat(port.X), Y: copyFloat(port.Y), Width: copyFloat(port.Width), Height: copyFloat(port.Height),
				AdditionalFields: port.AdditionalFields,
			}
			copied.SetLayoutOptionKeys(port.LayoutOptionKeys())
			if port.Labels != nil {
				copied.Labels = make([]*Label, len(port.Labels))
				for j, label := range port.Labels {
					copied.Labels[j] = rebuildLabel(label)
				}
			}
			node.Ports[i] = copied
		}
	}
	if source.Children != nil {
		node.Children = make([]*Node, len(source.Children))
		for i, child := range source.Children {
			node.Children[i] = rebuildGraph(child)
		}
	}
	if source.Edges != nil {
		node.Edges = make([]*Edge, len(source.Edges))
		for i, edge := range source.Edges {
			if edge == nil {
				continue
			}
			copied := *edge
			copied.LayoutOptions = copyOptions(edge.LayoutOptions)
			copied.SetLayoutOptionKeys(edge.LayoutOptionKeys())
			copied.Sources = append([]ID(nil), edge.Sources...)
			copied.Targets = append([]ID(nil), edge.Targets...)
			if edge.Sources != nil && len(edge.Sources) == 0 {
				copied.Sources = []ID{}
			}
			if edge.Targets != nil && len(edge.Targets) == 0 {
				copied.Targets = []ID{}
			}
			if edge.Labels != nil {
				copied.Labels = make([]*Label, len(edge.Labels))
				for j, label := range edge.Labels {
					copied.Labels[j] = rebuildLabel(label)
				}
			}
			node.Edges[i] = &copied
		}
	}
	return node
}

func rebuildLabel(source *Label) *Label {
	if source == nil {
		return nil
	}
	label := &Label{
		ID: source.ID, Text: source.Text, LayoutOptions: copyOptions(source.LayoutOptions),
		X: copyFloat(source.X), Y: copyFloat(source.Y), Width: copyFloat(source.Width), Height: copyFloat(source.Height),
		AdditionalFields: source.AdditionalFields,
	}
	label.SetLayoutOptionKeys(source.LayoutOptionKeys())
	if source.Labels != nil {
		label.Labels = make([]*Label, len(source.Labels))
		for i, nested := range source.Labels {
			label.Labels[i] = rebuildLabel(nested)
		}
	}
	return label
}

func copyOptions(options LayoutOptions) LayoutOptions {
	if options == nil {
		return nil
	}
	copied := make(LayoutOptions, len(options))
	for key, value := range options {
		copied[key] = value
	}
	return copied
}

func copyFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	return Float64(*value)
}

func decodeLikeLayoutJSON(tb testing.TB, input []byte) *Node {
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()
	var graph Node
	if err := decoder.Decode(&graph); err != nil {
		tb.Fatal(err)
	}
	return &graph
}

// TestSetLayoutOptionKeysOrdersImport checks that an explicit member order
// changes the import order exactly as the decoded JSON order does.
func TestSetLayoutOptionKeysOrdersImport(t *testing.T) {
	const input = `{"id":"root","layoutOptions":{"spacing.nodeNode":"5","elk.spacing.nodeNode":"7"},` +
		`"children":[{"id":"a","width":10,"height":10},{"id":"b","width":10,"height":10}]}`
	want, err := LayoutJSON([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	graph := rebuildGraph(decodeLikeLayoutJSON(t, []byte(input)))
	if keys := graph.LayoutOptionKeys(); len(keys) != 2 || keys[0] != "spacing.nodeNode" {
		t.Fatalf("decoded order %v", keys)
	}
	if err := Layout(graph); err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("typed %s\njson  %s", got, want)
	}
}
