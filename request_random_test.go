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
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/graph"
)

var revOptionPool = map[string][]string{
	"elk.direction":                                {`"DOWN"`, `"UP"`, `"LEFT"`, `"RIGHT"`, `""`, `2`, `"3"`},
	"org.eclipse.elk.direction":                    {`"RIGHT"`},
	"direction":                                    {`"LEFT"`},
	"elk.algorithm":                                {`"layered"`, `"org.eclipse.elk.layered"`, `"force"`, `"stress"`, `"box"`, `"rectpacking"`},
	"elk.spacing.nodeNode":                         {`20`, `20.5`, `"20.5d"`, `" 12 "`, `"abc"`, `1e2`, `-0`, `true`},
	"spacing.nodeNodeBetweenLayers":                {`5`, `200`, `"33"`},
	"elk.layered.spacing.nodeNodeBetweenLayers":    {`15`, `150`},
	"spacing.edgeEdgeBetweenLayers":                {`3`, `30`},
	"elk.spacing.edgeNode":                         {`1`, `40`, `1e-7`},
	"elk.spacing.nodeSelfLoop":                     {`5`, `50`},
	"elk.padding":                                  {`"[top=10,left=10,bottom=10,right=10]"`, `"[top=1.5,left=0]"`, `"bogus"`, `12`},
	"elk.nodeSize.constraints":                     {`"MINIMUM_SIZE"`, `"[PORTS, NODE_LABELS]"`, `""`, `"BOGUS"`, `"0"`},
	"elk.nodeSize.minimum":                         {`"(20, 30)"`, `"(a,b)"`},
	"elk.nodeLabels.placement":                     {`"[H_CENTER, V_TOP, INSIDE]"`, `"OUTSIDE V_BOTTOM H_LEFT"`},
	"elk.portConstraints":                          {`"FIXED_POS"`, `"FIXED_SIDE"`, `"FREE"`},
	"elk.port.side":                                {`"NORTH"`, `"SOUTH"`, `"EAST"`, `"WEST"`},
	"elk.hierarchyHandling":                        {`"INCLUDE_CHILDREN"`, `"SEPARATE_CHILDREN"`, `"INHERIT"`},
	"elk.edgeRouting":                              {`"ORTHOGONAL"`, `"POLYLINE"`, `"SPLINES"`},
	"elk.layered.thoroughness":                     {`1`, `8`, `"7"`, `7.5`, `1000001`},
	"elk.layered.considerModelOrder.strategy":      {`"NODES_AND_EDGES"`, `"NONE"`},
	"elk.layered.cycleBreaking.strategy":           {`"GREEDY"`, `"GREEDY_MODEL_ORDER"`},
	"elk.edgeLabels.inline":                        {`true`, `false`, `"true"`},
	"elk.layered.mergeEdges":                       {`true`},
	"elk.json.shapeCoords":                         {`"PARENT"`, `"ROOT"`, `"INHERIT"`},
	"elk.json.edgeCoords":                          {`"CONTAINER"`, `"PARENT"`, `"ROOT"`, `"INHERIT"`},
	"elk.layered.layering.layerConstraint":         {`"FIRST"`, `"LAST"`},
	"elk.layered.edgeRouting.selfLoopDistribution": {`"EQUALLY"`, `"NORTH"`},
	"elk.contentAlignment":                         {`"H_CENTER V_CENTER"`, `"[V_TOP]"`},
	"spacing":                                      {`10`},
	"foo.bar":                                      {`1`, `"x"`},
	"elk.layered.nodePlacement.bk.fixedAlignment":  {`"BALANCED"`, `"LEFTUP"`},
	"elk.layered.wrapping.strategy":                {`"OFF"`, `"MULTI_EDGE"`},
}

type revGen struct {
	rng   *rand.Rand
	keys  []string
	n     int
	ids   []string
	ports []string
}

func (g *revGen) options() string {
	var parts []string
	for i := g.rng.Intn(6); i > 0; i-- {
		key := g.keys[g.rng.Intn(len(g.keys))]
		values := revOptionPool[key]
		parts = append(parts, fmt.Sprintf("%q:%s", key, values[g.rng.Intn(len(values))]))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func (g *revGen) shape() string {
	v := func() string { return []string{"0", "10", "25.5", "-3", "1e2", "7", "-0"}[g.rng.Intn(7)] }
	return fmt.Sprintf(`"x":%s,"y":%s,"width":%s,"height":%s`, v(), v(), v(), v())
}

func (g *revGen) label() string {
	s := `{"text":` + []string{`"l"`, `""`, `"<&>"`}[g.rng.Intn(3)] + "," + g.shape()
	if g.rng.Intn(3) == 0 {
		s += `,"layoutOptions":` + g.options()
	}
	return s + "}"
}

func (g *revGen) labels() string {
	var parts []string
	for i := g.rng.Intn(3); i > 0; i-- {
		parts = append(parts, g.label())
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func (g *revGen) node(depth int) string {
	g.n++
	id := fmt.Sprintf("n%d", g.n)
	if g.rng.Intn(40) == 0 && len(g.ids) > 0 {
		id = g.ids[g.rng.Intn(len(g.ids))] // duplicate
	}
	g.ids = append(g.ids, id)
	s := fmt.Sprintf(`{"id":%q,%s`, id, g.shape())
	if g.rng.Intn(2) == 0 {
		s += `,"layoutOptions":` + g.options()
	}
	if g.rng.Intn(3) == 0 {
		s += `,"labels":` + g.labels()
	}
	if g.rng.Intn(4) == 0 {
		var ports []string
		for i := 1 + g.rng.Intn(3); i > 0; i-- {
			g.n++
			pid := fmt.Sprintf("p%d", g.n)
			g.ports = append(g.ports, pid)
			p := fmt.Sprintf(`{"id":%q,%s`, pid, g.shape())
			if g.rng.Intn(2) == 0 {
				p += `,"layoutOptions":` + g.options()
			}
			if g.rng.Intn(4) == 0 {
				p += `,"labels":` + g.labels()
			}
			ports = append(ports, p+"}")
		}
		s += `,"ports":[` + strings.Join(ports, ",") + "]"
	}
	if depth < 3 && g.rng.Intn(3) == 0 {
		var kids []string
		for i := 1 + g.rng.Intn(4); i > 0; i-- {
			kids = append(kids, g.node(depth+1))
		}
		s += `,"children":[` + strings.Join(kids, ",") + "]"
		if g.rng.Intn(3) == 0 {
			s += `,"edges":` + g.edges()
		}
	}
	return s + "}"
}

func (g *revGen) ref() string {
	pool := append(append([]string{}, g.ids...), g.ports...)
	if len(pool) == 0 || g.rng.Intn(60) == 0 {
		return `"missing"`
	}
	return fmt.Sprintf("%q", pool[g.rng.Intn(len(pool))])
}

func (g *revGen) edges() string {
	var parts []string
	for i := g.rng.Intn(6); i > 0; i-- {
		g.n++
		s := fmt.Sprintf(`{"id":"e%d","sources":[%s],"targets":[%s]`, g.n, g.ref(), g.ref())
		if g.rng.Intn(3) == 0 {
			s += `,"labels":` + g.labels()
		}
		if g.rng.Intn(4) == 0 {
			s += `,"layoutOptions":` + g.options()
		}
		parts = append(parts, s+"}")
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func (g *revGen) graph() []byte {
	var kids []string
	for i := g.rng.Intn(7); i > 0; i-- {
		kids = append(kids, g.node(0))
	}
	s := `{"id":"root","layoutOptions":` + g.options()
	if len(kids) > 0 {
		s += `,"children":[` + strings.Join(kids, ",") + "]"
	}
	return []byte(s + `,"edges":` + g.edges() + "}")
}

// TestRequestMatchesImport builds random graphs (string IDs, full
// shapes, labels with text, sources/targets edges; option keys of every kind,
// duplicates and missing ends) both as Request and through Import, and
// compares models before and after layout, the read-back values with the
// transfer, and errors (a Request may only fail where Import fails, and must
// fail where Import fails).
func TestRequestMatchesImport(t *testing.T) {
	options, _ := sharedRegistries()
	keys := make([]string, 0, len(revOptionPool))
	for key := range revOptionPool {
		keys = append(keys, key)
	}
	// Deterministic key order.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	laidOut, failed, panics := 0, 0, 0
	for seed := int64(0); seed < 3000; seed++ {
		gen := &revGen{rng: rand.New(rand.NewSource(seed)), keys: keys}
		input := gen.graph()
		graph := decodeLikeLayoutJSON(t, input)
		codec := core.NewJSONCodec(options)
		want, importErr := codec.Import(graph)
		b := buildRequest(t, decodeLikeLayoutJSON(t, input))
		reqErr := b.req.Err()
		if (importErr != nil) != (reqErr != nil) {
			t.Fatalf("seed %d: import error %v, request error %v\n%s", seed, importErr, reqErr, input)
		}
		if importErr != nil {
			if importErr.Error() != reqErr.Error() {
				t.Fatalf("seed %d: import error %q, request error %q", seed, importErr, reqErr)
			}
			failed++
			b.req.Release()
			continue
		}
		if !revSameModel(b.req.model, want) {
			t.Fatalf("seed %d: models differ before layout\n%s", seed, input)
		}
		guard := func(run func() error) (err error, panicked string) {
			defer func() {
				if r := recover(); r != nil {
					panicked = fmt.Sprint(r)
				}
			}()
			return run(), ""
		}
		layoutErr, layoutPanic := guard(func() error { return layoutModel(want) })
		reqLayoutErr, reqPanic := guard(b.req.Layout)
		if layoutPanic != reqPanic {
			t.Fatalf("seed %d: layout panic %q, request panic %q", seed, layoutPanic, reqPanic)
		}
		if layoutPanic != "" {
			panics++
			continue
		}
		if (layoutErr != nil) != (reqLayoutErr != nil) {
			t.Fatalf("seed %d: layout error %v, request %v", seed, layoutErr, reqLayoutErr)
		}
		if layoutErr != nil {
			if !strings.Contains(reqLayoutErr.Error(), layoutErr.Error()) {
				t.Fatalf("seed %d: layout error %q, request %q", seed, layoutErr, reqLayoutErr)
			}
			failed++
			b.req.Release()
			continue
		}
		if !revSameModel(b.req.model, want) {
			t.Fatalf("seed %d: models differ after layout\n%s", seed, input)
		}
		if err := codec.TransferLayout(want); err != nil {
			t.Fatal(err)
		}
		if _, err := json.Marshal(graph); err != nil {
			t.Fatal(err)
		}
		b.nodes[0] = graph
		// checkReadBack reads the builder's Graph elements, which must be
		// the transferred ones: rebuild the references against graph.
		rb := buildRequestRefs(graph)
		b.nodes, b.edges = rb.nodes, rb.edges
		checkReadBack(t, fmt.Sprint("seed ", seed), b)
		b.req.Release()
		laidOut++
	}
	if laidOut < 1000 {
		t.Fatalf("only %d of 3000 random graphs laid out (%d failed)", laidOut, failed)
	}
	t.Logf("%d laid out, %d failed the same way, %d panicked the same way", laidOut, failed, panics)
	_ = bytes.Equal
}

// buildRequestRefs lists g's nodes and edges in Request reference order
// (nodes in import order, edges by parent in that order).
func buildRequestRefs(g *Node) *requestBuilder {
	b := &requestBuilder{}
	var nodes func(n *Node)
	nodes = func(n *Node) {
		b.nodes = append(b.nodes, n)
		for _, child := range n.Children {
			if child != nil {
				nodes(child)
			}
		}
	}
	nodes(g)
	for _, n := range b.nodes {
		for _, edge := range n.Edges {
			if edge != nil {
				b.edges = append(b.edges, edge)
			}
		}
	}
	return b
}

// revSameModel is sameModel with floats compared by their bits (layout
// results may hold NaN, which reflect.DeepEqual never finds equal).
func revSameModel(got, want *graph.Model) bool {
	sameModel(got, want) // normalizes empty property maps of both
	return revDeepEqual(reflect.ValueOf(got), reflect.ValueOf(want))
}

func revDeepEqual(a, b reflect.Value) bool {
	if a.Kind() != b.Kind() {
		return false
	}
	switch a.Kind() {
	case reflect.Float32, reflect.Float64:
		return math.Float64bits(a.Float()) == math.Float64bits(b.Float())
	case reflect.Bool:
		return a.Bool() == b.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return a.Int() == b.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return a.Uint() == b.Uint()
	case reflect.String:
		return a.String() == b.String()
	case reflect.Pointer, reflect.Interface:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() == b.IsNil()
		}
		if a.Kind() == reflect.Interface && a.Elem().Type() != b.Elem().Type() {
			return false
		}
		return revDeepEqual(a.Elem(), b.Elem())
	case reflect.Slice:
		if a.IsNil() != b.IsNil() || a.Len() != b.Len() {
			return false
		}
		fallthrough
	case reflect.Array:
		for i := 0; i < a.Len(); i++ {
			if !revDeepEqual(a.Index(i), b.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			if !revDeepEqual(a.Field(i), b.Field(i)) {
				return false
			}
		}
		return true
	case reflect.Map:
		if a.IsNil() != b.IsNil() || a.Len() != b.Len() {
			return false
		}
		for _, key := range a.MapKeys() {
			other := b.MapIndex(key)
			if !other.IsValid() || !revDeepEqual(a.MapIndex(key), other) {
				return false
			}
		}
		return true
	case reflect.Func:
		return a.IsNil() == b.IsNil() && (a.IsNil() || a.Pointer() == b.Pointer())
	default:
		return true
	}
}
