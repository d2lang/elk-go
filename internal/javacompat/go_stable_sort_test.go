// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package javacompat

import (
	"math/rand"
	"sort"
	"testing"
)

type stableSortItem struct{ key, id int }

// TestGoStableSortMatchesSliceStable checks the replica against
// sort.SliceStable: same permutation and the same comparison sequence, for
// consistent comparators with ties and for inconsistent ones.
func TestGoStableSortMatchesSliceStable(t *testing.T) {
	random := rand.New(rand.NewSource(1))
	comparators := map[string]func(a, b stableSortItem) bool{
		"keys": func(a, b stableSortItem) bool { return a.key < b.key },
		"inconsistent": func(a, b stableSortItem) bool {
			return (a.id*7919+b.id*104729+a.key)%3 == 0
		},
	}
	for name, less := range comparators {
		for length := 0; length < 260; length++ {
			for round := 0; round < 3; round++ {
				values := make([]stableSortItem, length)
				for index := range values {
					values[index] = stableSortItem{key: random.Intn(length/3 + 1), id: index}
				}
				expected := append([]stableSortItem(nil), values...)
				var expectedCalls, actualCalls [][2]int
				sort.SliceStable(expected, func(i, j int) bool {
					expectedCalls = append(expectedCalls, [2]int{expected[i].id, expected[j].id})
					return less(expected[i], expected[j])
				})
				actual := append([]stableSortItem(nil), values...)
				GoStableSort(actual, func(a, b stableSortItem) bool {
					actualCalls = append(actualCalls, [2]int{a.id, b.id})
					return less(a, b)
				})
				for index := range expected {
					if expected[index] != actual[index] {
						t.Fatalf("%s length %d: permutation differs at %d", name, length, index)
					}
				}
				if len(expectedCalls) != len(actualCalls) {
					t.Fatalf("%s length %d: %d comparisons, want %d", name, length, len(actualCalls), len(expectedCalls))
				}
				for index := range expectedCalls {
					if expectedCalls[index] != actualCalls[index] {
						t.Fatalf("%s length %d: comparison %d differs", name, length, index)
					}
				}
			}
		}
	}
}
