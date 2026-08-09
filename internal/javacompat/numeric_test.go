// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package javacompat

import (
	"math"
	"testing"
)

func TestJavaFloatingCompare(t *testing.T) {
	negativeZero := math.Copysign(0, -1)
	values := []float64{math.Inf(-1), -1, negativeZero, 0, 1, math.Inf(1), math.NaN()}
	for i, a := range values {
		for j, b := range values {
			got := DoubleCompare(a, b)
			want := IntCompare(int32(i), int32(j))
			if got != want {
				t.Fatalf("DoubleCompare(%v, %v) = %d, want %d", a, b, got, want)
			}
		}
	}
	if FloatCompare(float32(negativeZero), 0) >= 0 {
		t.Fatal("FloatCompare did not order -0.0 before +0.0")
	}
	if FloatCompare(float32(math.NaN()), float32(math.Inf(1))) <= 0 {
		t.Fatal("FloatCompare did not order NaN after +Inf")
	}
}

func TestJavaHashCodes(t *testing.T) {
	if got := DoubleHashCode(1); got != 1072693248 {
		t.Fatalf("DoubleHashCode(1) = %d", got)
	}
	if got := FloatHashCode(1); got != 1065353216 {
		t.Fatalf("FloatHashCode(1) = %d", got)
	}
	if got := LongHashCode(0x123456789abcdef0); got != -2004318072 {
		t.Fatalf("LongHashCode = %#x", uint32(got))
	}
	if StringHashCode("hello") != 99162322 {
		t.Fatalf("StringHashCode(hello) = %d", StringHashCode("hello"))
	}
	// U+1F600 is the UTF-16 surrogate pair D83D DE00.
	if got := StringHashCode("😀"); got != 1772899 {
		t.Fatalf("StringHashCode(emoji) = %d", got)
	}
	if DoubleHashCode(math.Float64frombits(0x7ff0000000000001)) != DoubleHashCode(math.NaN()) {
		t.Fatal("NaNs were not canonicalized")
	}
}

func TestJavaNarrowingFloatingCasts(t *testing.T) {
	intCases := []struct {
		value float64
		want  int32
	}{
		{math.NaN(), 0}, {math.Inf(1), math.MaxInt32}, {math.Inf(-1), math.MinInt32},
		{2147483646.9, 2147483646}, {-2147483647.9, -2147483647}, {1.9, 1}, {-1.9, -1},
	}
	for _, tc := range intCases {
		if got := DoubleToInt(tc.value); got != tc.want {
			t.Errorf("DoubleToInt(%v) = %d, want %d", tc.value, got, tc.want)
		}
	}
	if DoubleToLong(math.Inf(1)) != math.MaxInt64 || DoubleToLong(math.Inf(-1)) != math.MinInt64 {
		t.Fatal("DoubleToLong did not saturate infinities")
	}
	if DoubleToLong(math.NaN()) != 0 || DoubleToLong(-12.9) != -12 {
		t.Fatal("DoubleToLong did not implement Java cast")
	}
}

func TestJavaMathSignedZeroAndRound(t *testing.T) {
	negativeZero := math.Copysign(0, -1)
	if !math.Signbit(MathMinDouble(0, negativeZero)) {
		t.Fatal("MathMinDouble lost negative zero")
	}
	if math.Signbit(MathMaxDouble(0, negativeZero)) {
		t.Fatal("MathMaxDouble returned negative zero")
	}
	if MathRoundDouble(2.5) != 3 || MathRoundDouble(-2.5) != -2 {
		t.Fatal("MathRoundDouble differs from Java")
	}
	if MathRoundFloat(2.5) != 3 || MathRoundFloat(-2.5) != -2 {
		t.Fatal("MathRoundFloat differs from Java")
	}
}

func TestJavaUnsignedShiftMasksDistance(t *testing.T) {
	if got := UnsignedRightShiftInt(-1, 1); got != math.MaxInt32 {
		t.Fatalf("int >>> = %d", got)
	}
	if got := UnsignedRightShiftInt(-1, 33); got != math.MaxInt32 {
		t.Fatalf("masked int >>> = %d", got)
	}
	if got := UnsignedRightShiftLong(-1, 1); got != math.MaxInt64 {
		t.Fatalf("long >>> = %d", got)
	}
}
