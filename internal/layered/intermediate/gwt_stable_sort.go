// Copyright 2008 Google Inc.
// Copyright (c) 2026 The elk-go Authors.
// Adapted to Go for elk-go in 2026.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package intermediate

// gwtStableSort is a Go adaptation of the legacy merge sort in GWT 2.9.0's
// emulated java.util.Arrays. Comparator invocation order matters here because
// ModelOrderNodeComparator records comparisons to make later comparisons
// transitive. The seven-element insertion-sort cutoff and already-sorted copy
// fast path therefore intentionally match the frozen JavaScript oracle.
func gwtStableSort[T any](values []T, compare func(T, T) int) {
	auxiliary := append([]T(nil), values...)
	gwtMergeSort(auxiliary, values, 0, len(values), compare)
}

func gwtMergeSort[T any](source, destination []T, low, high int, compare func(T, T) int) {
	length := high - low
	if length < 7 {
		for index := low + 1; index < high; index++ {
			for current := index; current > low && compare(destination[current-1], destination[current]) > 0; current-- {
				destination[current], destination[current-1] = destination[current-1], destination[current]
			}
		}
		return
	}

	middle := low + (length >> 1)
	gwtMergeSort(destination, source, low, middle, compare)
	gwtMergeSort(destination, source, middle, high, compare)
	if compare(source[middle-1], source[middle]) <= 0 {
		copy(destination[low:high], source[low:high])
		return
	}

	left, right := low, middle
	for index := low; index < high; index++ {
		if right >= high || left < middle && compare(source[left], source[right]) <= 0 {
			destination[index] = source[left]
			left++
		} else {
			destination[index] = source[right]
			right++
		}
	}
}
