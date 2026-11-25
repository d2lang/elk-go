// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package graph

import (
	"reflect"
	"testing"
)

type constantTestBox struct{ top, right, bottom, left float64 }

// sameBoxedValue reports whether two interface values hold the same value:
// the same dynamic type and, for a slice, the same elements in the same
// backing array.
func sameBoxedValue(first, second any) bool {
	if reflect.TypeOf(first) != reflect.TypeOf(second) {
		return false
	}
	value := reflect.ValueOf(first)
	if value.Kind() == reflect.Slice {
		other := reflect.ValueOf(second)
		return value.Len() == other.Len() && value.Pointer() == other.Pointer()
	}
	return first == second
}

// TestConstantDefaultBoxIsTheClone checks that the box a constant default is
// materialised from is what cloneValue gives for the default value: the
// constructor stores the value without calling cloneValue.
func TestConstantDefaultBoxIsTheClone(t *testing.T) {
	check := func(name string, box, value any) {
		t.Helper()
		if !sameBoxedValue(box, cloneValue(value)) {
			t.Fatalf("%s: box %#v is not cloneValue of %#v", name, box, value)
		}
	}
	structProperty := CloneablePropertyWithConstantDefault("test.constant.struct",
		func() constantTestBox { return constantTestBox{1, 2, 3, 4} })
	check("struct", structProperty.constant.box, structProperty.Default())

	names := []string{"V_TOP", "H_LEFT"}
	sliceProperty := CloneablePropertyWithConstantDefault("test.constant.strings", func() []string { return names })
	check("[]string", sliceProperty.constant.box, sliceProperty.Default())

	nilProperty := CloneablePropertyWithConstantDefault("test.constant.nilStrings", func() []string { return nil })
	check("nil []string", nilProperty.constant.box, nilProperty.Default())
	if nilProperty.constant.box == nil {
		t.Fatal("a nil slice default must still be boxed as a slice")
	}

	// Get materialises that box, in every map.
	for round := 0; round < 2; round++ {
		var properties PropertyMap
		if got := Get(&properties, structProperty); got != (constantTestBox{1, 2, 3, 4}) {
			t.Fatalf("Get = %#v", got)
		}
		stored, found := properties.RawKey(structProperty.Key())
		if !found || !sameBoxedValue(stored, structProperty.constant.box) {
			t.Fatalf("materialised %#v, found %v", stored, found)
		}
		if got := Get(&properties, sliceProperty); len(got) != 2 || &got[0] != &names[0] {
			t.Fatalf("Get = %#v", got)
		}
		stored, found = properties.RawKey(sliceProperty.Key())
		if !found || !sameBoxedValue(stored, sliceProperty.constant.box) {
			t.Fatalf("materialised %#v, found %v", stored, found)
		}
	}
}

// TestInternPropertyKeyOrder checks that keys are assigned in the order in
// which new IDs are interned, starting after the keys assigned so far, and
// that interning an ID again returns its key.
func TestInternPropertyKeyOrder(t *testing.T) {
	ids := []string{"test.intern.order.a", "test.intern.order.b", "test.intern.order.c"}
	first := InternPropertyKey(ids[0])
	if first == 0 {
		t.Fatal("key 0 was assigned")
	}
	for index, id := range ids {
		if key := InternPropertyKey(id); key != first+PropertyKey(index) {
			t.Fatalf("InternPropertyKey(%q) = %d, want %d", id, key, first+PropertyKey(index))
		}
	}
	for index, id := range ids {
		if key := InternPropertyKey(id); key != first+PropertyKey(index) {
			t.Fatalf("second InternPropertyKey(%q) = %d, want %d", id, key, first+PropertyKey(index))
		}
		if key, found := LookupPropertyKey(id); !found || key != first+PropertyKey(index) {
			t.Fatalf("LookupPropertyKey(%q) = %d, %v", id, key, found)
		}
		if name := propertyKeyName(first + PropertyKey(index)); name != id {
			t.Fatalf("propertyKeyName(%d) = %q, want %q", first+PropertyKey(index), name, id)
		}
	}
	if _, found := LookupPropertyKey("test.intern.order.never"); found {
		t.Fatal("LookupPropertyKey found an ID that was never interned")
	}
}
