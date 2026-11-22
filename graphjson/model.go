// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package graphjson defines ELK's JSON graph interchange model.
package graphjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/d2lang/elk-go/internal/jsonorder"
)

// ID is an ELK graph identifier. ELK's documented API uses string IDs, while
// its JSON importer also accepts integral JSON numbers. ID preserves which
// representation was supplied.
type ID struct {
	text     string
	number   int64
	isNumber bool
	valid    bool
}

// StringID constructs a string-valued graph identifier.
func StringID(value string) ID { return ID{text: value, valid: true} }

// IntID constructs an integer-valued graph identifier.
func IntID(value int64) ID { return ID{number: value, isNumber: true, valid: true} }

// IsValid reports whether the ID was explicitly initialized.
func (id ID) IsValid() bool { return id.valid }

// IsNumber reports whether the ID uses the JSON number representation.
func (id ID) IsNumber() bool { return id.valid && id.isNumber }

// String returns the identifier's canonical text representation.
func (id ID) String() string {
	if !id.valid {
		return ""
	}
	if id.isNumber {
		return strconv.FormatInt(id.number, 10)
	}
	return id.text
}

// Int64 returns the numeric identifier and whether this ID is numeric.
func (id ID) Int64() (int64, bool) { return id.number, id.valid && id.isNumber }

func (id ID) MarshalJSON() ([]byte, error) {
	if !id.valid {
		return []byte("null"), nil
	}
	if id.isNumber {
		return []byte(strconv.FormatInt(id.number, 10)), nil
	}
	return json.Marshal(id.text)
}

func (id *ID) UnmarshalJSON(data []byte) error {
	if id == nil {
		return fmt.Errorf("graphjson.ID: UnmarshalJSON on nil receiver")
	}
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		*id = ID{}
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*id = StringID(value)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return fmt.Errorf("graph element id must be a string or integer: %s", data)
	}
	value, err := strconv.ParseInt(number.String(), 10, 64)
	if err != nil {
		return fmt.Errorf("graph element id must be a string or integer: %s", data)
	}
	*id = IntID(value)
	return nil
}

// LayoutOptions contains layout option values. ELK's JavaScript facade
// accepts strings as documented as well as JSON booleans and numbers.
type LayoutOptions map[string]any

// Point is a two-dimensional point in parent-relative ELK coordinates.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Node is both ELK's root graph type and its recursive node type. Nil slices
// are absent JSON members; non-nil empty slices are emitted as empty arrays.
type Node struct {
	ID                 ID                         `json:"id,omitempty"`
	Labels             []*Label                   `json:"labels,omitempty"`
	LayoutOptions      LayoutOptions              `json:"layoutOptions,omitempty"`
	IndividualSpacings LayoutOptions              `json:"individualSpacings,omitempty"`
	X                  *float64                   `json:"x,omitempty"`
	Y                  *float64                   `json:"y,omitempty"`
	Width              *float64                   `json:"width,omitempty"`
	Height             *float64                   `json:"height,omitempty"`
	Children           []*Node                    `json:"children,omitempty"`
	Ports              []*Port                    `json:"ports,omitempty"`
	Edges              []*Edge                    `json:"edges,omitempty"`
	AdditionalFields   map[string]json.RawMessage `json:"-"`
	layoutOptionOrder  []string
	individualOrder    []string
}

// Graph is the root-node spelling used by Layout.
type Graph = Node

// Port is a connectable shape owned by a node.
type Port struct {
	ID                ID                         `json:"id,omitempty"`
	Labels            []*Label                   `json:"labels,omitempty"`
	LayoutOptions     LayoutOptions              `json:"layoutOptions,omitempty"`
	X                 *float64                   `json:"x,omitempty"`
	Y                 *float64                   `json:"y,omitempty"`
	Width             *float64                   `json:"width,omitempty"`
	Height            *float64                   `json:"height,omitempty"`
	AdditionalFields  map[string]json.RawMessage `json:"-"`
	layoutOptionOrder []string
}

// Label is a shape attached to a node, port, or edge.
type Label struct {
	ID                *ID                        `json:"id,omitempty"`
	Text              *string                    `json:"text,omitempty"`
	Labels            []*Label                   `json:"labels,omitempty"`
	LayoutOptions     LayoutOptions              `json:"layoutOptions,omitempty"`
	X                 *float64                   `json:"x,omitempty"`
	Y                 *float64                   `json:"y,omitempty"`
	Width             *float64                   `json:"width,omitempty"`
	Height            *float64                   `json:"height,omitempty"`
	AdditionalFields  map[string]json.RawMessage `json:"-"`
	layoutOptionOrder []string
}

// Edge accepts both ELK's extended sources/targets form and its deprecated
// primitive source/target form.
type Edge struct {
	ID                ID                         `json:"id,omitempty"`
	Labels            []*Label                   `json:"labels,omitempty"`
	LayoutOptions     LayoutOptions              `json:"layoutOptions,omitempty"`
	Sources           []ID                       `json:"sources,omitempty"`
	Targets           []ID                       `json:"targets,omitempty"`
	Sections          []*EdgeSection             `json:"sections,omitempty"`
	JunctionPoints    []Point                    `json:"junctionPoints,omitempty"`
	Source            *ID                        `json:"source,omitempty"`
	SourcePort        *ID                        `json:"sourcePort,omitempty"`
	Target            *ID                        `json:"target,omitempty"`
	TargetPort        *ID                        `json:"targetPort,omitempty"`
	SourcePoint       *Point                     `json:"sourcePoint,omitempty"`
	TargetPoint       *Point                     `json:"targetPoint,omitempty"`
	BendPoints        []Point                    `json:"bendPoints,omitempty"`
	Container         *ID                        `json:"container,omitempty"`
	AdditionalFields  map[string]json.RawMessage `json:"-"`
	layoutOptionOrder []string
}

// EdgeSection is one connected piece of an edge route.
type EdgeSection struct {
	ID                ID                         `json:"id,omitempty"`
	LayoutOptions     LayoutOptions              `json:"layoutOptions,omitempty"`
	StartPoint        *Point                     `json:"startPoint,omitempty"`
	EndPoint          *Point                     `json:"endPoint,omitempty"`
	BendPoints        []Point                    `json:"bendPoints,omitempty"`
	IncomingShape     *ID                        `json:"incomingShape,omitempty"`
	OutgoingShape     *ID                        `json:"outgoingShape,omitempty"`
	IncomingSections  []ID                       `json:"incomingSections,omitempty"`
	OutgoingSections  []ID                       `json:"outgoingSections,omitempty"`
	AdditionalFields  map[string]json.RawMessage `json:"-"`
	layoutOptionOrder []string
}

// LayoutOptionKeys returns the layout-option member order captured while
// decoding JSON. Programmatically constructed nodes return nil.
func (node *Node) LayoutOptionKeys() []string { return cloneStrings(node.layoutOptionOrder) }

// IndividualSpacingKeys returns the individual-spacing member order captured
// while decoding JSON. Programmatically constructed nodes return nil.
func (node *Node) IndividualSpacingKeys() []string { return cloneStrings(node.individualOrder) }

// LayoutOptionKeys returns the layout-option member order captured while
// decoding JSON. Programmatically constructed ports return nil.
func (port *Port) LayoutOptionKeys() []string { return cloneStrings(port.layoutOptionOrder) }

// LayoutOptionKeys returns the layout-option member order captured while
// decoding JSON. Programmatically constructed labels return nil.
func (label *Label) LayoutOptionKeys() []string { return cloneStrings(label.layoutOptionOrder) }

// LayoutOptionKeys returns the layout-option member order captured while
// decoding JSON. Programmatically constructed edges return nil.
func (edge *Edge) LayoutOptionKeys() []string { return cloneStrings(edge.layoutOptionOrder) }

// LayoutOptionKeys returns the layout-option member order captured while
// decoding JSON. Programmatically constructed sections return nil.
func (section *EdgeSection) LayoutOptionKeys() []string {
	return cloneStrings(section.layoutOptionOrder)
}

// SetLayoutOptionKeys records keys as the member order of LayoutOptions, which
// is the order in which layout applies the options, as decoding JSON does
// (LayoutOptionKeys reports it). Keys missing from LayoutOptions are skipped
// and a repeated key counts once; options that are not listed are applied
// after the listed ones in sorted key order. A nil order (the default for
// programmatically constructed nodes) applies the options in canonical
// option order instead. keys is retained, not copied, so elements may share
// one slice; it must not be modified afterwards.
func (node *Node) SetLayoutOptionKeys(keys []string) { node.layoutOptionOrder = keys }

// SetIndividualSpacingKeys records the member order of IndividualSpacings, as
// SetLayoutOptionKeys does for LayoutOptions.
func (node *Node) SetIndividualSpacingKeys(keys []string) { node.individualOrder = keys }

// SetLayoutOptionKeys records the member order of LayoutOptions; see
// Node.SetLayoutOptionKeys.
func (port *Port) SetLayoutOptionKeys(keys []string) { port.layoutOptionOrder = keys }

// SetLayoutOptionKeys records the member order of LayoutOptions; see
// Node.SetLayoutOptionKeys.
func (label *Label) SetLayoutOptionKeys(keys []string) { label.layoutOptionOrder = keys }

// SetLayoutOptionKeys records the member order of LayoutOptions; see
// Node.SetLayoutOptionKeys.
func (edge *Edge) SetLayoutOptionKeys(keys []string) { edge.layoutOptionOrder = keys }

// SetLayoutOptionKeys records the member order of LayoutOptions; see
// Node.SetLayoutOptionKeys.
func (section *EdgeSection) SetLayoutOptionKeys(keys []string) {
	section.layoutOptionOrder = keys
}

// The jsonorder accessors return what the exported LayoutOptionKeys and
// IndividualSpacingKeys methods return, without copying: the recorded order,
// or nil when it is nil or empty (cloneStrings returns nil for both, and the
// importer sorts unlisted keys differently for a nil order).
func init() {
	jsonorder.LayoutOptionKeys = func(element any) []string {
		var order []string
		switch typed := element.(type) {
		case *Node:
			order = typed.layoutOptionOrder
		case *Port:
			order = typed.layoutOptionOrder
		case *Label:
			order = typed.layoutOptionOrder
		case *Edge:
			order = typed.layoutOptionOrder
		case *EdgeSection:
			order = typed.layoutOptionOrder
		}
		return sharedOrder(order)
	}
	jsonorder.IndividualSpacingKeys = func(node any) []string {
		if typed, ok := node.(*Node); ok {
			return sharedOrder(typed.individualOrder)
		}
		return nil
	}
}

// sharedOrder is order, shared, with an empty order as nil: the nil-ness of
// cloneStrings(order).
func sharedOrder(order []string) []string {
	if len(order) == 0 {
		return nil
	}
	return order
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string(nil), values...)
}

// Float64 returns a pointer suitable for optional numeric graph fields.
func Float64(value float64) *float64 { return &value }

// String returns a pointer suitable for optional text fields.
func String(value string) *string { return &value }

// IDPtr returns a pointer to a copy of id.
func IDPtr(id ID) *ID { return &id }

// Number returns value, or zero when value is nil.
func Number(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}
