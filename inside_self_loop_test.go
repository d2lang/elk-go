// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package elk

import (
	"encoding/json"
	"testing"
)

func TestLayoutRoutesInsideSelfLoopInNodeContents(t *testing.T) {
	output, err := LayoutJSON([]byte(`{
		"id":"root",
		"layoutOptions":{"elk.algorithm":"layered"},
		"children":[{
			"id":"a","width":100,"height":100,
			"layoutOptions":{
				"org.eclipse.elk.insideSelfLoops.activate":"true",
				"elk.portConstraints":"FIXED_POS"
			},
			"ports":[
				{"id":"p1","x":0,"y":20,"width":10,"height":10,
				 "layoutOptions":{"elk.port.side":"WEST"}},
				{"id":"p2","x":90,"y":70,"width":10,"height":10,
				 "layoutOptions":{"elk.port.side":"EAST"}}
			]
		}],
		"edges":[{
			"id":"e","sources":["p1"],"targets":["p2"],
			"layoutOptions":{"org.eclipse.elk.insideSelfLoops.yo":"true"}
		}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	var result Graph
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	node := result.Children[0]
	if node.X == nil || node.Y == nil || node.Width == nil || node.Height == nil ||
		*node.X != 12 || *node.Y != 12 || *node.Width != 64 || *node.Height != 80 {
		t.Fatalf("node geometry = x=%v y=%v w=%v h=%v, want 12,12 64x80: %s",
			node.X, node.Y, node.Width, node.Height, output)
	}
	if len(node.Ports) != 2 || node.Ports[0].X == nil || node.Ports[0].Y == nil ||
		node.Ports[1].X == nil || node.Ports[1].Y == nil ||
		*node.Ports[0].X != 0 || *node.Ports[0].Y != 20 ||
		*node.Ports[1].X != 54 || *node.Ports[1].Y != 70 {
		t.Fatalf("external port positions differ from elkjs 0.8.2: %+v\n%s", node.Ports, output)
	}
	if result.Width == nil || result.Height == nil || *result.Width != 88 || *result.Height != 104 {
		t.Fatalf("root size = %vx%v, want 88x104: %s", result.Width, result.Height, output)
	}
	if len(result.Edges) != 1 || len(result.Edges[0].Sections) != 1 {
		t.Fatalf("inside self loop has no routed section: %s", output)
	}
	section := result.Edges[0].Sections[0]
	if section.StartPoint == nil || section.EndPoint == nil ||
		section.StartPoint.X != 22 || section.StartPoint.Y != 37 ||
		section.EndPoint.X != 66 || section.EndPoint.Y != 87 ||
		len(section.BendPoints) != 2 ||
		section.BendPoints[0].X != 44 || section.BendPoints[0].Y != 37 ||
		section.BendPoints[1].X != 44 || section.BendPoints[1].Y != 87 {
		t.Fatalf("inside self-loop routing differs from elkjs 0.8.2: %+v\n%s", section, output)
	}
}
