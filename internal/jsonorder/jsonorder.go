// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package jsonorder gives elk-go's importer the member orders that graphjson
// records for option maps, without the copy the exported LayoutOptionKeys
// methods make. Package graphjson installs the accessors when it initializes.
package jsonorder

// LayoutOptionKeys returns the recorded member order of the LayoutOptions of
// a *graphjson.Node, Port, Label, Edge or EdgeSection (nil for anything
// else). The result is shared and must not be modified.
var LayoutOptionKeys func(element any) []string

// IndividualSpacingKeys returns the recorded member order of the
// IndividualSpacings of a *graphjson.Node. The result is shared and must not
// be modified.
var IndividualSpacingKeys func(node any) []string
