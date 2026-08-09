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
	"unicode/utf16"
)

const (
	canonicalDoubleNaN uint64 = 0x7ff8000000000000
	canonicalFloatNaN  uint32 = 0x7fc00000
)

// DoubleToLongBits implements Double.doubleToLongBits, including NaN
// canonicalization. Use math.Float64bits when Java's doubleToRawLongBits is
// specifically required.
func DoubleToLongBits(value float64) uint64 {
	if math.IsNaN(value) {
		return canonicalDoubleNaN
	}
	return math.Float64bits(value)
}

// FloatToIntBits implements Float.floatToIntBits, including NaN
// canonicalization.
func FloatToIntBits(value float32) uint32 {
	if math.IsNaN(float64(value)) {
		return canonicalFloatNaN
	}
	return math.Float32bits(value)
}

// DoubleCompare implements Double.compare. In particular, -0.0 sorts before
// +0.0 and every NaN sorts after positive infinity and compares equal to every
// other NaN.
func DoubleCompare(a, b float64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	aBits := int64(DoubleToLongBits(a))
	bBits := int64(DoubleToLongBits(b))
	if aBits == bBits {
		return 0
	}
	if aBits < bBits {
		return -1
	}
	return 1
}

// FloatCompare implements Float.compare.
func FloatCompare(a, b float32) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	aBits := int32(FloatToIntBits(a))
	bBits := int32(FloatToIntBits(b))
	if aBits == bBits {
		return 0
	}
	if aBits < bBits {
		return -1
	}
	return 1
}

// DoubleHashCode implements Double.hashCode(double).
func DoubleHashCode(value float64) int32 {
	bits := DoubleToLongBits(value)
	return int32(bits ^ (bits >> 32))
}

// FloatHashCode implements Float.hashCode(float).
func FloatHashCode(value float32) int32 { return int32(FloatToIntBits(value)) }

// LongHashCode implements Long.hashCode(long).
func LongHashCode(value int64) int32 {
	bits := uint64(value)
	return int32(bits ^ (bits >> 32))
}

// BooleanHashCode implements Boolean.hashCode(boolean).
func BooleanHashCode(value bool) int32 {
	if value {
		return 1231
	}
	return 1237
}

// StringHashCode implements String.hashCode over Java UTF-16 code units, not
// Unicode code points. This distinction matters for supplementary characters.
func StringHashCode(value string) int32 {
	var hash int32
	for _, unit := range utf16.Encode([]rune(value)) {
		hash = 31*hash + int32(unit)
	}
	return hash
}

// HashMapSpread reproduces the one-step high-bit spread observed by ELK's
// Java-compatible hash tables. Its behavior is pinned by independent vectors.
func HashMapSpread(hash int32) int32 { return hash ^ int32(uint32(hash)>>16) }

// IntCompare and LongCompare implement Integer.compare and Long.compare.
func IntCompare(a, b int32) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func LongCompare(a, b int64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// DoubleToInt and DoubleToLong implement Java's narrowing primitive casts:
// NaN becomes zero, finite out-of-range values and infinities saturate, and
// finite in-range values truncate toward zero.
func DoubleToInt(value float64) int32 {
	switch {
	case math.IsNaN(value):
		return 0
	case value >= float64(math.MaxInt32):
		return math.MaxInt32
	case value <= float64(math.MinInt32):
		return math.MinInt32
	default:
		return int32(value)
	}
}

func DoubleToLong(value float64) int64 {
	switch {
	case math.IsNaN(value):
		return 0
	// float64(MaxInt64) rounds to 2^63, which is precisely the first value
	// Java must saturate rather than convert.
	case value >= 0x1p63:
		return math.MaxInt64
	case value <= -0x1p63:
		return math.MinInt64
	default:
		return int64(value)
	}
}

// MathRoundDouble and MathRoundFloat implement Math.round.
func MathRoundDouble(value float64) int64 { return DoubleToLong(math.Floor(value + 0.5)) }

func MathRoundFloat(value float32) int32 {
	return DoubleToInt(math.Floor(float64(value) + 0.5))
}

// MathMinDouble and MathMaxDouble preserve Java's NaN and signed-zero rules.
func MathMinDouble(a, b float64) float64 {
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.NaN()
	}
	if a == 0 && b == 0 {
		if math.Signbit(a) || math.Signbit(b) {
			return math.Copysign(0, -1)
		}
		return 0
	}
	return math.Min(a, b)
}

func MathMaxDouble(a, b float64) float64 {
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.NaN()
	}
	if a == 0 && b == 0 {
		if !math.Signbit(a) || !math.Signbit(b) {
			return 0
		}
		return math.Copysign(0, -1)
	}
	return math.Max(a, b)
}

// UnsignedRightShiftInt and UnsignedRightShiftLong implement Java's >>>. Java
// masks the shift distance to five or six bits respectively.
func UnsignedRightShiftInt(value int32, distance uint) int32 {
	return int32(uint32(value) >> (distance & 31))
}

func UnsignedRightShiftLong(value int64, distance uint) int64 {
	return int64(uint64(value) >> (distance & 63))
}

// Narrowing integral conversions implement Java casts.
func IntToByte(value int32) int8   { return int8(value) }
func IntToShort(value int32) int16 { return int16(value) }
func IntToChar(value int32) uint16 { return uint16(value) }
