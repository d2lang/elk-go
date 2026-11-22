// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package graph

import (
	"fmt"
	"math/rand"
	"testing"
)

// TestPropertyMapMatchesOrderedList checks lookups, updates, deletes,
// copies and recycling against an insertion-ordered list, for maps from a
// few entries to more than 255 (where the slot index gives up).
func TestPropertyMapMatchesOrderedList(t *testing.T) {
	random := rand.New(rand.NewSource(3))
	keys := make([]string, 600)
	for index := range keys {
		keys[index] = fmt.Sprintf("test.random.%d", index)
	}
	type entry struct {
		id    string
		value int
	}
	find := func(list []entry, id string) int {
		for index, candidate := range list {
			if candidate.id == id {
				return index
			}
		}
		return -1
	}
	for round := 0; round < 300; round++ {
		universe := 1 + random.Intn(len(keys))
		var properties PropertyMap
		var reference []entry
		if round%3 == 0 {
			// start from recycled storage
			var old PropertyMap
			for index := 0; index < random.Intn(40); index++ {
				old.SetRaw(keys[random.Intn(universe)], index)
			}
			properties = old.Recycled()
		}
		for step := 0; step < 4*universe; step++ {
			id := keys[random.Intn(universe)]
			switch random.Intn(10) {
			case 0:
				properties.Delete(id)
				if index := find(reference, id); index >= 0 {
					reference = append(reference[:index], reference[index+1:]...)
				}
			case 1, 2, 3, 4:
				properties.SetRaw(id, step)
				if index := find(reference, id); index >= 0 {
					reference[index].value = step
				} else {
					reference = append(reference, entry{id, step})
				}
			default:
				value, found := properties.Raw(id)
				index := find(reference, id)
				if found != (index >= 0) || found && value != reference[index].value {
					t.Fatalf("round %d step %d: Raw(%s) = %v,%v; want index %d", round, step, id, value, found, index)
				}
			}
		}
		entries := properties.Entries()
		if len(entries) != len(reference) {
			t.Fatalf("round %d: %d entries, want %d", round, len(entries), len(reference))
		}
		for index := range entries {
			if entries[index].ID != reference[index].id || entries[index].Value != reference[index].value {
				t.Fatalf("round %d: entry %d = %v, want %v", round, index, entries[index], reference[index])
			}
		}
		clone := properties.Clone()
		for _, item := range reference {
			if value, found := clone.Raw(item.id); !found || value != item.value {
				t.Fatalf("round %d: clone lost %s", round, item.id)
			}
		}
	}
}

func TestSetIntMatchesSet(t *testing.T) {
	property := NewProperty[int]("test.setint")
	for _, value := range []int{-3, -2, -1, 0, 1, 255, 256, 257, 4094, 4095, 4096, 1 << 40} {
		var viaSet, viaSetInt PropertyMap
		Set(&viaSet, property, value)
		SetInt(&viaSetInt, property, value)
		got, found := TryGet(&viaSetInt, property)
		want, _ := TryGet(&viaSet, property)
		if !found || got != want {
			t.Fatalf("SetInt(%d) stored %d", value, got)
		}
		raw, _ := viaSetInt.RawKey(property.Key())
		if _, isInt := raw.(int); !isInt {
			t.Fatalf("SetInt(%d) stored a %T", value, raw)
		}
	}
	// Every value of every chunk of boxes, twice (made, then reused).
	for round := 0; round < 2; round++ {
		for value := -2; value <= maxBoxedInt+1; value++ {
			if boxed, isInt := boxInt(value).(int); !isInt || boxed != value {
				t.Fatalf("boxInt(%d) = %v", value, boxInt(value))
			}
		}
	}
}
