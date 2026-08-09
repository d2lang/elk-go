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

func TestOrderedMapLinkedHashMapSemantics(t *testing.T) {
	var m OrderedMap[string, int]
	if _, replaced := m.Put("b", 2); replaced {
		t.Fatal("first Put replaced")
	}
	m.Put("a", 1)
	m.Put("c", 3)
	if old, replaced := m.Put("a", 10); !replaced || old != 1 {
		t.Fatalf("replacement = (%d, %v)", old, replaced)
	}
	if want := []string{"b", "a", "c"}; !reflect.DeepEqual(m.Keys(), want) {
		t.Fatalf("after replace got %v, want %v", m.Keys(), want)
	}
	if old, ok := m.Remove("a"); !ok || old != 10 {
		t.Fatalf("Remove = (%d, %v)", old, ok)
	}
	m.Put("a", 11)
	if want := []string{"b", "c", "a"}; !reflect.DeepEqual(m.Keys(), want) {
		t.Fatalf("after reinsert got %v, want %v", m.Keys(), want)
	}
	if want := []int{2, 3, 11}; !reflect.DeepEqual(m.Values(), want) {
		t.Fatalf("values got %v, want %v", m.Values(), want)
	}
	firstKey, firstValue, ok := m.First()
	if !ok || firstKey != "b" || firstValue != 2 {
		t.Fatalf("First = (%q, %d, %v)", firstKey, firstValue, ok)
	}
	lastKey, lastValue, ok := m.Last()
	if !ok || lastKey != "a" || lastValue != 11 {
		t.Fatalf("Last = (%q, %d, %v)", lastKey, lastValue, ok)
	}
}

func TestOrderedMapRangeAndClear(t *testing.T) {
	m := NewOrderedMap[int, string]()
	for i, value := range []string{"zero", "one", "two"} {
		m.Put(i, value)
	}
	var got []string
	m.Range(func(_ int, value string) bool {
		got = append(got, value)
		return len(got) < 2
	})
	if want := []string{"zero", "one"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	m.Clear()
	if !m.IsEmpty() || m.Len() != 0 {
		t.Fatal("not empty after Clear")
	}
	m.Put(4, "four")
	if got, ok := m.Get(4); !ok || got != "four" {
		t.Fatalf("zero value unusable after Clear: (%q, %v)", got, ok)
	}
}

func TestOrderedSetSemantics(t *testing.T) {
	var set OrderedSet[int]
	for _, value := range []int{4, 2, 4, 1} {
		set.Add(value)
	}
	if want := []int{4, 2, 1}; !reflect.DeepEqual(set.Values(), want) {
		t.Fatalf("got %v, want %v", set.Values(), want)
	}
	set.Remove(2)
	set.Add(2)
	if want := []int{4, 1, 2}; !reflect.DeepEqual(set.Values(), want) {
		t.Fatalf("got %v, want %v", set.Values(), want)
	}
}

func TestIdentityCollectionsUsePointerIdentity(t *testing.T) {
	type value struct{ n int }
	a := &value{n: 1}
	b := &value{n: 1}
	var m IdentityMap[*value, string]
	m.Put(a, "a")
	m.Put(b, "b")
	if m.Len() != 2 {
		t.Fatalf("structurally equal objects collapsed: len=%d", m.Len())
	}
	if got, _ := m.Get(a); got != "a" {
		t.Fatalf("Get(a) = %q", got)
	}
	var set IdentitySet[*value]
	if !set.Add(a) || set.Add(a) || !set.Add(b) || set.Len() != 2 {
		t.Fatalf("unexpected identity set: %v", set.Values())
	}
}
