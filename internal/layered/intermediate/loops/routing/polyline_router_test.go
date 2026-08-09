// Copyright (c) 2019 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package routing

import (
	"reflect"
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
)

func TestCutPolylineCornersMatchesUpstream(t *testing.T) {
	input := geometry.Chain{
		{X: 0, Y: 0},
		{X: 100, Y: 0},
		{X: 100, Y: 100},
		{X: -100, Y: 100},
		{X: -100, Y: -100},
		{X: 0, Y: -100},
	}
	tests := []struct {
		name     string
		distance float64
		want     geometry.Chain
	}{
		{
			name:     "usual distance",
			distance: 10,
			want: geometry.Chain{
				{X: 90, Y: 0}, {X: 100, Y: 10},
				{X: 100, Y: 90}, {X: 90, Y: 100},
				{X: -90, Y: 100}, {X: -100, Y: 90},
				{X: -100, Y: -90}, {X: -90, Y: -100},
			},
		},
		{
			name:     "distance limited by adjacent segments",
			distance: 80,
			want: geometry.Chain{
				{X: 50, Y: 0}, {X: 100, Y: 50},
				{X: 100, Y: 50}, {X: 50, Y: 100},
				{X: -20, Y: 100}, {X: -100, Y: 20},
				{X: -100, Y: -50}, {X: -50, Y: -100},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CutPolylineCorners(input, test.distance); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("cut corners = %v, want %v", got, test.want)
			}
		})
	}
}
