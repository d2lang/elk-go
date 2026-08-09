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

func TestRandomJavaCompatibilityVectors(t *testing.T) {
	t.Run("nextInt", func(t *testing.T) {
		r := NewRandom(42)
		want := []int32{-1170105035, 234785527, -1360544799, 205897768, 1325939940}
		for i, expected := range want {
			if got := r.NextInt(); got != expected {
				t.Fatalf("value %d: got %d, want %d", i, got, expected)
			}
		}
	})

	t.Run("nextIntBound", func(t *testing.T) {
		for _, tc := range []struct {
			seed  int64
			bound int32
			want  []int32
		}{
			{0, 100, []int32{60, 48, 29, 47, 15}},
			{42, 16, []int32{11, 0, 10, 0, 4}},
			{42, 7, []int32{1, 5, 6, 3, 5}},
			{math.MinInt64, math.MaxInt32, []int32{1569741360, 1785505948, 516548029}},
		} {
			r := NewRandom(tc.seed)
			for i, expected := range tc.want {
				if got := r.NextIntN(tc.bound); got != expected {
					t.Fatalf("seed=%d bound=%d value %d: got %d, want %d", tc.seed, tc.bound, i, got, expected)
				}
			}
		}
	})

	t.Run("mixed", func(t *testing.T) {
		r := NewRandom(42)
		if got := r.NextLong(); got != -5025562857975149833 {
			t.Errorf("NextLong: got %d", got)
		}
		if got := r.NextBoolean(); got != true {
			t.Errorf("NextBoolean: got %v", got)
		}
		if got := r.NextFloat(); math.Float32bits(got) != math.Float32bits(0.0479393) {
			t.Errorf("NextFloat: got %.9g (%08x)", got, math.Float32bits(got))
		}
		if got := r.NextDouble(); got != 0.30871945533265976 {
			t.Errorf("NextDouble: got %.17g", got)
		}
	})

	t.Run("setSeed", func(t *testing.T) {
		r := NewRandom(1)
		_ = r.NextInt()
		r.SetSeed(42)
		if got := r.NextInt(); got != -1170105035 {
			t.Fatalf("got %d", got)
		}
	})

	t.Run("bytes", func(t *testing.T) {
		r := NewRandom(42)
		got := make([]byte, 10)
		r.NextBytes(got)
		want := []byte{0x35, 0x9d, 0x41, 0xba, 0xf7, 0x8a, 0xfe, 0x0d, 0xe1, 0xbb}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got %x, want %x", got, want)
			}
		}
	})
}

func TestRandomPanicsForInvalidBound(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NextIntN did not panic")
		}
	}()
	NewRandom(0).NextIntN(0)
}

func TestRandomGaussianCacheAndSeedReset(t *testing.T) {
	r := NewRandom(42)
	first := r.NextGaussian()
	second := r.NextGaussian()
	if first != 1.1419053154730547 || second != 0.9194079489827879 {
		t.Fatalf("got (%0.17g, %0.17g)", first, second)
	}
	r.SetSeed(42)
	if got := r.NextGaussian(); got != first {
		t.Fatalf("after reset got %.17g, want %.17g", got, first)
	}
}
