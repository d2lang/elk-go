// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package p4nodes_test

import (
	"testing"

	elk "github.com/d2lang/elk-go"
	"github.com/d2lang/elk-go/internal/layered/p4nodes"
)

// denseInput is a minimized random graph (randomized
// test, seed 59): a nested graph with LAST_SEPARATE / LAST layer constraints, an edge
// into a port of a sibling and a labelled edge between containers.
const denseInput = `{"children":[{"children":[{"id":"n1"},{"children":[{"id":"n3","layoutOptions":{"elk.layered.layering.layerConstraint":"LAST_SEPARATE"}},{"id":"n4","layoutOptions":{"elk.layered.layering.layerConstraint":"LAST"}}],"id":"n2"},{"id":"n5","ports":[{"id":"n5.p1"}]}],"id":"n0"}],"edges":[{"id":"e1","sources":["n3"],"targets":["n5.p1"]},{"id":"e5","labels":[{"text":"l"}],"sources":["n2"],"targets":["n1"]}],"id":"root","layoutOptions":{"elk.hierarchyHandling":"INCLUDE_CHILDREN"}}`

// denseWant is what the pinned elk-go (8b285820520e, the version d2
// pins) returns for denseInput, in fresh processes every time.
const denseWant = `{"children":[{"children":[{"height":0,"id":"n1","width":0,"x":81,"y":32},{"children":[{"height":0,"id":"n3","layoutOptions":{"elk.layered.layering.layerConstraint":"LAST_SEPARATE"},"width":0,"x":12,"y":22},{"height":0,"id":"n4","layoutOptions":{"elk.layered.layering.layerConstraint":"LAST"},"width":0,"x":0,"y":0}],"height":34,"id":"n2","width":24,"x":12,"y":20},{"height":0,"id":"n5","ports":[{"height":0,"id":"n5.p1","width":0,"x":0,"y":0}],"width":0,"x":61,"y":12}],"height":66,"id":"n0","width":93,"x":12,"y":12}],"edges":[{"container":"n0","id":"e1","sections":[{"bendPoints":[{"x":51,"y":42},{"x":51,"y":12}],"endPoint":{"x":61,"y":12},"id":"e1_s0","incomingShape":"n3","outgoingShape":"n5.p1","startPoint":{"x":24,"y":42}}],"sources":["n3"],"targets":["n5.p1"]},{"container":"n0","id":"e5","labels":[{"height":0,"text":"l","width":0,"x":61,"y":35}],"sections":[{"endPoint":{"x":81,"y":32},"id":"e5_s0","incomingShape":"n2","outgoingShape":"n1","startPoint":{"x":36,"y":32}}],"sources":["n2"],"targets":["n1"]}],"height":90,"id":"root","layoutOptions":{"elk.hierarchyHandling":"INCLUDE_CHILDREN"},"width":117,"x":0,"y":0}`

// TestDenseBKDuplicateLayerEntryMatchesOriginal lays the input out with each
// combination of the Brandes-Koepf fast paths and compares with the pinned
// version's output.
func TestDenseBKDuplicateLayerEntryMatchesOriginal(t *testing.T) {
	for _, mode := range []struct {
		name          string
		dense, single bool
	}{
		{"dense and single-node paths on (production)", false, false},
		{"dense off", true, false},
		{"single-node shortcut off", false, true},
		{"both off (the original code)", true, true},
	} {
		restore := p4nodes.SetBKFastPathsDisabled(mode.dense, mode.single)
		out, err := elk.LayoutJSON([]byte(denseInput))
		restore()
		if err != nil {
			t.Fatalf("%s: %v", mode.name, err)
		}
		if string(out) != denseWant {
			t.Errorf("%s: differs from the pinned elk-go:\n got %s\nwant %s", mode.name, out, denseWant)
		} else {
			t.Logf("%s: same as the pinned elk-go", mode.name)
		}
	}
}
