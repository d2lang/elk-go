// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package elk provides a native Go implementation of the Eclipse Layout
// Kernel JSON behavior used by D2, pinned to official ELK.js 0.12.0. It does
// not promise compatibility with expert option combinations outside D2's
// documented profile.
package elk

import "github.com/d2lang/elk-go/graphjson"

// Graph is the root ELK JSON graph type.
type Graph = graphjson.Graph

// Node is an ELK JSON node.
type Node = graphjson.Node

// Port is an ELK JSON port.
type Port = graphjson.Port

// Label is an ELK JSON label.
type Label = graphjson.Label

// Edge is an ELK JSON edge.
type Edge = graphjson.Edge

// EdgeSection is a routed section of an ELK JSON edge.
type EdgeSection = graphjson.EdgeSection

// Point is an ELK JSON bend or junction point.
type Point = graphjson.Point

// ID is a string- or integer-valued ELK graph identifier.
type ID = graphjson.ID

// LayoutOptions maps ELK option identifiers to their JSON values.
type LayoutOptions = graphjson.LayoutOptions

// StringID constructs a string-valued graph identifier.
func StringID(value string) ID { return graphjson.StringID(value) }

// IntID constructs an integer-valued graph identifier.
func IntID(value int64) ID { return graphjson.IntID(value) }

// IDPtr returns a pointer to a copy of id.
func IDPtr(id ID) *ID { return graphjson.IDPtr(id) }

// Float64 returns a pointer suitable for optional numeric graph fields.
func Float64(value float64) *float64 { return graphjson.Float64(value) }

// String returns a pointer suitable for optional text fields.
func String(value string) *string { return graphjson.String(value) }

// Number returns value, or zero when value is nil.
func Number(value *float64) float64 { return graphjson.Number(value) }
