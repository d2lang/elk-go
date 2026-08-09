// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package javacompat

import (
	"reflect"
	"testing"
)

func intComparator(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func TestPriorityQueuePollOrder(t *testing.T) {
	q := NewPriorityQueue(intComparator)
	for _, value := range []int{5, 1, 4, 2, 3} {
		q.Add(value)
	}
	var got []int
	for !q.IsEmpty() {
		value, ok := q.Poll()
		if !ok {
			t.Fatal("Poll returned false for non-empty queue")
		}
		got = append(got, value)
	}
	if want := []int{1, 2, 3, 4, 5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestPriorityQueueEqualElementsUseJavaHeapOrder(t *testing.T) {
	type item struct {
		priority int
		name     string
	}
	q := NewPriorityQueue(func(a, b item) int { return intComparator(a.priority, b.priority) })
	for _, name := range []string{"a", "b", "c", "d"} {
		q.Add(item{priority: 1, name: name})
	}
	var got []string
	for !q.IsEmpty() {
		value, _ := q.Poll()
		got = append(got, value.name)
	}
	// PriorityQueue is not stable. This is the oracle-pinned observable sift order.
	if want := []string{"a", "d", "c", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestPriorityQueueComparatorCallSequence(t *testing.T) {
	var calls [][2]int
	q := NewPriorityQueue(func(a, b int) int {
		calls = append(calls, [2]int{a, b})
		return intComparator(a, b)
	})
	for _, value := range []int{7, 2, 5, 1, 6, 3, 4} {
		q.Add(value)
	}
	if got, want := calls, [][2]int{{2, 7}, {5, 2}, {1, 7}, {1, 2}, {6, 2}, {3, 5}, {3, 1}, {4, 3}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("offer calls:\n got %v\nwant %v", got, want)
	}
	calls = nil
	if got, _ := q.Poll(); got != 1 {
		t.Fatalf("got %d", got)
	}
	if want := [][2]int{{2, 3}, {4, 2}, {7, 6}, {4, 6}}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("poll calls:\n got %v\nwant %v", calls, want)
	}
}

func TestPriorityQueueRemoveFirstUsesJavaRemoveAt(t *testing.T) {
	q := NewPriorityQueue(intComparator)
	for _, value := range []int{1, 4, 2, 8, 5, 3, 7, 9, 10, 6} {
		q.Add(value)
	}
	if !q.RemoveFirst(func(v int) bool { return v == 8 }) {
		t.Fatal("did not remove present value")
	}
	if q.RemoveFirst(func(v int) bool { return v == 99 }) {
		t.Fatal("removed absent value")
	}
	var got []int
	for !q.IsEmpty() {
		v, _ := q.Poll()
		got = append(got, v)
	}
	if want := []int{1, 2, 3, 4, 5, 6, 7, 9, 10}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestPriorityQueuePeekElementsAndClear(t *testing.T) {
	q := NewPriorityQueueCapacity(8, intComparator)
	if _, ok := q.Peek(); ok {
		t.Fatal("empty Peek returned a value")
	}
	q.Add(2)
	q.Add(1)
	if got, ok := q.Peek(); !ok || got != 1 {
		t.Fatalf("Peek = (%d, %v)", got, ok)
	}
	copyOfHeap := q.Elements()
	copyOfHeap[0] = 99
	if got, _ := q.Peek(); got != 1 {
		t.Fatal("Elements exposed backing array")
	}
	q.Clear()
	if q.Len() != 0 || !q.IsEmpty() {
		t.Fatalf("queue not empty after Clear: %v", q.Elements())
	}
}
