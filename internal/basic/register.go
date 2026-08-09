// Copyright (c) 2015, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package basic registers ELK's three core utility layout providers.
package basic

import (
	"github.com/d2lang/elk-go/internal/basicoptions"
	"github.com/d2lang/elk-go/internal/box"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/fixed"
	randomlayout "github.com/d2lang/elk-go/internal/random"
)

func RegisterOptions(registry *core.OptionRegistry) {
	basicoptions.RegisterOptions(registry)
}

func Register(registry *core.AlgorithmRegistry) error {
	for _, register := range []func(*core.AlgorithmRegistry) error{
		fixed.Register,
		box.Register,
		randomlayout.Register,
	} {
		if err := register(registry); err != nil {
			return err
		}
	}
	return nil
}
