// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package javacompat contains the small parts of the Java runtime whose exact
// behavior is observable in ELK layouts.
package javacompat

import "math"

const (
	randomMultiplier uint64 = 0x5DEECE66D
	randomAddend     uint64 = 0xB
	randomMask       uint64 = (1 << 48) - 1
)

// Random is a bit-for-bit implementation of java.util.Random from Java 8.
//
// Random is not safe for concurrent use. ELK owns one instance per layout, as
// the Java implementation does, so synchronization would only obscure the
// order in which random values are consumed.
type Random struct {
	seed             uint64
	haveNextGaussian bool
	nextGaussian     float64
}

// NewRandom constructs a Java-compatible pseudo-random number generator.
func NewRandom(seed int64) *Random {
	r := &Random{}
	r.SetSeed(seed)
	return r
}

// SetSeed implements Random.setSeed(long). It also clears the cached value
// used by NextGaussian.
func (r *Random) SetSeed(seed int64) {
	r.seed = (uint64(seed) ^ randomMultiplier) & randomMask
	r.haveNextGaussian = false
}

// next implements Random.next(int). Only Java's public Random methods should
// call it; the valid range is 1..32.
func (r *Random) next(bits uint) int32 {
	if bits == 0 || bits > 32 {
		panic("javacompat.Random.next: bits must be in [1, 32]")
	}
	r.seed = (r.seed*randomMultiplier + randomAddend) & randomMask
	return int32(r.seed >> (48 - bits))
}

// NextInt implements Random.nextInt().
func (r *Random) NextInt() int32 {
	return r.next(32)
}

// NextIntN implements Random.nextInt(bound). It panics for a non-positive
// bound, corresponding to Java's IllegalArgumentException.
func (r *Random) NextIntN(bound int32) int32 {
	if bound <= 0 {
		panic("javacompat.Random.NextIntN: bound must be positive")
	}
	if bound&-bound == bound {
		return int32((int64(bound) * int64(r.next(31))) >> 31)
	}
	for {
		bits := r.next(31)
		value := bits % bound
		// The expression deliberately has int32 overflow semantics. Java uses
		// the overflow to reject the small biased tail of the 31-bit range.
		if bits-value+(bound-1) >= 0 {
			return value
		}
	}
}

// NextLong implements Random.nextLong(). The low 32 bits are sign-extended
// before addition, as required by Java's two signed int32 draws.
func (r *Random) NextLong() int64 {
	return (int64(r.next(32)) << 32) + int64(r.next(32))
}

// NextBoolean implements Random.nextBoolean().
func (r *Random) NextBoolean() bool {
	return r.next(1) != 0
}

// NextFloat implements Random.nextFloat().
func (r *Random) NextFloat() float32 {
	return float32(r.next(24)) / float32(1<<24)
}

// NextDouble implements Random.nextDouble().
func (r *Random) NextDouble() float64 {
	high := int64(r.next(26)) << 27
	low := int64(r.next(27))
	return float64(high+low) / float64(int64(1)<<53)
}

// NextBytes implements Random.nextBytes(byte[]). Java's byte is signed, but
// assigning its low eight bits to a Go byte gives the same byte sequence.
func (r *Random) NextBytes(dst []byte) {
	for i := 0; i < len(dst); {
		value := r.NextInt()
		remaining := len(dst) - i
		if remaining > 4 {
			remaining = 4
		}
		for ; remaining > 0; remaining-- {
			dst[i] = byte(value)
			i++
			value >>= 8
		}
	}
}

// NextGaussian implements Random.nextGaussian(), including its cached second
// sample. ELK 0.8.1 does not use this in a layout path, but providing the Java
// behavior prevents consumers from accidentally substituting math/rand.
// Math-library results may differ by a final bit across platforms, just as
// Java's Math implementation may, while the random-value consumption is exact.
func (r *Random) NextGaussian() float64 {
	if r.haveNextGaussian {
		r.haveNextGaussian = false
		return r.nextGaussian
	}
	for {
		v1 := 2*r.NextDouble() - 1
		v2 := 2*r.NextDouble() - 1
		s := v1*v1 + v2*v2
		if s >= 1 || s == 0 {
			continue
		}
		multiplier := math.Sqrt(-2 * math.Log(s) / s)
		r.nextGaussian = v2 * multiplier
		r.haveNextGaussian = true
		return v1 * multiplier
	}
}
