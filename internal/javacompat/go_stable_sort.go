// Copyright 2009 The Go Authors.
// Copyright (c) 2026 The elk-go Authors.
//
// Use of this source code is governed by a BSD-style license that can be
// found in the Go distribution's LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause

package javacompat

// GoStableSort sorts values exactly as sort.SliceStable(values, less') does
// when less'(i, j) == less(values[i], values[j]): it is the same insertion
// sort on blocks of 20 followed by the same SymMerge passes, so it performs
// the same comparisons on the same elements in the same order and leaves the
// same permutation, even for comparators that are not strict weak orders.
// Unlike sort.SliceStable it needs no reflection and allocates nothing.
func GoStableSort[T any](values []T, less func(T, T) bool) {
	n := len(values)
	blockSize := 20 // must be > 0
	a, b := 0, blockSize
	for b <= n {
		goInsertionSort(values, a, b, less)
		a = b
		b += blockSize
	}
	goInsertionSort(values, a, n, less)

	for blockSize < n {
		a, b = 0, 2*blockSize
		for b <= n {
			goSymMerge(values, a, a+blockSize, b, less)
			a = b
			b += 2 * blockSize
		}
		if m := a + blockSize; m < n {
			goSymMerge(values, a, m, n, less)
		}
		blockSize *= 2
	}
}

func goInsertionSort[T any](data []T, a, b int, less func(T, T) bool) {
	for i := a + 1; i < b; i++ {
		for j := i; j > a && less(data[j], data[j-1]); j-- {
			data[j], data[j-1] = data[j-1], data[j]
		}
	}
}

func goSymMerge[T any](data []T, a, m, b int, less func(T, T) bool) {
	if m-a == 1 {
		i := m
		j := b
		for i < j {
			h := int(uint(i+j) >> 1)
			if less(data[h], data[a]) {
				i = h + 1
			} else {
				j = h
			}
		}
		for k := a; k < i-1; k++ {
			data[k], data[k+1] = data[k+1], data[k]
		}
		return
	}

	if b-m == 1 {
		i := a
		j := m
		for i < j {
			h := int(uint(i+j) >> 1)
			if !less(data[m], data[h]) {
				i = h + 1
			} else {
				j = h
			}
		}
		for k := m; k > i; k-- {
			data[k], data[k-1] = data[k-1], data[k]
		}
		return
	}

	mid := int(uint(a+b) >> 1)
	n := mid + m
	var start, r int
	if m > mid {
		start = n - b
		r = mid
	} else {
		start = a
		r = m
	}
	p := n - 1

	for start < r {
		c := int(uint(start+r) >> 1)
		if !less(data[p-c], data[c]) {
			start = c + 1
		} else {
			r = c
		}
	}

	end := n - start
	if start < m && m < end {
		goRotate(data, start, m, end)
	}
	if a < start && start < mid {
		goSymMerge(data, a, start, mid, less)
	}
	if mid < end && end < b {
		goSymMerge(data, mid, end, b, less)
	}
}

func goSwapRange[T any](data []T, a, b, n int) {
	for i := 0; i < n; i++ {
		data[a+i], data[b+i] = data[b+i], data[a+i]
	}
}

func goRotate[T any](data []T, a, m, b int) {
	i := m - a
	j := b - m
	for i != j {
		if i > j {
			goSwapRange(data, m-i, m, j)
			i -= j
		} else {
			goSwapRange(data, m-i, m+j-i, i)
			j -= i
		}
	}
	goSwapRange(data, m-i, m, i)
}
