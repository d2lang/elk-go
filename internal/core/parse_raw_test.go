// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package core

import (
	"encoding/json"
	"math"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func sameParsed(left, right any) bool {
	leftFloat, leftIsFloat := left.(float64)
	rightFloat, rightIsFloat := right.(float64)
	if leftIsFloat || rightIsFloat {
		return leftIsFloat && rightIsFloat && math.Float64bits(leftFloat) == math.Float64bits(rightFloat)
	}
	return reflect.DeepEqual(left, right)
}

// TestParseRawMatchesParseValue checks parseRaw against the text round trip
// it replaces, for every option kind and awkward values.
func TestParseRawMatchesParseValue(t *testing.T) {
	registry := NewOptionRegistry()
	for _, option := range []OptionData{
		{ID: "test.string", Kind: StringOption},
		{ID: "test.integer", Kind: IntegerOption},
		{ID: "test.double", Kind: DoubleOption},
		{ID: "test.enum", Kind: EnumOption, EnumValues: []string{"A", "B", "1", "C"}},
	} {
		if err := registry.Register(option); err != nil {
			t.Fatal(err)
		}
	}
	raws := []any{nil, "", "null", "NULL", " 8 ", "8", "8.5d", "1e3", "A", "B", "C", "1", "2", "4", "-1", "true",
		"V_TOP H_LEFT", "[top=1,left=2]", "(3, 4)", true, false,
		json.Number("8"), json.Number("1e400"), int(7), int64(-3), float32(2.5),
		0.0, math.Copysign(0, -1), 1.0, -1.0, 8.0, 0.5, 1e-7, 999999.0, -999999.0, 1e6, -1e6, 1234567.0,
		2147483647.0, 2147483648.0, 1e21, 5e-324, math.MaxFloat64, math.NaN(), math.Inf(1), math.Inf(-1),
		math.Float64frombits(0x7ff8000000000123)}
	rng := rand.New(rand.NewSource(3))
	for i := 0; i < 2000; i++ {
		raws = append(raws, math.Float64frombits(rng.Uint64()), float64(rng.Intn(4000000)-2000000),
			rng.NormFloat64()*math.Pow(10, float64(rng.Intn(40)-20)))
	}
	for _, option := range registry.Options() {
		for _, raw := range raws {
			gotValue, gotValid := option.parseRaw(raw)
			wantValue, wantValid := option.ParseValue(optionString(raw))
			if gotValid != wantValid || gotValid && !sameParsed(gotValue, wantValue) {
				t.Fatalf("%s %v(%T): parseRaw = %#v %v, ParseValue = %#v %v",
					option.ID, raw, raw, gotValue, gotValid, wantValue, wantValid)
			}
		}
	}
}

// TestIntegralFloatsFormatAsIntegers backs parseRaw's IntegerOption case:
// below 1e6, FormatFloat('g', -1) writes integral values as plain decimals.
func TestIntegralFloatsFormatAsIntegers(t *testing.T) {
	for value := -999999; value <= 999999; value++ {
		if got := strconv.FormatFloat(float64(value), 'g', -1, 64); got != strconv.Itoa(value) {
			t.Fatalf("FormatFloat(%d) = %q", value, got)
		}
	}
	if got := strconv.FormatFloat(math.Copysign(0, -1), 'g', -1, 64); got != "-0" {
		t.Fatalf("FormatFloat(-0) = %q", got)
	}
	if parsed, err := strconv.ParseInt("-0", 10, 32); err != nil || parsed != 0 {
		t.Fatalf("ParseInt(-0) = %d, %v", parsed, err)
	}
}

// TestParseEnumSetMatchesFieldsFunc checks the byte scan against the
// strings.FieldsFunc split it replaced, invalid UTF-8 included.
func TestParseEnumSetMatchesFieldsFunc(t *testing.T) {
	values := []string{"V_TOP", "H_LEFT", "é", "\xff", "A B"}
	reference := func(value string) (any, bool) {
		parts := strings.FieldsFunc(value, func(r rune) bool { return strings.ContainsRune("[] ,\t\r\n", r) })
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			parsed, ok := parseEnum(part, values)
			if !ok {
				return nil, false
			}
			result = append(result, parsed)
		}
		return result, true
	}
	alphabet := []string{"V_TOP", "H_LEFT", "é", "\xff", "\xc3", "\xe2\x82", "0", "1", "4", "x", "[", "]", " ", ",", "\t", "\r", "\n", " "}
	rng := rand.New(rand.NewSource(5))
	inputs := []string{"", " ", "[]", "[V_TOP, H_LEFT]", "V_TOP H_LEFT", "0,1", "\xff", "\xc3,", "é,\xff", "[,]"}
	for i := 0; i < 20000; i++ {
		var b strings.Builder
		for n := rng.Intn(6); n > 0; n-- {
			b.WriteString(alphabet[rng.Intn(len(alphabet))])
		}
		inputs = append(inputs, b.String())
	}
	for _, input := range inputs {
		got, gotOK := parseEnumSet(input, values)
		want, wantOK := reference(input)
		if gotOK != wantOK || !reflect.DeepEqual(got, want) {
			t.Fatalf("parseEnumSet(%q) = %#v %v, want %#v %v", input, got, gotOK, want, wantOK)
		}
		if gotOK && cap(got.([]string)) != cap(want.([]string)) {
			t.Fatalf("parseEnumSet(%q) capacity %d, want %d", input, cap(got.([]string)), cap(want.([]string)))
		}
	}
}
