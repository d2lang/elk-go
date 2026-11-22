// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package reuse

import (
	"runtime"
	"testing"
)

// TestPoolKeepsOnlyBoundedValuesAcrossGC: a value released with
// keep=false (more than a bounded amount of storage) never goes to the slot,
// which survives garbage collection, and is gone after two collections.
// Under TinyGo it is dropped (TinyGo's sync.Pool never frees what it holds); under gc it may go to the sync.Pool,
// which collections empty.
// Bounded values go to the slot, then to the sync.Pool; the slot's value
// survives the collections.
func TestPoolKeepsOnlyBoundedValuesAcrossGC(t *testing.T) {
	var pool Pool[[]int]
	large := new([]int)
	pool.Put(large, false)
	if pool.slot != nil {
		t.Fatal("an unbounded value went to the slot")
	}
	got := pool.Get()
	switch {
	case !OversizedToPool && got != nil:
		t.Fatalf("Get = %p after releasing only an unbounded value, want nil (dropped)", got)
	case OversizedToPool && got != nil && got != large:
		t.Fatalf("Get = %p, want nil or the unbounded value %p", got, large)
	}
	first, second := new([]int), new([]int)
	pool.Put(first, true)
	pool.Put(second, true)
	pool.Put(large, false)
	if pool.slot != first {
		t.Fatalf("slot = %p, want the first bounded value %p", pool.slot, first)
	}
	// The slot keeps its value across collections; whatever went to the
	// sync.Pool is gone after two (the victim cache holds it for one).
	runtime.GC()
	runtime.GC()
	kept := map[*[]int]bool{}
	for value := pool.Get(); value != nil; value = pool.Get() {
		kept[value] = true
	}
	if kept[large] {
		t.Fatal("an unbounded value survived two collections")
	}
	if !kept[first] {
		t.Fatal("the slot's value was lost")
	}
}
