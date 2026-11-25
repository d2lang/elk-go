// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package core

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitWords(t *testing.T) {
	for _, test := range []struct {
		text string
		want []string
	}{
		{"", nil},
		{"A", []string{"A"}},
		{"A B", []string{"A", "B"}},
		{"NONE LEFT_RIGHT x.y", []string{"NONE", "LEFT_RIGHT", "x.y"}},
	} {
		if got := splitWords(test.text); !reflect.DeepEqual(got, test.want) {
			t.Errorf("splitWords(%q) = %#v, want %#v", test.text, got, test.want)
		}
	}
}

// TestRegisterSpec checks that a spec registers the option it declares, with
// slices of its own, and that RegisterSpec and RegisterMissing treat an ID
// that is registered already as Register and the packages' loops did.
func TestRegisterSpec(t *testing.T) {
	registry := NewOptionRegistry()
	coreOptions := len(registry.Options())
	spec := OptionSpec{
		ID: "test.spec.option", Group: "spec", Kind: EnumOption, Targets: Parents | Nodes,
		LegacyIDs: "test.spec.old test.spec.older", EnumValues: "ONE TWO", MetadataOnly: true,
	}
	if err := registry.RegisterSpec(spec); err != nil {
		t.Fatal(err)
	}
	option := registry.ByID(spec.ID)
	if option == nil {
		t.Fatal("option not registered")
	}
	if option.Group != "spec" || option.Kind != EnumOption || option.Targets != Parents|Nodes || !option.MetadataOnly ||
		option.Parse != nil || option.pureParse ||
		!reflect.DeepEqual(option.LegacyIDs, []string{"test.spec.old", "test.spec.older"}) ||
		!reflect.DeepEqual(option.EnumValues, []string{"ONE", "TWO"}) {
		t.Fatalf("registered %+v", option)
	}
	if registry.ByID("test.spec.older") != option {
		t.Fatal("legacy ID does not resolve to the option")
	}
	if value, valid := option.ParseValue("TWO"); !valid || value != "TWO" {
		t.Fatalf("ParseValue(TWO) = %v, %v", value, valid)
	}

	if err := registry.RegisterSpec(spec); err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("second RegisterSpec: %v", err)
	}
	if err := registry.RegisterSpec(OptionSpec{}); err == nil {
		t.Fatal("RegisterSpec accepted an empty ID")
	}
	// RegisterMissing leaves a registered option as it is.
	registry.RegisterMissing(OptionSpec{ID: spec.ID, Kind: BooleanOption})
	if registry.ByID(spec.ID) != option || option.Kind != EnumOption || len(registry.Options()) != coreOptions+1 {
		t.Fatal("RegisterMissing replaced or added an option")
	}
	registry.RegisterMissing(OptionSpec{ID: "test.spec.point", Kind: ObjectOption, Parse: purePointParser})
	point := registry.ByID("test.spec.point")
	if point == nil || point.Parse == nil || !point.pureParse {
		t.Fatalf("registered %+v", point)
	}
}
