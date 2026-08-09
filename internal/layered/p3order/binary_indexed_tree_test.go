// Copyright (c) 2016 Kiel University and others.
// SPDX-License-Identifier: EPL-2.0

package p3order

import "testing"

func TestBinaryIndexedTreeRankAndRemoveAll(t *testing.T) {
	tree := newBinaryIndexedTree(7)
	for _, value := range []int{4, 1, 4, 2, 6, 1} {
		tree.add(value)
	}
	wantRanks := []int{0, 0, 2, 3, 3, 5, 5, 6}
	for index, want := range wantRanks {
		if got := tree.rank(index); got != want {
			t.Fatalf("rank(%d) = %d, want %d", index, got, want)
		}
	}
	if tree.size != 6 || tree.empty() {
		t.Fatalf("unexpected size/empty: %d/%v", tree.size, tree.empty())
	}

	tree.removeAll(4)
	if tree.size != 4 || tree.rank(7) != 4 || tree.rank(5) != 3 {
		t.Fatalf("removeAll left size=%d rank(7)=%d rank(5)=%d", tree.size, tree.rank(7), tree.rank(5))
	}
	tree.removeAll(4) // idempotent
	tree.clear()
	if !tree.empty() || tree.rank(7) != 0 {
		t.Fatalf("clear left size=%d rank(7)=%d", tree.size, tree.rank(7))
	}
}
