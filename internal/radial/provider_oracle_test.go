// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0/.
//
// SPDX-License-Identifier: EPL-2.0

package radial_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/d2lang/elk-go/graphjson"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/oracle"
	"github.com/d2lang/elk-go/internal/radial"
)

func TestELKJS0120OracleSimpleTree(t *testing.T) {
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Skip(err)
	}
	input := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"radial"},"children":[{"id":"a","width":30,"height":20},{"id":"b","width":40,"height":10},{"id":"c","width":20,"height":30}],"edges":[{"id":"e1","sources":["a"],"targets":["b"]},{"id":"e2","sources":["a"],"targets":["c"]}]}`)
	if err := oracle.Differential(context.Background(), runner, input, radialLayoutJSON, oracle.ExactTolerance); err != nil {
		t.Fatal(err)
	}
}

func TestOfficialELKJS0120EdgeLengthTieOrder(t *testing.T) {
	input := []byte(`{"id":"root","children":[{"id":"a","x":3,"y":7,"width":23,"height":17,"layoutOptions":{"elk.radial.orderId":4,"elk.position":"(3,7)"}},{"id":"b","x":91,"y":13,"width":31,"height":11,"layoutOptions":{"elk.radial.orderId":3,"elk.position":"(91,13)"}},{"id":"c","x":37,"y":89,"width":13,"height":29,"layoutOptions":{"elk.radial.orderId":2,"elk.position":"(37,89)"}},{"id":"d","x":151,"y":67,"width":41,"height":7,"layoutOptions":{"elk.radial.orderId":1,"elk.position":"(151,67)"}}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"ac","sources":["a"],"targets":["c"]},{"id":"bd","sources":["b"],"targets":["d"]}],"layoutOptions":{"elk.algorithm":"radial","elk.radial.radius":42,"elk.radial.compactionStepSize":3,"elk.radial.optimizationCriteria":"EDGE_LENGTH"}}`)
	want := []byte(`{"id":"root","children":[{"id":"a","x":46.703106529728615,"y":31.699026676703014,"width":23,"height":17,"layoutOptions":{"elk.radial.orderId":4,"elk.position":"(3,7)"}},{"id":"b","x":82.40621305945722,"y":48.39805335340607,"width":31,"height":11,"layoutOptions":{"elk.radial.orderId":3,"elk.position":"(91,13)"}},{"id":"c","x":12,"y":11.999999999999996,"width":13,"height":29,"layoutOptions":{"elk.radial.orderId":2,"elk.position":"(37,89)"}},{"id":"d","x":117.10931958918582,"y":64.09708003010913,"width":41,"height":7,"layoutOptions":{"elk.radial.orderId":1,"elk.position":"(151,67)"}}],"edges":[{"id":"ab","sources":["a"],"targets":["b"],"sections":[{"id":"ab_s0","startPoint":{"x":69.70310652972861,"y":44.166948095248046},"endPoint":{"x":82.40621305945722,"y":48.54998535449755}}],"container":"root"},{"id":"ac","sources":["a"],"targets":["c"],"sections":[{"id":"ac_s0","startPoint":{"x":46.703106529728615,"y":36.231105258157996},"endPoint":{"x":25,"y":28.74273819309066}}],"container":"root"},{"id":"bd","sources":["b"],"targets":["d"],"sections":[{"id":"bd_s0","startPoint":{"x":113.40621305945722,"y":59.24612135231459},"endPoint":{"x":127.4654695089191,"y":64.09708003010913}}],"container":"root"}],"layoutOptions":{"elk.algorithm":"radial","elk.radial.radius":42,"elk.radial.compactionStepSize":3,"elk.radial.optimizationCriteria":"EDGE_LENGTH"},"x":0,"y":0,"width":170.10931958918582,"height":83.09708003010913}`)
	got, err := radialLayoutJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := oracle.CompareJSON(want, got, oracle.ExactTolerance); err != nil {
		t.Fatal(err)
	}
}

func TestELKJS0120OracleRadialOptions(t *testing.T) {
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Skip(err)
	}
	fixtures := map[string]string{
		"radius id sorter leaf wedge":  `{"id":"root","layoutOptions":{"elk.algorithm":"radial","elk.radial.radius":37,"elk.radial.sorter":"ID","elk.radial.wedgeCriteria":"LEAF_NUMBER","elk.spacing.nodeNode":7},"children":[{"id":"a","width":23,"height":17},{"id":"b","width":31,"height":11,"layoutOptions":{"elk.radial.orderId":8}},{"id":"c","width":13,"height":29,"layoutOptions":{"elk.radial.orderId":2}},{"id":"d","width":41,"height":7},{"id":"e","width":19,"height":21}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"ac","sources":["a"],"targets":["c"]},{"id":"bd","sources":["b"],"targets":["d"]},{"id":"ce","sources":["c"],"targets":["e"]}]}`,
		"first-ring overlap extension": `{"id":"root","layoutOptions":{"elk.algorithm":"radial","elk.radial.radius":18,"elk.spacing.nodeNode":20},"children":[{"id":"a","width":50,"height":40},{"id":"b","width":70,"height":30},{"id":"c","width":60,"height":45},{"id":"d","width":55,"height":35}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"ac","sources":["a"],"targets":["c"]},{"id":"ad","sources":["a"],"targets":["d"]}]}`,
		"disconnected input":           `{"id":"root","layoutOptions":{"elk.algorithm":"radial"},"children":[{"id":"a","width":20,"height":20},{"id":"b","width":20,"height":20},{"id":"c","width":30,"height":10,"x":5,"y":8}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]}]}`,
		"radial compaction":            `{"id":"root","layoutOptions":{"elk.algorithm":"radial","elk.radial.radius":60,"elk.radial.compactor":"RADIAL_COMPACTION","elk.radial.compactionStepSize":3},"children":[{"id":"a","width":20,"height":20},{"id":"b","width":20,"height":20},{"id":"c","width":20,"height":20}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"bc","sources":["b"],"targets":["c"]}]}`,
		"polar coordinate sorter":      `{"id":"root","layoutOptions":{"elk.algorithm":"radial","elk.radial.sorter":"POLAR_COORDINATE","elk.radial.radius":40},"children":[{"id":"a","width":20,"height":20,"x":4,"y":7,"layoutOptions":{"elk.position":"(4,7)"}},{"id":"b","width":20,"height":20,"x":90,"y":10,"layoutOptions":{"elk.position":"(90,10)"}},{"id":"c","width":20,"height":20,"x":3,"y":80,"layoutOptions":{"elk.position":"(3,80)"}},{"id":"d","width":20,"height":20,"x":60,"y":70,"layoutOptions":{"elk.position":"(60,70)"}}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"ac","sources":["a"],"targets":["c"]},{"id":"bd","sources":["b"],"targets":["d"]}]}`,
		"edge-length translation":      `{"id":"root","layoutOptions":{"elk.algorithm":"radial","elk.radial.optimizationCriteria":"EDGE_LENGTH","elk.radial.radius":35},"children":[{"id":"a","width":30,"height":10},{"id":"b","width":50,"height":20},{"id":"c","width":10,"height":40}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"ac","sources":["a"],"targets":["c"]}]}`,
		"wedge compaction":             `{"id":"root","layoutOptions":{"elk.algorithm":"radial","elk.radial.radius":70,"elk.radial.compactor":"WEDGE_COMPACTION","elk.radial.compactionStepSize":5},"children":[{"id":"a","width":20,"height":20},{"id":"b","width":20,"height":20},{"id":"c","width":20,"height":20}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"ac","sources":["a"],"targets":["c"]}]}`,
		"center on root":               `{"id":"root","layoutOptions":{"elk.algorithm":"radial","elk.radial.centerOnRoot":"true","elk.radial.radius":"45"},"children":[{"id":"a","width":35,"height":20},{"id":"b","width":70,"height":25},{"id":"c","width":15,"height":45},{"id":"d","width":30,"height":15}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"ac","sources":["a"],"targets":["c"]},{"id":"bd","sources":["b"],"targets":["d"]}]}`,
		"target angle rotation":        `{"id":"root","layoutOptions":{"elk.algorithm":"radial","elk.radial.rotate":"true","elk.radial.rotation.targetAngle":"1.25","elk.radial.radius":"45"},"children":[{"id":"a","width":35,"height":20},{"id":"b","width":70,"height":25},{"id":"c","width":15,"height":45}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"ac","sources":["a"],"targets":["c"]}]}`,
		"wedge-aligned rotation":       `{"id":"root","layoutOptions":{"elk.algorithm":"radial","elk.radial.rotate":"true","elk.radial.rotation.targetAngle":"-0.75","elk.radial.rotation.computeAdditionalWedgeSpace":"true","elk.radial.radius":"45"},"children":[{"id":"a","width":35,"height":20},{"id":"b","width":70,"height":25},{"id":"c","width":15,"height":45},{"id":"d","width":30,"height":15}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"ac","sources":["a"],"targets":["c"]},{"id":"ad","sources":["a"],"targets":["d"]}]}`,
		"outgoing edge angles":         `{"id":"root","layoutOptions":{"elk.algorithm":"radial","elk.radial.rotation.outgoingEdgeAngles":"true","elk.radial.radius":"45"},"children":[{"id":"a","width":35,"height":20},{"id":"b","width":70,"height":25},{"id":"c","width":15,"height":45}],"edges":[{"id":"ab","sources":["a"],"targets":["b"]},{"id":"ac","sources":["a"],"targets":["c"]}]}`,
	}
	for name, input := range fixtures {
		t.Run(name, func(t *testing.T) {
			if err := oracle.Differential(context.Background(), runner, []byte(input), radialLayoutJSON, oracle.ExactTolerance); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func radialLayoutJSON(input []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()
	var value graphjson.Graph
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	options := core.NewOptionRegistry()
	radial.RegisterOptions(options)
	algorithms := core.NewAlgorithmRegistry()
	if err := radial.Register(algorithms); err != nil {
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
