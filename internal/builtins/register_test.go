// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package builtins

import (
	"reflect"
	"testing"

	"github.com/d2lang/elk-go/internal/core"
)

func TestELKJS0120AlgorithmRegistry(t *testing.T) {
	options := core.NewOptionRegistry()
	RegisterOptions(options)
	algorithms := core.NewAlgorithmRegistry()
	if err := Register(algorithms); err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(algorithms.Algorithms()))
	for _, algorithm := range algorithms.Algorithms() {
		got = append(got, algorithm.ID)
	}
	want := []string{
		"org.eclipse.elk.fixed",
		"org.eclipse.elk.box",
		"org.eclipse.elk.random",
		"org.eclipse.elk.layered",
		"org.eclipse.elk.stress",
		"org.eclipse.elk.mrtree",
		"org.eclipse.elk.radial",
		"org.eclipse.elk.force",
		"org.eclipse.elk.sporeOverlap",
		"org.eclipse.elk.sporeCompaction",
		"org.eclipse.elk.rectpacking",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("algorithm registry = %v, want %v", got, want)
	}
	for index, id := range want {
		if got := algorithms.BySuffix(id[len("org.eclipse.elk."):]); got == nil || got.ID != id {
			t.Fatalf("suffix lookup for %q at %d = %#v", id, index, got)
		}
	}
}
