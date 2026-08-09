// Copyright (c) 2011, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package force

import (
	"math"
	"testing"
)

func TestEadesForceFunctions(t *testing.T) {
	if got := eadesAttractive(0, 80); got != -100 {
		t.Fatalf("zero-distance attraction = %v, want -100", got)
	}
	if got := eadesRepulsive(0, 5); got != 500 {
		t.Fatalf("zero-distance repulsion = %v, want 500", got)
	}
	if got, want := eadesAttractive(160, 80), math.Log(2); got != want {
		t.Fatalf("attraction = %v, want %v", got, want)
	}
}

func TestFruchtermanReingoldForceFunctions(t *testing.T) {
	if got := frAttractive(10, 5); got != 20 {
		t.Fatalf("attraction = %v, want 20", got)
	}
	if got := frRepulsive(10, 5); got != 2.5 {
		t.Fatalf("repulsion = %v, want 2.5", got)
	}
	if got := frRepulsive(0, 5); got != 2500 {
		t.Fatalf("zero-distance repulsion = %v, want 2500", got)
	}
}
