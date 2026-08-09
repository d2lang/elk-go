// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package javacompat

// Comparator has the contract of java.util.Comparator.compare: negative when
// a is less than b, zero when they compare equal, and positive otherwise.
type Comparator[T any] func(a, b T) int

// PriorityQueue is a clean-room binary min-heap whose tie behavior is pinned by
// elkjs oracle vectors. Java does not define FIFO ordering for equal elements,
// but ELK observes the heap-array order, so left-child and strict-comparison
// choices are part of this compatibility type's tested behavior.
type PriorityQueue[T any] struct {
	heap    []T
	compare Comparator[T]
}

// NewPriorityQueue returns an empty Java-compatible min-priority queue.
func NewPriorityQueue[T any](compare Comparator[T]) *PriorityQueue[T] {
	if compare == nil {
		panic("javacompat.NewPriorityQueue: nil comparator")
	}
	return &PriorityQueue[T]{compare: compare}
}

// NewPriorityQueueCapacity is NewPriorityQueue with a capacity hint. Java's
// growth policy does not affect heap order, but avoiding reallocations is handy
// for the larger layered-layout queues.
func NewPriorityQueueCapacity[T any](capacity int, compare Comparator[T]) *PriorityQueue[T] {
	if capacity < 0 {
		panic("javacompat.NewPriorityQueueCapacity: negative capacity")
	}
	q := NewPriorityQueue(compare)
	q.heap = make([]T, 0, capacity)
	return q
}

// Len returns the number of queued elements.
func (q *PriorityQueue[T]) Len() int { return len(q.heap) }

// IsEmpty reports whether the queue is empty.
func (q *PriorityQueue[T]) IsEmpty() bool { return len(q.heap) == 0 }

// Add inserts item. Java's add and offer have identical heap behavior.
func (q *PriorityQueue[T]) Add(item T) {
	q.Offer(item)
}

// Offer inserts item and returns true, matching PriorityQueue.offer.
func (q *PriorityQueue[T]) Offer(item T) bool {
	k := len(q.heap)
	q.heap = append(q.heap, item)
	q.siftUp(k, item)
	return true
}

// Peek returns the least element without removing it.
func (q *PriorityQueue[T]) Peek() (T, bool) {
	if len(q.heap) == 0 {
		var zero T
		return zero, false
	}
	return q.heap[0], true
}

// Poll removes and returns the least element.
func (q *PriorityQueue[T]) Poll() (T, bool) {
	n := len(q.heap)
	if n == 0 {
		var zero T
		return zero, false
	}
	result := q.heap[0]
	last := n - 1
	moved := q.heap[last]
	var zero T
	q.heap[last] = zero
	q.heap = q.heap[:last]
	if last != 0 {
		q.siftDown(0, moved)
	}
	return result, true
}

// RemoveFirst removes the first element in the queue's internal array for
// which match returns true. This is PriorityQueue.remove(Object) with equality
// supplied explicitly, which also supports non-comparable Go values.
func (q *PriorityQueue[T]) RemoveFirst(match func(T) bool) bool {
	if match == nil {
		panic("javacompat.PriorityQueue.RemoveFirst: nil predicate")
	}
	for i, item := range q.heap {
		if match(item) {
			q.removeAt(i)
			return true
		}
	}
	return false
}

// Clear removes every element while releasing references held by the backing
// slice.
func (q *PriorityQueue[T]) Clear() {
	clear(q.heap)
	q.heap = q.heap[:0]
}

// Elements returns a copy in PriorityQueue iterator order (the heap-array
// order). This order is deliberately not sorted, just like Java's iterator.
func (q *PriorityQueue[T]) Elements() []T {
	return append([]T(nil), q.heap...)
}

func (q *PriorityQueue[T]) siftUp(index int, item T) int {
	q.heap[index] = item
	for index > 0 {
		parent := (index - 1) / 2
		if q.compare(q.heap[index], q.heap[parent]) >= 0 {
			break
		}
		q.heap[index], q.heap[parent] = q.heap[parent], q.heap[index]
		index = parent
	}
	return index
}

func (q *PriorityQueue[T]) siftDown(index int, item T) int {
	q.heap[index] = item
	for {
		left := index*2 + 1
		if left >= len(q.heap) {
			return index
		}
		selected := left
		right := left + 1
		if right < len(q.heap) && q.compare(q.heap[left], q.heap[right]) > 0 {
			selected = right
		}
		if q.compare(q.heap[index], q.heap[selected]) <= 0 {
			return index
		}
		q.heap[index], q.heap[selected] = q.heap[selected], q.heap[index]
		index = selected
	}
}

func (q *PriorityQueue[T]) removeAt(i int) {
	last := len(q.heap) - 1
	if last == i {
		var zero T
		q.heap[last] = zero
		q.heap = q.heap[:last]
		return
	}
	moved := q.heap[last]
	var zero T
	q.heap[last] = zero
	q.heap = q.heap[:last]
	if q.siftDown(i, moved) == i {
		q.siftUp(i, moved)
	}
}
