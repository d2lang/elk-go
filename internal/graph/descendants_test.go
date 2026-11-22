// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package graph

import (
	"math/rand"
	"slices"
	"testing"
)

// TestDescendantsPreOrder compares Descendants with a recursive depth-first
// pre-order on random trees.
func TestDescendantsPreOrder(t *testing.T) {
	random := rand.New(rand.NewSource(5))
	for round := 0; round < 200; round++ {
		model := NewModel()
		for count := random.Intn(60); count > 0; count-- {
			parent := NodeID(random.Intn(len(model.Nodes)))
			model.AddNode(&parent)
		}
		for node := range model.Nodes {
			var want []NodeID
			var visit func(NodeID)
			visit = func(current NodeID) {
				for _, child := range model.Node(current).Children {
					want = append(want, child)
					visit(child)
				}
			}
			visit(NodeID(node))
			if got := model.Descendants(NodeID(node)); !slices.Equal(got, want) || got == nil {
				t.Fatalf("round %d node %d: Descendants = %v, want %v", round, node, got, want)
			}
		}
	}
}
