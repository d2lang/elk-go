// Copyright (c) 2018, 2020 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package rectpacking_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/d2lang/elk-go/graphjson"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/oracle"
	"github.com/d2lang/elk-go/internal/rectpacking"
)

func rectpackingLayoutJSON(input []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()
	var value graphjson.Graph
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	options := core.NewOptionRegistry()
	rectpacking.RegisterOptions(options)
	algorithms := core.NewAlgorithmRegistry()
	if err := rectpacking.Register(algorithms); err != nil {
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

func TestOfficialELKJS0120RectPacking(t *testing.T) {
	input := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"rectpacking"},"children":[{"id":"a","width":30,"height":20},{"id":"b","width":40,"height":10},{"id":"c","width":20,"height":30}]}`)
	want := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"rectpacking"},"children":[{"id":"a","width":30,"height":20,"x":15,"y":15},{"id":"b","width":40,"height":10,"x":60,"y":15},{"id":"c","width":20,"height":30,"x":15,"y":50}],"x":0,"y":0,"width":115,"height":95}`)
	got, err := rectpackingLayoutJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := oracle.CompareJSON(want, got, oracle.ExactTolerance); err != nil {
		t.Fatal(err)
	}
}

func TestOfficialELKJS0120RectPackingStrategies(t *testing.T) {
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Skip(err)
	}
	fixtures := map[string]string{
		"greedy area driven":                 `{"id":"root","layoutOptions":{"elk.algorithm":"rectpacking","elk.rectpacking.widthApproximation.optimizationGoal":"AREA_DRIVEN","elk.rectpacking.widthApproximation.lastPlaceShift":false,"elk.spacing.nodeNode":7,"elk.padding":"[top=3,left=5,bottom=11,right=13]"},"children":[{"id":"a","width":23,"height":17},{"id":"b","width":31,"height":11},{"id":"c","width":13,"height":29},{"id":"d","width":41,"height":7}]}`,
		"target width simple":                `{"id":"root","layoutOptions":{"elk.algorithm":"rectpacking","elk.rectpacking.widthApproximation.strategy":"TARGET_WIDTH","elk.rectpacking.widthApproximation.targetWidth":80,"elk.rectpacking.packing.strategy":"SIMPLE","elk.spacing.nodeNode":9},"children":[{"id":"a","width":23,"height":17},{"id":"b","width":31,"height":11},{"id":"c","width":13,"height":29},{"id":"d","width":41,"height":7}]}`,
		"packing none preserves coordinates": `{"id":"root","layoutOptions":{"elk.algorithm":"rectpacking","elk.rectpacking.packing.strategy":"NONE"},"children":[{"id":"a","x":21,"y":9,"width":23,"height":17},{"id":"b","x":70,"y":31,"width":31,"height":11},{"id":"c","x":8,"y":63,"width":13,"height":29}]}`,
		"equal whitespace":                   `{"id":"root","layoutOptions":{"elk.algorithm":"rectpacking","elk.nodeSize.constraints":"MINIMUM_SIZE","elk.nodeSize.minimum":"(180,130)","elk.rectpacking.whiteSpaceElimination.strategy":"EQUAL_BETWEEN_STRUCTURES"},"children":[{"id":"a","width":23,"height":17},{"id":"b","width":31,"height":11},{"id":"c","width":13,"height":29},{"id":"d","width":41,"height":7}]}`,
		"simple equal whitespace":            `{"id":"root","layoutOptions":{"elk.algorithm":"rectpacking","elk.nodeSize.constraints":"MINIMUM_SIZE","elk.nodeSize.minimum":"(180,130)","elk.rectpacking.packing.strategy":"SIMPLE","elk.rectpacking.whiteSpaceElimination.strategy":"EQUAL_BETWEEN_STRUCTURES"},"children":[{"id":"a","width":23,"height":17},{"id":"b","width":31,"height":11},{"id":"c","width":13,"height":29},{"id":"d","width":41,"height":7}]}`,
		"to aspect ratio":                    `{"id":"root","layoutOptions":{"elk.algorithm":"rectpacking","elk.aspectRatio":2.4,"elk.rectpacking.whiteSpaceElimination.strategy":"TO_ASPECT_RATIO"},"children":[{"id":"a","width":23,"height":17},{"id":"b","width":31,"height":11},{"id":"c","width":13,"height":29},{"id":"d","width":41,"height":7}]}`,
		"simple to aspect ratio":             `{"id":"root","layoutOptions":{"elk.algorithm":"rectpacking","elk.nodeSize.constraints":"MINIMUM_SIZE","elk.nodeSize.minimum":"(180,130)","elk.rectpacking.packing.strategy":"SIMPLE","elk.aspectRatio":2.4,"elk.rectpacking.whiteSpaceElimination.strategy":"TO_ASPECT_RATIO"},"children":[{"id":"a","width":23,"height":17},{"id":"b","width":31,"height":11},{"id":"c","width":13,"height":29},{"id":"d","width":41,"height":7}]}`,
		"order by size":                      `{"id":"root","layoutOptions":{"elk.algorithm":"rectpacking","elk.rectpacking.orderBySize":true},"children":[{"id":"a","width":20,"height":11},{"id":"b","width":20,"height":43},{"id":"c","width":20,"height":27},{"id":"d","width":20,"height":43}]}`,
		"interactive order":                  `{"id":"root","layoutOptions":{"elk.algorithm":"rectpacking","elk.interactive":true},"children":[{"id":"a","width":20,"height":10,"layoutOptions":{"elk.rectpacking.desiredPosition":2}},{"id":"b","width":20,"height":20},{"id":"c","width":20,"height":30,"layoutOptions":{"elk.rectpacking.desiredPosition":0}},{"id":"d","width":20,"height":40}]}`,
		"force new row":                      `{"id":"root","layoutOptions":{"elk.algorithm":"rectpacking","elk.rectpacking.widthApproximation.strategy":"TARGET_WIDTH","elk.rectpacking.widthApproximation.targetWidth":200},"children":[{"id":"a","width":30,"height":20},{"id":"b","width":30,"height":20,"layoutOptions":{"elk.rectpacking.inNewRow":true}},{"id":"c","width":30,"height":20}]}`,
		"row height reevaluation":            `{"id":"root","layoutOptions":{"elk.algorithm":"rectpacking","elk.rectpacking.widthApproximation.strategy":"TARGET_WIDTH","elk.rectpacking.widthApproximation.targetWidth":100,"elk.rectpacking.packing.compaction.rowHeightReevaluation":true},"children":[{"id":"a","width":22,"height":16},{"id":"b","width":18,"height":47},{"id":"c","width":31,"height":13},{"id":"d","width":24,"height":38},{"id":"e","width":17,"height":21}]}`,
		"two compaction iterations":          `{"id":"root","layoutOptions":{"elk.algorithm":"rectpacking","elk.rectpacking.packing.compaction.iterations":2},"children":[{"id":"a","width":37,"height":20},{"id":"b","width":55,"height":74},{"id":"c","width":63,"height":14},{"id":"d","width":81,"height":16},{"id":"e","width":75,"height":10},{"id":"f","width":23,"height":50},{"id":"g","width":85,"height":80},{"id":"h","width":51,"height":10}]}`,
		"try box":                            `{"id":"root","layoutOptions":{"elk.algorithm":"rectpacking","elk.rectpacking.trybox":true},"children":[{"id":"a","width":20,"height":30},{"id":"b","width":25,"height":30},{"id":"c","width":30,"height":30},{"id":"d","width":35,"height":30}]}`,
		"fixed graph and content alignment":  `{"id":"root","width":240,"height":160,"layoutOptions":{"elk.algorithm":"rectpacking","elk.nodeSize.fixedGraphSize":true,"elk.nodeSize.constraints":"MINIMUM_SIZE","elk.nodeSize.minimum":"(180,120)","elk.contentAlignment":"[H_RIGHT,V_BOTTOM]"},"children":[{"id":"a","width":20,"height":30},{"id":"b","width":25,"height":15},{"id":"c","width":30,"height":20}]}`,
		"default mixed sizes":                `{"id":"root","layoutOptions":{"elk.algorithm":"rectpacking"},"children":[{"id":"a","width":12,"height":42},{"id":"b","width":33,"height":11},{"id":"c","width":24,"height":20},{"id":"d","width":18,"height":36},{"id":"e","width":50,"height":9}]}`,
	}
	for name, input := range fixtures {
		t.Run(name, func(t *testing.T) {
			if err := oracle.Differential(context.Background(), runner, []byte(input), rectpackingLayoutJSON, oracle.ExactTolerance); err != nil {
				t.Fatal(err)
			}
		})
	}
}
