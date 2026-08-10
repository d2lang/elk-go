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

package javacompat

// GWTPriorityQueue reproduces GWT 2.11.0's java.util.PriorityQueue
// implementation linked into elkjs 0.12.0. The relevant GWT source is
// unchanged from the 2.9.0 runtime used by the former elkjs 0.8.2 baseline.
// Its bulk-add heapification and equal-item sift-down behavior differ
// observably from OpenJDK's PriorityQueue.
type GWTPriorityQueue[T any] struct {
	heap    []T
	compare Comparator[T]
}

func NewGWTPriorityQueue[T any](compare Comparator[T]) *GWTPriorityQueue[T] {
	if compare == nil {
		panic("javacompat.NewGWTPriorityQueue: nil comparator")
	}
	return &GWTPriorityQueue[T]{compare: compare}
}

func (queue *GWTPriorityQueue[T]) Len() int      { return len(queue.heap) }
func (queue *GWTPriorityQueue[T]) IsEmpty() bool { return len(queue.heap) == 0 }

// AddAll appends the complete collection and then invokes GWT's recursive
// makeHeap routine. It does not repeatedly call Offer.
func (queue *GWTPriorityQueue[T]) AddAll(items []T) bool {
	if len(items) == 0 {
		return false
	}
	queue.heap = append(queue.heap, items...)
	queue.makeHeap(0)
	return true
}

func (queue *GWTPriorityQueue[T]) Add(item T) { queue.Offer(item) }

func (queue *GWTPriorityQueue[T]) Offer(item T) bool {
	node := len(queue.heap)
	queue.heap = append(queue.heap, item)
	for node > 0 {
		child := node
		node = (node - 1) / 2
		if queue.compare(queue.heap[node], item) <= 0 {
			queue.heap[child] = item
			return true
		}
		queue.heap[child] = queue.heap[node]
	}
	queue.heap[node] = item
	return true
}

func (queue *GWTPriorityQueue[T]) Peek() (T, bool) {
	if len(queue.heap) == 0 {
		var zero T
		return zero, false
	}
	return queue.heap[0], true
}

func (queue *GWTPriorityQueue[T]) Poll() (T, bool) {
	if len(queue.heap) == 0 {
		var zero T
		return zero, false
	}
	value := queue.heap[0]
	queue.removeAt(0)
	return value, true
}

func (queue *GWTPriorityQueue[T]) makeHeap(node int) {
	if node*2+1 >= len(queue.heap) {
		return
	}
	queue.makeHeap(node*2 + 1)
	right := node*2 + 2
	if right < len(queue.heap) {
		queue.makeHeap(right)
	}
	queue.mergeHeaps(node)
}

func (queue *GWTPriorityQueue[T]) removeAt(index int) {
	last := len(queue.heap) - 1
	lastValue := queue.heap[last]
	var zero T
	queue.heap[last] = zero
	queue.heap = queue.heap[:last]
	if index < len(queue.heap) {
		queue.heap[index] = lastValue
		queue.mergeHeaps(index)
	}
}

func (queue *GWTPriorityQueue[T]) mergeHeaps(node int) {
	heapSize := len(queue.heap)
	value := queue.heap[node]
	for node*2+1 < heapSize {
		left := node*2 + 1
		right := left + 1
		smallest := left
		if right < heapSize && queue.compare(queue.heap[right], queue.heap[left]) < 0 {
			smallest = right
		}
		// GWT uses a strict comparison here, so equal values move down.
		if queue.compare(value, queue.heap[smallest]) < 0 {
			break
		}
		queue.heap[node] = queue.heap[smallest]
		node = smallest
	}
	queue.heap[node] = value
}
