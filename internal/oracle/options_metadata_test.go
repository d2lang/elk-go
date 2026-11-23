// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package oracle

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/d2lang/elk-go/internal/builtins"
	"github.com/d2lang/elk-go/internal/core"
)

type oracleOptionMetadata struct {
	ID      string   `json:"id"`
	Group   string   `json:"group"`
	Type    string   `json:"type"`
	Targets []string `json:"targets"`
}

func TestKnownLayoutOptionsParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_OPTIONS") != "1" {
		t.Skip("set ELK_GO_ORACLE_OPTIONS=1 to compare the complete option metadata surface")
	}
	runner := DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	raw, err := runner.KnownLayoutOptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var reference []oracleOptionMetadata
	if err := json.Unmarshal(raw, &reference); err != nil {
		t.Fatalf("decode reference option metadata: %v", err)
	}

	registry := core.NewOptionRegistry()
	builtins.RegisterOptions(registry)
	want := make(map[string]oracleOptionMetadata, len(reference))
	for _, option := range reference {
		want[option.ID] = option
	}
	got := make(map[string]*core.OptionData, len(registry.Options()))
	for _, option := range registry.Options() {
		if option.ID != "d2.rankGroups" { // Native Go extension.
			got[option.ID] = option
		}
	}
	if len(want) != len(got) {
		t.Errorf("known option count = %d, want %d", len(got), len(want))
	}
	ids := make([]string, 0, len(want)+len(got))
	seen := make(map[string]struct{}, len(want)+len(got))
	for id := range want {
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	for id := range got {
		if _, found := seen[id]; !found {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		expected, inOracle := want[id]
		actual, inGo := got[id]
		if !inOracle {
			t.Errorf("Go registry exposes option absent from elkjs %s: %s", OracleVersion, id)
			continue
		}
		if !inGo {
			t.Errorf("Go registry is missing elkjs %s option: %s", OracleVersion, id)
			continue
		}
		if actual.Group != expected.Group {
			t.Errorf("option %s group = %q, want %q", id, actual.Group, expected.Group)
		}
		if gotType := optionTypeName(actual.Kind); gotType != expected.Type {
			t.Errorf("option %s type = %s, want %s", id, gotType, expected.Type)
		}
		if gotTargets, err := optionTargets(expected.Targets); err != nil {
			t.Errorf("option %s: %v", id, err)
		} else if actual.Targets != gotTargets {
			t.Errorf("option %s targets = %v, want %v", id, actual.Targets, gotTargets)
		}
	}
}

func optionTypeName(kind core.OptionKind) string {
	switch kind {
	case core.StringOption:
		return "STRING"
	case core.BooleanOption:
		return "BOOLEAN"
	case core.IntegerOption:
		return "INT"
	case core.DoubleOption:
		return "DOUBLE"
	case core.EnumOption:
		return "ENUM"
	case core.EnumSetOption:
		return "ENUMSET"
	case core.ObjectOption:
		return "OBJECT"
	case core.UndefinedOption:
		return "UNDEFINED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", kind)
	}
}

func optionTargets(values []string) (core.Targets, error) {
	var targets core.Targets
	for _, value := range values {
		switch value {
		case "PARENTS":
			targets |= core.Parents
		case "NODES":
			targets |= core.Nodes
		case "EDGES":
			targets |= core.Edges
		case "PORTS":
			targets |= core.Ports
		case "LABELS":
			targets |= core.Labels
		default:
			return 0, fmt.Errorf("unknown reference target %q", value)
		}
	}
	return targets, nil
}
