// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package layeredlayout_test

import (
	"testing"

	elk "github.com/d2lang/elk-go"
	"github.com/d2lang/elk-go/internal/oracle"
)

// TestHistoricalInvalidEnumFallbackPreservedBy0120 pins a historical importer
// fallback that remains observable in official elkjs 0.12.0. The input option
// is preserved in the output JSON, but layout uses the source-defined default.
func TestHistoricalInvalidEnumFallbackPreservedBy0120(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "edge-label-undefined-is-center",
			input: `{"id":"root","layoutOptions":{"elk.algorithm":"layered","elk.randomSeed":7},"children":[{"id":"a","width":30,"height":20},{"id":"b","width":40,"height":25},{"id":"c","width":35,"height":18}],"edges":[{"id":"ab","sources":["a"],"targets":["b"],"labels":[{"id":"l","text":"x","width":10,"height":7,"layoutOptions":{"elk.edgeLabels.placement":"UNDEFINED"}}]},{"id":"ac","sources":["a"],"targets":["c"]}]}`,
			want:  `{"id":"root","layoutOptions":{"elk.algorithm":"layered","elk.randomSeed":7},"children":[{"id":"a","width":30,"height":20,"x":12,"y":17.833333333333336},{"id":"b","width":40,"height":25,"x":117,"y":12},{"id":"c","width":35,"height":18,"x":62,"y":45.5}],"edges":[{"id":"ab","sources":["a"],"targets":["b"],"labels":[{"id":"l","text":"x","width":10,"height":7,"layoutOptions":{"elk.edgeLabels.placement":"UNDEFINED"},"x":74.5,"y":14.5}],"sections":[{"id":"ab_s0","startPoint":{"x":42,"y":24.5},"endPoint":{"x":117,"y":24.5},"incomingShape":"a","outgoingShape":"b"}],"container":"root"},{"id":"ac","sources":["a"],"targets":["c"],"sections":[{"id":"ac_s0","startPoint":{"x":42,"y":31.166666666666664},"endPoint":{"x":62,"y":54.5},"bendPoints":[{"x":52,"y":31.166666666666664},{"x":52,"y":54.5}],"incomingShape":"a","outgoingShape":"c"}],"container":"root"}],"x":0,"y":0,"width":169,"height":75.5}`,
		},
		{
			name:  "prefer-nodes-is-none",
			input: `{"id":"root","layoutOptions":{"elk.algorithm":"layered","elk.randomSeed":7,"elk.layered.considerModelOrder.strategy":"PREFER_NODES"},"children":[{"id":"a","width":30,"height":20},{"id":"b","width":40,"height":25},{"id":"c","width":35,"height":18}],"edges":[{"id":"ab","sources":["a"],"targets":["b"],"labels":[{"id":"l","text":"x","width":10,"height":7}]},{"id":"ac","sources":["a"],"targets":["c"]}]}`,
			want:  `{"id":"root","layoutOptions":{"elk.algorithm":"layered","elk.randomSeed":7,"elk.layered.considerModelOrder.strategy":"PREFER_NODES"},"children":[{"id":"a","width":30,"height":20,"x":12,"y":17.833333333333336},{"id":"b","width":40,"height":25,"x":117,"y":12},{"id":"c","width":35,"height":18,"x":62,"y":45.5}],"edges":[{"id":"ab","sources":["a"],"targets":["b"],"labels":[{"id":"l","text":"x","width":10,"height":7,"x":74.5,"y":14.5}],"sections":[{"id":"ab_s0","startPoint":{"x":42,"y":24.5},"endPoint":{"x":117,"y":24.5},"incomingShape":"a","outgoingShape":"b"}],"container":"root"},{"id":"ac","sources":["a"],"targets":["c"],"sections":[{"id":"ac_s0","startPoint":{"x":42,"y":31.166666666666664},"endPoint":{"x":62,"y":54.5},"bendPoints":[{"x":52,"y":31.166666666666664},{"x":52,"y":54.5}],"incomingShape":"a","outgoingShape":"c"}],"container":"root"}],"x":0,"y":0,"width":169,"height":75.5}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := elk.LayoutJSON([]byte(test.input))
			if err != nil {
				t.Fatal(err)
			}
			if err := oracle.CompareJSON([]byte(test.want), got, oracle.ExactTolerance); err != nil {
				t.Fatalf("%v\nwant: %s\ngot:  %s", err, test.want, got)
			}
		})
	}
}
