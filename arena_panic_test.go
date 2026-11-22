// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package elk

import (
	"testing"

	"github.com/d2lang/elk-go/internal/layered"
)

// drainArenas empties the arena pool and returns how many
// arenas it held (a released arena always holds a graph; a new one nothing).
func drainArenas() int {
	count := 0
	for {
		arena := layered.AcquireLGraphArena()
		if cap(arena.Graphs) == 0 {
			return count
		}
		count++
	}
}

// TestPanickingLayoutDropsItsArena: a layered layout that
// panics (the original panics on poisonGraph too, in crossing
// minimization) must not give its arena back to the pool: the arena and the
// processors' scratch kept in it may be half-updated. A layout that returns
// gives it back.
func TestPanickingLayoutDropsItsArena(t *testing.T) {
	drainArenas()
	_, _, panicked := crossminLayout(t, poisonGraph)
	if panicked == nil {
		t.Fatal("the poison graph did not panic")
	}
	if pooled := drainArenas(); pooled != 0 {
		t.Fatalf("a panicking layout pooled %d arena(s)", pooled)
	}
	if _, err, panicked := crossminLayout(t, victimGraph); err != nil || panicked != nil {
		t.Fatalf("victim: err %v, panic %v", err, panicked)
	}
	if pooled := drainArenas(); pooled != 1 {
		t.Fatalf("a completed layout pooled %d arenas, want 1", pooled)
	}
}
