// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0/.
//
// SPDX-License-Identifier: EPL-2.0

package oracle

import (
	"errors"
	"strings"
	"testing"
)

func TestCompareJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		want    string
		got     string
		wantErr string
	}{
		{
			name: "coordinate tolerance and numeric spelling",
			want: `{"id":"root","x":1000,"width":40,"children":[]}`,
			got:  `{"children":[],"width":40.0,"x":1000.0000002,"id":"root"}`,
		},
		{
			name: "GWT identity hash is ignored",
			want: `{"id":"root","$H":13,"children":[{"id":"a","$H":277}]}`,
			got:  `{"id":"root","children":[{"id":"a"}]}`,
		},
		{
			name:    "IDs are exact",
			want:    `{"id":"a"}`,
			got:     `{"id":"b"}`,
			wantErr: "$.id",
		},
		{
			name:    "array order is exact",
			want:    `{"children":[{"id":"a"},{"id":"b"}]}`,
			got:     `{"children":[{"id":"b"},{"id":"a"}]}`,
			wantErr: "$.children[0].id",
		},
		{
			name:    "field presence is exact",
			want:    `{"id":"root","edges":[]}`,
			got:     `{"id":"root"}`,
			wantErr: "$.edges",
		},
		{
			name:    "section count is exact",
			want:    `{"edges":[{"id":"e","sections":[{},{}]}]}`,
			got:     `{"edges":[{"id":"e","sections":[{}]}]}`,
			wantErr: "$.edges[0].sections",
		},
		{
			name:    "non-coordinate numbers are exact",
			want:    `{"layoutOptions":{"thoroughness":8}}`,
			got:     `{"layoutOptions":{"thoroughness":8.0000001}}`,
			wantErr: "thoroughness",
		},
		{
			name:    "large coordinate mismatch",
			want:    `{"x":1}`,
			got:     `{"x":1.01}`,
			wantErr: "$.x",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := CompareJSON([]byte(test.want), []byte(test.got), DefaultTolerance)
			if test.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("CompareJSON() error = %v, want substring %q", err, test.wantErr)
			}
			var difference *Difference
			if !errors.As(err, &difference) {
				t.Fatalf("CompareJSON() error type = %T, want *Difference", err)
			}
		})
	}
}

func TestCompareJSONRejectsTrailingValue(t *testing.T) {
	t.Parallel()
	err := CompareJSON([]byte(`{} {}`), []byte(`{}`), DefaultTolerance)
	if err == nil || !strings.Contains(err.Error(), "multiple JSON values") {
		t.Fatalf("CompareJSON() error = %v, want trailing-value error", err)
	}
}
