// The following is adapted from fdlibm (http://www.netlib.org/fdlibm).
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunSoft, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
// The original source code covered by the above license has been modified
// significantly by Google Inc.
// Copyright 2016 the V8 project authors. All rights reserved.
// Copyright (c) 2026 The elk-go Authors.
//
// The V8-derived portions are licensed under the BSD 3-Clause License; see
// LICENSE.v8-bsd. The fdlibm-derived portions retain the SunPro notice above.
//
// SPDX-License-Identifier: SunPro AND BSD-3-Clause

package javacompat

import "math"

// JSMathSin and JSMathCos reproduce the fdlibm implementation used by V8's
// Math.sin and Math.cos for the medium-size arguments emitted by ELK Radial.
// The kernels and constants are adapted from Node.js v24.19.0's pinned V8
// deps/v8/src/base/ieee754.cc (itself adapted from fdlibm). Very large
// arguments fall back to Go's math package; layout angles are many orders of
// magnitude below that boundary.
func JSMathSin(x float64) float64 {
	ix := highWord(x) & 0x7fffffff
	if ix <= 0x3fe921fb {
		return jsKernelSin(x, 0, false)
	}
	if ix >= 0x7ff00000 {
		return x - x
	}
	n, y0, y1, ok := jsRemPiOverTwo(x)
	if !ok {
		return math.Sin(x)
	}
	switch n & 3 {
	case 0:
		return jsKernelSin(y0, y1, true)
	case 1:
		return jsKernelCos(y0, y1)
	case 2:
		return -jsKernelSin(y0, y1, true)
	default:
		return -jsKernelCos(y0, y1)
	}
}

func JSMathCos(x float64) float64 {
	ix := highWord(x) & 0x7fffffff
	if ix <= 0x3fe921fb {
		return jsKernelCos(x, 0)
	}
	if ix >= 0x7ff00000 {
		return x - x
	}
	n, y0, y1, ok := jsRemPiOverTwo(x)
	if !ok {
		return math.Cos(x)
	}
	switch n & 3 {
	case 0:
		return jsKernelCos(y0, y1)
	case 1:
		return -jsKernelSin(y0, y1, true)
	case 2:
		return -jsKernelCos(y0, y1)
	default:
		return jsKernelSin(y0, y1, true)
	}
}

func jsRemPiOverTwo(x float64) (int32, float64, float64, bool) {
	const (
		half    = 5.00000000000000000000e-01
		invpio2 = 6.36619772367581382433e-01
		pio2_1  = 1.57079632673412561417e+00
		pio2_1t = 6.07710050650619224932e-11
		pio2_2  = 6.07710050630396597660e-11
		pio2_2t = 2.02226624879595063154e-21
		pio2_3  = 2.02226624871116645580e-21
		pio2_3t = 8.47842766036889956997e-32
	)
	npio2HighWords := [...]uint32{
		0x3ff921fb, 0x400921fb, 0x4012d97c, 0x401921fb, 0x401f6a7a, 0x4022d97c,
		0x4025fdbb, 0x402921fb, 0x402c463a, 0x402f6a7a, 0x4031475c, 0x4032d97c,
		0x40346b9c, 0x4035fdbb, 0x40378fdb, 0x403921fb, 0x403ab41b, 0x403c463a,
		0x403dd85a, 0x403f6a7a, 0x40407e4c, 0x4041475c, 0x4042106c, 0x4042d97c,
		0x4043a28c, 0x40446b9c, 0x404534ac, 0x4045fdbb, 0x4046c6cb, 0x40478fdb,
		0x404858eb, 0x404921fb,
	}
	hx := highWord(x)
	ix := hx & 0x7fffffff
	if ix <= 0x3fe921fb {
		return 0, x, 0, true
	}
	if ix < 0x4002d97c {
		if int32(hx) > 0 {
			z := jsSub(x, pio2_1)
			if ix != 0x3ff921fb {
				y0 := jsSub(z, pio2_1t)
				return 1, y0, jsSub(jsSub(z, y0), pio2_1t), true
			}
			z = jsSub(z, pio2_2)
			y0 := jsSub(z, pio2_2t)
			return 1, y0, jsSub(jsSub(z, y0), pio2_2t), true
		}
		z := jsAdd(x, pio2_1)
		if ix != 0x3ff921fb {
			y0 := jsAdd(z, pio2_1t)
			return -1, y0, jsAdd(jsSub(z, y0), pio2_1t), true
		}
		z = jsAdd(z, pio2_2)
		y0 := jsAdd(z, pio2_2t)
		return -1, y0, jsAdd(jsSub(z, y0), pio2_2t), true
	}
	if ix > 0x413921fb {
		return 0, 0, 0, false
	}
	t := math.Abs(x)
	n := int32(jsAdd(jsMul(t, invpio2), half))
	fn := float64(n)
	r := jsSub(t, jsMul(fn, pio2_1))
	w := jsMul(fn, pio2_1t)
	y0 := 0.0
	if n < 32 && ix != npio2HighWords[n-1] {
		y0 = jsSub(r, w)
	} else {
		j := int(ix >> 20)
		y0 = jsSub(r, w)
		high := highWord(y0)
		i := j - int((high>>20)&0x7ff)
		if i > 16 {
			t = r
			w = jsMul(fn, pio2_2)
			r = jsSub(t, w)
			w = jsSub(jsMul(fn, pio2_2t), jsSub(jsSub(t, r), w))
			y0 = jsSub(r, w)
			high = highWord(y0)
			i = j - int((high>>20)&0x7ff)
			if i > 49 {
				t = r
				w = jsMul(fn, pio2_3)
				r = jsSub(t, w)
				w = jsSub(jsMul(fn, pio2_3t), jsSub(jsSub(t, r), w))
				y0 = jsSub(r, w)
			}
		}
	}
	y1 := jsSub(jsSub(r, y0), w)
	if int32(hx) < 0 {
		return -n, -y0, -y1, true
	}
	return n, y0, y1, true
}

func jsKernelCos(x, y float64) float64 {
	const (
		one = 1.00000000000000000000e+00
		c1  = 4.16666666666666019037e-02
		c2  = -1.38888888888741095749e-03
		c3  = 2.48015872894767294178e-05
		c4  = -2.75573143513906633035e-07
		c5  = 2.08757232129817482790e-09
		c6  = -1.13596475577881948265e-11
	)
	ix := highWord(x) & 0x7fffffff
	if ix < 0x3e400000 && int(x) == 0 {
		return one
	}
	z := jsMul(x, x)
	polynomial := jsAdd(c5, jsMul(z, c6))
	polynomial = jsAdd(c4, jsMul(z, polynomial))
	polynomial = jsAdd(c3, jsMul(z, polynomial))
	polynomial = jsAdd(c2, jsMul(z, polynomial))
	polynomial = jsAdd(c1, jsMul(z, polynomial))
	r := jsMul(z, polynomial)
	zrMinusXY := jsSub(jsMul(z, r), jsMul(x, y))
	if ix < 0x3fd33333 {
		return jsSub(one, jsSub(jsMul(.5, z), zrMinusXY))
	}
	qx := .28125
	if ix <= 0x3fe90000 {
		qx = math.Float64frombits(uint64(ix-0x00200000) << 32)
	}
	iz := jsSub(jsMul(.5, z), qx)
	a := jsSub(one, qx)
	return jsSub(a, jsSub(iz, zrMinusXY))
}

func jsKernelSin(x, y float64, hasTail bool) float64 {
	const (
		half = 5.00000000000000000000e-01
		s1   = -1.66666666666666324348e-01
		s2   = 8.33333333332248946124e-03
		s3   = -1.98412698298579493134e-04
		s4   = 2.75573137070700676789e-06
		s5   = -2.50507602534068634195e-08
		s6   = 1.58969099521155010221e-10
	)
	ix := highWord(x) & 0x7fffffff
	if ix < 0x3e400000 && int(x) == 0 {
		return x
	}
	// Keep these source-shaped expressions unbarriered. The official Node.js
	// 24.19.0 V8 builds contract them on arm64 but not x64, and that one-ULP
	// architecture difference is observable in Math.cos. Go reproduces the same
	// per-architecture results; forcing jsMul/jsAdd boundaries here would not.
	z := x * x
	v := z * x
	r := s2 + z*(s3+z*(s4+z*(s5+z*s6)))
	if !hasTail {
		return x + v*(s1+z*r)
	}
	return x - ((z*(half*y-v*r) - y) - v*s1)
}

func highWord(value float64) uint32 { return uint32(math.Float64bits(value) >> 32) }

// Go permits fused multiply-add across ordinary expression boundaries. These
// non-inline helpers preserve fdlibm's binary64 operation boundaries.
//
//go:noinline
func jsAdd(a, b float64) float64 { return a + b }

//go:noinline
func jsSub(a, b float64) float64 { return a - b }

//go:noinline
func jsMul(a, b float64) float64 { return a * b }
