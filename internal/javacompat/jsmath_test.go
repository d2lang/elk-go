// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package javacompat

import (
	"math"
	"runtime"
	"testing"
)

func TestJSMathSinCosV8Bits(t *testing.T) {
	tests := []struct {
		x, sin, cos uint64
	}{
		{0x400155b8c11ee451, 0x3fea7b5256ed3ae0, 0xbfe1f6e04d506bdc},
		{0x3ff3333333333333, 0x3fedd343a21a55c4, 0x3fd730de943b79d4},
		{0x3fe0c152382d7365, 0x3fdfffffffffffff, 0x3febb67ae8584cab},
		{0x400921fb54442d18, 0x3ca1a62633145c07, 0xbff0000000000000},
		{0x401921fb54442d18, 0xbcb1a62633145c07, 0x3ff0000000000000},
		// Radial's nested top-down wedge boundary exercises the high end of
		// kernel_sin's reduced interval. V8 contracts the source expression
		// differently in Node.js 24.19.0's official arm64 and x64 builds; the
		// architecture-specific cosine expectation is selected below.
		{0x4002d813136b9487, 0x3fe6a49c4d0af50d, 0xbfe69c9fcb8c8c23},
	}
	for _, test := range tests {
		x := math.Float64frombits(test.x)
		if got := math.Float64bits(JSMathSin(x)); got != test.sin {
			t.Errorf("sin(%x) = %x, want %x", test.x, got, test.sin)
		}
		wantCos := test.cos
		if test.x == 0x4002d813136b9487 {
			switch runtime.GOARCH {
			case "arm64":
				// Verified with the official Node.js v24.19.0 darwin-arm64 binary.
			case "amd64":
				// Verified with the official Node.js v24.19.0 darwin-x64 binary.
				// Its V8 build returns one ULP below the official arm64 build.
				wantCos = 0xbfe69c9fcb8c8c24
			default:
				t.Logf("cos(%x): no verified official Node.js expectation for %s", test.x, runtime.GOARCH)
				continue
			}
		}
		if got := math.Float64bits(JSMathCos(x)); got != wantCos {
			t.Errorf("cos(%x) on %s = %x, want %x", test.x, runtime.GOARCH, got, wantCos)
		}
	}
}
