// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package oracle provides the pinned elkjs 0.12.0 differential oracle used
// while porting ELK to Go. It is test and development infrastructure, not part
// of elk-go's public layout API.
package oracle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"sort"
	"strconv"
)

// Tolerance controls comparison of finite coordinate values. Every other
// value, including IDs, array order, field presence, topology, and section
// counts, must match exactly.
type Tolerance struct {
	Absolute float64
	Relative float64
}

// DefaultTolerance permits only sub-micro-pixel differences at ordinary D2
// diagram scales.
var DefaultTolerance = Tolerance{Absolute: 1e-7, Relative: 1e-9}

// ExactTolerance requires byte-level numeric equality after JSON numbers are
// parsed. Use it for deterministic provider smoke cases; a narrow coordinate
// tolerance can otherwise hide a reproducible Java/GWT runtime mismatch.
var ExactTolerance = Tolerance{}

// Difference is the first structural or value mismatch found in two outputs.
type Difference struct {
	Path   string
	Reason string
	Want   any
	Got    any
}

func (d *Difference) Error() string {
	return fmt.Sprintf("ELK output mismatch at %s: %s (want %s, got %s)", d.Path, d.Reason, render(d.Want), render(d.Got))
}

// CompareJSON compares two ELK JSON results. Object member order is irrelevant,
// but member presence and every array's order are strict. The GWT-only `$H`
// identity hash is discarded recursively on both sides.
func CompareJSON(wantJSON, gotJSON []byte, tolerance Tolerance) error {
	want, err := decode(wantJSON)
	if err != nil {
		return fmt.Errorf("decode expected ELK JSON: %w", err)
	}
	got, err := decode(gotJSON)
	if err != nil {
		return fmt.Errorf("decode candidate ELK JSON: %w", err)
	}
	stripGWTIdentityHashes(want)
	stripGWTIdentityHashes(got)
	return compare("$", "", want, got, tolerance)
}

func decode(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("multiple JSON values")
		}
		return nil, err
	}
	return value, nil
}

func stripGWTIdentityHashes(value any) {
	switch value := value.(type) {
	case map[string]any:
		delete(value, "$H")
		for _, child := range value {
			stripGWTIdentityHashes(child)
		}
	case []any:
		for _, child := range value {
			stripGWTIdentityHashes(child)
		}
	}
}

func compare(path, field string, want, got any, tolerance Tolerance) error {
	if wantNumber, ok := want.(json.Number); ok {
		gotNumber, ok := got.(json.Number)
		if !ok {
			return mismatch(path, "JSON types differ", want, got)
		}
		if isCoordinate(field) {
			return compareCoordinate(path, wantNumber, gotNumber, tolerance)
		}
		if !numbersEqual(wantNumber, gotNumber) {
			return mismatch(path, "non-coordinate numbers differ", want, got)
		}
		return nil
	}

	switch want := want.(type) {
	case map[string]any:
		got, ok := got.(map[string]any)
		if !ok {
			return mismatch(path, "JSON types differ", want, got)
		}
		keys := make([]string, 0, len(want))
		for key := range want {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			gotValue, exists := got[key]
			if !exists {
				return mismatch(memberPath(path, key), "field is missing", want[key], nil)
			}
			if err := compare(memberPath(path, key), key, want[key], gotValue, tolerance); err != nil {
				return err
			}
		}
		extra := make([]string, 0)
		for key := range got {
			if _, exists := want[key]; !exists {
				extra = append(extra, key)
			}
		}
		if len(extra) > 0 {
			sort.Strings(extra)
			key := extra[0]
			return mismatch(memberPath(path, key), "unexpected field is present", nil, got[key])
		}
		return nil
	case []any:
		got, ok := got.([]any)
		if !ok {
			return mismatch(path, "JSON types differ", want, got)
		}
		if len(want) != len(got) {
			return mismatch(path, "array length differs", len(want), len(got))
		}
		for index := range want {
			if err := compare(fmt.Sprintf("%s[%d]", path, index), "", want[index], got[index], tolerance); err != nil {
				return err
			}
		}
		return nil
	case string:
		if got, ok := got.(string); !ok || want != got {
			return mismatch(path, "strings differ", want, got)
		}
		return nil
	case bool:
		if got, ok := got.(bool); !ok || want != got {
			return mismatch(path, "booleans differ", want, got)
		}
		return nil
	case nil:
		if got != nil {
			return mismatch(path, "JSON types differ", want, got)
		}
		return nil
	default:
		return mismatch(path, "unsupported decoded JSON type", want, got)
	}
}

func compareCoordinate(path string, want, got json.Number, tolerance Tolerance) error {
	wantFloat, err := strconv.ParseFloat(want.String(), 64)
	if err != nil || math.IsInf(wantFloat, 0) || math.IsNaN(wantFloat) {
		return mismatch(path, "expected coordinate is not finite", want, got)
	}
	gotFloat, err := strconv.ParseFloat(got.String(), 64)
	if err != nil || math.IsInf(gotFloat, 0) || math.IsNaN(gotFloat) {
		return mismatch(path, "candidate coordinate is not finite", want, got)
	}
	difference := math.Abs(wantFloat - gotFloat)
	limit := math.Max(tolerance.Absolute, tolerance.Relative*math.Max(math.Abs(wantFloat), math.Abs(gotFloat)))
	if difference > limit {
		return mismatch(path, fmt.Sprintf("coordinates differ by %.12g (limit %.12g)", difference, limit), want, got)
	}
	return nil
}

func numbersEqual(left, right json.Number) bool {
	leftRat, leftOK := new(big.Rat).SetString(left.String())
	rightRat, rightOK := new(big.Rat).SetString(right.String())
	return leftOK && rightOK && leftRat.Cmp(rightRat) == 0
}

func isCoordinate(field string) bool {
	switch field {
	case "x", "y", "width", "height":
		return true
	default:
		return false
	}
}

func mismatch(path, reason string, want, got any) error {
	return &Difference{Path: path, Reason: reason, Want: want, Got: got}
}

func memberPath(path, member string) string {
	if member != "" {
		valid := true
		for index, r := range member {
			if !(r == '_' || r == '$' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || index > 0 && r >= '0' && r <= '9') {
				valid = false
				break
			}
		}
		if valid {
			return path + "." + member
		}
	}
	encoded, _ := json.Marshal(member)
	return path + "[" + string(encoded) + "]"
}

func render(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	if len(encoded) > 160 {
		return string(encoded[:157]) + "..."
	}
	return string(encoded)
}
