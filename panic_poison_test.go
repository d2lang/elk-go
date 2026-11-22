package elk

// Regression test: a layout that panics inside crossing counting must not change
// later layouts in the same process. The pinned elk-go panics on
// poisonGraph in the north/south crossing counter ("binary indexed tree
// index out of range"); that panic is expected and recovered here. What must
// hold is that victimGraph lays out the same before and after it.

import (
	"bytes"
	"testing"
)

const poisonGraph = `{"children":[{"id":"n1","ports":[{"id":"p2"}]},{"id":"n3"},{"id":"n7"},{"children":[{"children":[{"children":[{"id":"n22"},{"id":"n23","ports":[{"id":"p24"},{"id":"p25"}]},{"id":"n26","ports":[{"id":"p27"}]}],"id":"n14","ports":[{"id":"p20","layoutOptions":{"elk.port.side":"WEST"}},{"id":"p21","layoutOptions":{"elk.port.side":"WEST"}}]},{"id":"n30","ports":[{"id":"p31"},{"id":"p32"}]},{"id":"n36","ports":[{"id":"p37"}]}],"id":"n9","ports":[{"id":"p10"},{"id":"p11"},{"id":"p12"},{"id":"p13","layoutOptions":{"elk.port.side":"WEST"}}]}],"id":"n8"}],"edges":[{"id":"e38","sources":["p13"],"targets":["p2"]},{"id":"e40","sources":["p37"],"targets":["p27"]},{"id":"e41","sources":["n22"],"targets":["n23"]},{"id":"e43","sources":["n23"],"targets":["n22"]},{"id":"e44","sources":["p37"],"targets":["n14"]},{"id":"e45","sources":["n7"],"targets":["n26"]},{"id":"e46","sources":["n23"],"targets":["n36"]},{"id":"e48","sources":["n3"],"targets":["n23"]},{"id":"e49","sources":["n23"],"targets":["n22"]},{"id":"e52","sources":["p21"],"targets":["p37"]}],"id":"root","layoutOptions":{"elk.direction":"UP","elk.hierarchyHandling":"INCLUDE_CHILDREN"}}`

const victimGraph = `{"id":"root","layoutOptions":{"elk.hierarchyHandling":"INCLUDE_CHILDREN"},"children":[{"id":"a","width":30,"height":30},{"id":"b","width":30,"height":30},{"id":"c","width":30,"height":30}],"edges":[{"id":"e1","sources":["a"],"targets":["b"]},{"id":"e2","sources":["a"],"targets":["c"]}]}`

func crossminLayout(t *testing.T, graph string) (out []byte, err error, panicked any) {
	t.Helper()
	defer func() { panicked = recover() }()
	out, err = LayoutJSON([]byte(graph))
	return out, err, nil
}

func TestPanicDoesNotPoisonLaterLayouts(t *testing.T) {
	before, err, panicked := crossminLayout(t, victimGraph)
	if err != nil || panicked != nil {
		t.Fatalf("victim before: err %v, panic %v", err, panicked)
	}
	for round := 0; round < 3; round++ {
		_, _, panicked = crossminLayout(t, poisonGraph)
		t.Logf("poison graph round %d: panic %v", round, panicked)
		after, err, panicked := crossminLayout(t, victimGraph)
		if err != nil || panicked != nil {
			t.Fatalf("victim after a panicking layout: err %v, panic %v", err, panicked)
		}
		if !bytes.Equal(before, after) {
			t.Fatalf("victim layout changed after a panicking layout:\nbefore %s\nafter  %s", before, after)
		}
	}
}
