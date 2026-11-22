// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package elk

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/graph"
)

// requestBuilder adds the elements of a decoded Graph to a Request in import
// order, remembering which Graph element each reference stands for.
type requestBuilder struct {
	t     testing.TB
	req   *Request
	nodes []*Node
	edges []*Edge
}

func (b *requestBuilder) options(options LayoutOptions, keys []string) *Options {
	if options == nil {
		return nil
	}
	values := make([]any, len(keys))
	for i, key := range keys {
		values[i] = options[key]
	}
	if len(keys) != len(options) {
		b.t.Fatalf("options %v listed as %v", options, keys)
	}
	return b.req.PrepareOptions(keys, values)
}

func requestShape(t testing.TB, what string, x, y, width, height *float64) (float64, float64, float64, float64) {
	if x == nil || y == nil || width == nil || height == nil {
		t.Fatalf("%s without a complete shape", what)
	}
	return *x, *y, *width, *height
}

func requestID(t testing.TB, id ID) string {
	if !id.IsValid() || id.IsNumber() {
		t.Fatalf("ID %v is not a string", id)
	}
	return id.String()
}

func (b *requestBuilder) node(ref NodeRef, n *Node) {
	if ref < 0 {
		return // the request failed
	}
	for len(b.nodes) <= int(ref) {
		b.nodes = append(b.nodes, nil)
	}
	b.nodes[ref] = n
	if n.IndividualSpacings != nil {
		b.t.Fatal("individual spacings")
	}
	for _, port := range n.Ports {
		if port == nil {
			continue
		}
		x, y, w, h := requestShape(b.t, "port", port.X, port.Y, port.Width, port.Height)
		portRef := b.req.Port(ref, requestID(b.t, port.ID), x, y, w, h, b.options(port.LayoutOptions, port.LayoutOptionKeys()))
		for _, label := range port.Labels {
			if label != nil {
				b.label(label, func(text string, x, y, w, h float64, options *Options) {
					b.req.PortLabel(portRef, text, x, y, w, h, options)
				})
			}
		}
	}
	for _, label := range n.Labels {
		if label != nil {
			b.label(label, func(text string, x, y, w, h float64, options *Options) {
				b.req.NodeLabel(ref, text, x, y, w, h, options)
			})
		}
	}
	for _, child := range n.Children {
		if child == nil {
			continue
		}
		x, y, w, h := requestShape(b.t, "node", child.X, child.Y, child.Width, child.Height)
		childRef := b.req.Node(ref, requestID(b.t, child.ID), x, y, w, h, b.options(child.LayoutOptions, child.LayoutOptionKeys()))
		b.node(childRef, child)
	}
}

func (b *requestBuilder) label(label *Label, add func(string, float64, float64, float64, float64, *Options)) {
	if label.Text == nil || label.ID != nil || label.Labels != nil {
		b.t.Fatal("label without text, or with an ID or labels")
	}
	x, y, w, h := requestShape(b.t, "label", label.X, label.Y, label.Width, label.Height)
	add(*label.Text, x, y, w, h, b.options(label.LayoutOptions, label.LayoutOptionKeys()))
}

func (b *requestBuilder) edgesOf(ref NodeRef, n *Node) {
	for _, edge := range n.Edges {
		if edge == nil {
			continue
		}
		if edge.Sections != nil || edge.Source != nil || edge.Target != nil {
			b.t.Fatal("edge with sections or in primitive form")
		}
		ids := func(values []ID) []string {
			if values == nil {
				return nil
			}
			out := make([]string, len(values))
			for i, value := range values {
				out[i] = requestID(b.t, value)
			}
			return out
		}
		edgeRef := b.req.Edge(ref, requestID(b.t, edge.ID), ids(edge.Sources), ids(edge.Targets), b.options(edge.LayoutOptions, edge.LayoutOptionKeys()))
		for len(b.edges) <= int(edgeRef) {
			b.edges = append(b.edges, nil)
		}
		if edgeRef >= 0 {
			b.edges[edgeRef] = edge
		}
		for _, label := range edge.Labels {
			if label != nil {
				b.label(label, func(text string, x, y, w, h float64, options *Options) {
					b.req.EdgeLabel(edgeRef, text, x, y, w, h, options)
				})
			}
		}
	}
	childIndex := 0
	for _, child := range n.Children {
		if child == nil {
			continue
		}
		// Children were added in order right after their parent's
		// earlier subtrees: find the reference of this one.
		for childIndex < len(b.nodes) && b.nodes[childIndex] != child {
			childIndex++
		}
		b.edgesOf(NodeRef(childIndex), child)
	}
}

func buildRequest(t testing.TB, g *Node) *requestBuilder {
	b := &requestBuilder{t: t, req: NewRequest(1, 0, 0, 0)}
	if g.X != nil || g.Y != nil || g.Width != nil || g.Height != nil {
		t.Fatal("root with a shape")
	}
	root := b.req.Root(requestID(t, g.ID), b.options(g.LayoutOptions, g.LayoutOptionKeys()))
	b.node(root, g)
	b.edgesOf(root, g)
	return b
}

// sameModel compares models, not distinguishing empty property maps with and
// without storage (the request's codec recycles storage; see
// core.ReleaseCodec).
func sameModel(got, want *graph.Model) bool {
	normalize := func(model *graph.Model) {
		empty := func(properties *graph.PropertyMap) {
			if properties.Empty() {
				*properties = graph.PropertyMap{}
			}
		}
		for i := range model.Nodes {
			empty(&model.Nodes[i].Properties)
		}
		for i := range model.Ports {
			empty(&model.Ports[i].Properties)
		}
		for i := range model.Labels {
			empty(&model.Labels[i].Properties)
		}
		for i := range model.Edges {
			empty(&model.Edges[i].Properties)
		}
		for i := range model.Sections {
			empty(&model.Sections[i].Properties)
		}
	}
	normalize(got)
	normalize(want)
	return reflect.DeepEqual(got, want)
}

func sameFloat(a, b float64) bool { return math.Float64bits(a) == math.Float64bits(b) }

func samePoint(a, b Point) bool { return sameFloat(a.X, b.X) && sameFloat(a.Y, b.Y) }

func checkReadBack(t *testing.T, name string, b *requestBuilder) {
	for ref, node := range b.nodes {
		if node == nil {
			continue
		}
		x, y, w, h := b.req.NodeLayout(NodeRef(ref))
		if !sameFloat(x, *node.X) || !sameFloat(y, *node.Y) || !sameFloat(w, *node.Width) || !sameFloat(h, *node.Height) {
			t.Fatalf("%s: node %d read back as %v %v %v %v", name, ref, x, y, w, h)
		}
	}
	for ref, edge := range b.edges {
		if edge == nil {
			continue
		}
		n := b.req.Sections(EdgeRef(ref))
		if n == 0 {
			if edge.Sections != nil {
				t.Fatalf("%s: edge %d without sections read back", name, ref)
			}
		} else if len(edge.Sections) != n {
			t.Fatalf("%s: edge %d has %d sections, read back %d", name, ref, len(edge.Sections), n)
		}
		for i := 0; i < n; i++ {
			start, end, bends := b.req.Section(EdgeRef(ref), i)
			section := edge.Sections[i]
			if !samePoint(start, *section.StartPoint) || !samePoint(end, *section.EndPoint) {
				t.Fatalf("%s: edge %d section %d ends differ", name, ref, i)
			}
			if bends == 0 {
				if section.BendPoints != nil {
					t.Fatalf("%s: edge %d section %d bends", name, ref, i)
				}
				continue
			}
			if len(section.BendPoints) != bends {
				t.Fatalf("%s: edge %d section %d has %d bends, read back %d", name, ref, i, len(section.BendPoints), bends)
			}
			for j := 0; j < bends; j++ {
				if !samePoint(b.req.Bend(EdgeRef(ref), i, j), section.BendPoints[j]) {
					t.Fatalf("%s: edge %d section %d bend %d differs", name, ref, i, j)
				}
			}
		}
		container, ok := b.req.Container(EdgeRef(ref))
		switch {
		case ok && (edge.Container == nil || edge.Container.String() != b.nodes[container].ID.String()):
			t.Fatalf("%s: edge %d container read back as node %d", name, ref, container)
		case !ok && edge.Container != nil && edge.Container.IsValid() && edge.Container.String() != "":
			// d2 sends "" and keeps it when no container is written.
			t.Fatalf("%s: edge %d container %v not read back", name, ref, edge.Container)
		}
	}
}

// TestRequestErrorsMatchLayout checks that invalid requests and requests
// given out of order fail, with Layout's errors for invalid elements.
func TestRequestErrorsMatchLayout(t *testing.T) {
	for _, input := range []string{
		`{"id":"r","children":[{"id":"a","x":0,"y":0,"width":1,"height":1},{"id":"a","x":0,"y":0,"width":1,"height":1}]}`,
		`{"id":"r","children":[{"id":"a","x":0,"y":0,"width":1,"height":1,"ports":[{"id":"p","x":0,"y":0,"width":1,"height":1},{"id":"p","x":0,"y":0,"width":1,"height":1}]}]}`,
		`{"id":"r","children":[{"id":"a","x":0,"y":0,"width":1,"height":1}],"edges":[{"id":"e","sources":["a"],"targets":["missing"]}]}`,
		`{"id":"r","children":[{"id":"a","x":0,"y":0,"width":1,"height":1}],"edges":[{"id":"e","sources":["a"],"targets":[]}]}`,
		`{"id":"r","children":[{"id":"a","x":0,"y":0,"width":1,"height":1}],"edges":[{"id":"e"}]}`,
		`{"id":"r","children":[{"id":"r","x":0,"y":0,"width":1,"height":1}]}`,
	} {
		want := Layout(decodeLikeLayoutJSON(t, []byte(input)))
		if want == nil {
			t.Fatalf("%s: no error", input)
		}
		b := buildRequest(t, decodeLikeLayoutJSON(t, []byte(input)))
		got := b.req.Layout()
		b.req.Release()
		if got == nil || got.Error() != want.Error() {
			t.Fatalf("%s:\n got %v\nwant %v", input, got, want)
		}
	}
	misuse := []func(r *Request){
		func(r *Request) { r.Node(0, "a", 0, 0, 1, 1, nil) },
		func(r *Request) {
			r.Root("", nil)
			a := r.Node(0, "a", 0, 0, 1, 1, nil)
			r.Node(0, "b", 0, 0, 1, 1, nil)
			r.Port(a, "p", 0, 0, 1, 1, nil)
		},
		func(r *Request) {
			r.Root("", nil)
			a := r.Node(0, "a", 0, 0, 1, 1, nil)
			r.NodeLabel(a, "l", 0, 0, 1, 1, nil)
			r.Port(a, "p", 0, 0, 1, 1, nil)
		},
		func(r *Request) {
			r.Root("", nil)
			a := r.Node(0, "a", 0, 0, 1, 1, nil)
			r.Edge(0, "e", []string{"a"}, []string{"a"}, nil)
			r.Node(a, "b", 0, 0, 1, 1, nil)
		},
	}
	for i, use := range misuse {
		r := NewRequest(1, 0, 0, 0)
		use(r)
		err := r.Layout()
		r.Release()
		if err == nil || !strings.Contains(err.Error(), "import order") {
			t.Fatalf("misuse %d: %v", i, err)
		}
	}
}

// TestRequestSharedOptionsMatchImport gives several nodes one prepared set
// (applied element by element first, then copied from its map) and checks the
// model against Import's, for request-scoped and package-level sets.
func TestRequestSharedOptionsMatchImport(t *testing.T) {
	keys := []string{"elk.direction", "elk.padding", "elk.nodeSize.minimum", "elk.nodeSize.constraints",
		"elk.contentAlignment", "elk.spacing.nodeNode", "elk.layered.thoroughness", "elk.hierarchyHandling",
		"spacing.nodeNodeBetweenLayers", "elk.portConstraints"}
	values := []any{"DOWN", "[top=1,left=2,bottom=3,right=4]", "(5, 6)", "MINIMUM_SIZE", "H_CENTER V_CENTER",
		12.5, 8.0, "INCLUDE_CHILDREN", 70.0, "FIXED_SIDE"}
	options := LayoutOptions{}
	for i, key := range keys {
		options[key] = values[i]
	}
	graph := &Node{ID: StringID("root")}
	for i := 0; i < 4; i++ {
		child := &Node{ID: StringID("n" + string(rune('a'+i))), X: Float64(0), Y: Float64(0), Width: Float64(10), Height: Float64(10),
			LayoutOptions: copyOptions(options)}
		child.SetLayoutOptionKeys(keys)
		child.Children = []*Node{{ID: StringID("c" + string(rune('a'+i))), X: Float64(1), Y: Float64(1), Width: Float64(2), Height: Float64(2)}}
		graph.Children = append(graph.Children, child)
	}
	registry, _ := sharedRegistries()
	for _, global := range []bool{false, true} {
		want, err := core.NewJSONCodec(registry).Import(graph)
		if err != nil {
			t.Fatal(err)
		}
		req := NewRequest(9, 0, 0, 0)
		root := req.Root("root", nil)
		set := req.PrepareOptions(keys, values)
		if global {
			set = PrepareOptions(keys, values)
		}
		for i := 0; i < 4; i++ {
			node := req.Node(root, "n"+string(rune('a'+i)), 0, 0, 10, 10, set)
			req.Node(node, "c"+string(rune('a'+i)), 1, 1, 2, 2, nil)
		}
		if err := req.Err(); err != nil {
			t.Fatal(err)
		}
		if !sameModel(req.model, want) {
			t.Fatalf("global %v: models differ", global)
		}
		req.Release()
	}
}
