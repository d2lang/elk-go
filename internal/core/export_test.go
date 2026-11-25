// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package core

import "github.com/d2lang/elk-go/internal/graph"

// BySuffixScan exposes the reference implementation of BySuffix to the
// external tests of this package.
func (registry *OptionRegistry) BySuffixScan(suffix string) *OptionData {
	return registry.bySuffixScan(suffix)
}

// bySuffixScan is the reference implementation of BySuffix (a linear scan),
// kept for tests.
func (registry *OptionRegistry) bySuffixScan(suffix string) *OptionData {
	if suffix == "" {
		return nil
	}
	if exact := registry.byID[suffix]; exact != nil {
		return exact
	}
	var match *OptionData
	for _, option := range registry.options {
		if dotSuffix(option.ID, suffix) {
			if match != nil && match != option {
				return nil
			}
			match = option
		}
	}
	if match != nil {
		return match
	}
	for _, option := range registry.options {
		for _, legacy := range option.LegacyIDs {
			if dotSuffix(legacy, suffix) {
				if match != nil && match != option {
					return nil
				}
				match = option
			}
		}
	}
	return match
}

// PureParse and RegisteredKey expose what Register records about an option
// besides its exported fields.
func (option *OptionData) PureParse() bool { return option.pureParse }

func (option *OptionData) RegisteredKey() graph.PropertyKey { return option.key }
