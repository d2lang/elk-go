// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package reuse

import (
	"runtime"
	"testing"
)

// TestOversizedValuesReusableUntilCollected: under gc a value above
// its keep-bound goes to the sync.Pool, so that a large layout run right after
// another finds its storage, and the collector empties it; under TinyGo it is
// dropped. The race detector drops some sync.Pool
// puts at random, so the reuse is checked over several tries.
func TestOversizedValuesReusableUntilCollected(t *testing.T) {
	var pool Pool[[]int]
	large := new([]int)
	reused := false
	for try := 0; try < 64 && !reused; try++ {
		pool.Put(large, false)
		if pool.slot != nil {
			t.Fatal("an oversized value went to the slot")
		}
		got := pool.Get()
		if got != nil && got != large {
			t.Fatalf("Get = %p, want nil or %p", got, large)
		}
		reused = got == large
	}
	if reused != OversizedToPool {
		t.Fatalf("oversized value reused: %v, want %v (OversizedToPool)", reused, OversizedToPool)
	}
	pool.Put(large, false)
	runtime.GC()
	runtime.GC()
	if got := pool.Get(); got != nil {
		t.Fatalf("Get = %p after two collections, want nil", got)
	}
}
