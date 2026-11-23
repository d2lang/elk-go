// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package builtins registers the layout providers compiled into ELK.js 0.12.0.
package builtins

import (
	"github.com/d2lang/elk-go/internal/basic"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/force"
	"github.com/d2lang/elk-go/internal/layered"
	"github.com/d2lang/elk-go/internal/layeredlayout"
	"github.com/d2lang/elk-go/internal/mrtree"
	"github.com/d2lang/elk-go/internal/radial"
	"github.com/d2lang/elk-go/internal/rectpacking"
	"github.com/d2lang/elk-go/internal/selfloops"
	"github.com/d2lang/elk-go/internal/spore"
	"github.com/d2lang/elk-go/internal/stress"
)

// RegisterOptions installs the option metadata needed by the native provider
// set. Individual registration functions are idempotent for shared core IDs.
func RegisterOptions(registry *core.OptionRegistry) {
	basic.RegisterOptions(registry)
	layered.RegisterOptions(registry)
	force.RegisterOptions(registry)
	stress.RegisterOptions(registry)
	mrtree.RegisterOptions(registry)
	radial.RegisterOptions(registry)
	spore.RegisterOptions(registry)
	rectpacking.RegisterOptions(registry)
	registerCompatibilityOptions(registry)
	for _, id := range []string{selfloops.RouteOption, core.RouteCrossContainerEdges.ID} {
		if err := registry.Register(core.OptionData{ID: id, Kind: core.BooleanOption, Targets: core.Parents}); err != nil {
			panic(err)
		}
	}
}

// Register installs algorithms in the same relative order as the official
// ELK.js 0.12.0 registry. That order is observable during suffix resolution.
func Register(registry *core.AlgorithmRegistry) error {
	for _, register := range []func(*core.AlgorithmRegistry) error{
		basic.Register,
		layeredlayout.Register,
		stress.Register,
		mrtree.Register,
		radial.Register,
		force.Register,
		spore.Register,
		rectpacking.Register,
	} {
		if err := register(registry); err != nil {
			return err
		}
	}
	return nil
}
