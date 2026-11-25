// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package builtins

import (
	"testing"

	"github.com/d2lang/elk-go/internal/core"
)

func TestIndividualSpacingOptionParsesNestedOverrides(t *testing.T) {
	registry := core.NewOptionRegistry()
	RegisterOptions(registry)
	option := registry.ByID(core.SpacingIndividual.ID)
	if option == nil {
		t.Fatal("individual spacing option is not registered")
	}
	parsed, ok := option.ParseValue("spacing.portPort:17;,;spacing.nodeNode:25")
	if !ok {
		t.Fatal("individual spacing value was rejected")
	}
	individual, ok := parsed.(*core.IndividualSpacings)
	if !ok {
		t.Fatalf("individual spacing value has type %T", parsed)
	}
	if value, found := individual.Properties.Raw("org.eclipse.elk.spacing.portPort"); !found || value != 17.0 {
		t.Fatalf("portPort override = %#v, found %v", value, found)
	}
	if value, found := individual.Properties.Raw("org.eclipse.elk.spacing.nodeNode"); !found || value != 25.0 {
		t.Fatalf("nodeNode override = %#v, found %v", value, found)
	}
}

// TestCompatibilityCorrectionsListEachOptionOnce checks that no option is
// listed twice in a table of corrections: the later entry would silently win.
func TestCompatibilityCorrectionsListEachOptionOnce(t *testing.T) {
	seen := map[string]bool{}
	for _, correction := range compatibilityGroups {
		if seen[correction.id] {
			t.Errorf("compatibilityGroups lists %q twice", correction.id)
		}
		seen[correction.id] = true
	}
	seen = map[string]bool{}
	for _, correction := range compatibilityTargets {
		if seen[correction.id] {
			t.Errorf("compatibilityTargets lists %q twice", correction.id)
		}
		seen[correction.id] = true
	}
}
