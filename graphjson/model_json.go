// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package graphjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
)

// javascriptNumber preserves JSON.stringify's numeric boundary semantics.
// JavaScript serializes NaN and infinities as null instead of rejecting the
// containing object, which matters when reproducing an upstream ELK failure
// mode byte-for-byte.
type javascriptNumber float64

func (number javascriptNumber) MarshalJSON() ([]byte, error) {
	value := float64(number)
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return []byte("null"), nil
	}
	return json.Marshal(value)
}

func decodeObject(data []byte, target any, known ...string) (
	map[string]json.RawMessage,
	map[string]json.RawMessage,
	error,
) {
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return nil, nil, err
	}
	extra := make(map[string]json.RawMessage, len(all))
	for key, value := range all {
		extra[key] = value
	}
	for _, key := range known {
		delete(extra, key)
	}
	if len(extra) == 0 {
		extra = nil
	}
	return extra, all, nil
}

func orderedObjectKeys(data json.RawMessage) ([]string, error) {
	if len(data) == 0 || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delimiter, ok := token.(json.Delim)
	if !ok || delimiter != '{' {
		return nil, fmt.Errorf("layout options must be an object")
	}
	keys := make([]string, 0)
	seen := make(map[string]struct{})
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("layout option key must be a string")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		if _, duplicate := seen[key]; !duplicate {
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	return keys, nil
}

func encodeObject(extra map[string]json.RawMessage, fields map[string]any) ([]byte, error) {
	object := make(map[string]any, len(extra)+len(fields))
	for key, value := range extra {
		object[key] = value
	}
	for key, value := range fields {
		object[key] = value
	}
	return json.Marshal(object)
}

func addID(fields map[string]any, key string, value ID) {
	if value.IsValid() {
		fields[key] = value
	}
}

func addSlice[T any](fields map[string]any, key string, value []T) {
	if value != nil {
		fields[key] = value
	}
}

func addMap(fields map[string]any, key string, value LayoutOptions) {
	if value != nil {
		fields[key] = value
	}
}

func addPointer[T any](fields map[string]any, key string, value *T) {
	if value != nil {
		fields[key] = value
	}
}

func addNumberPointer(fields map[string]any, key string, value *float64) {
	if value != nil {
		fields[key] = javascriptNumber(*value)
	}
}

func (point Point) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		X javascriptNumber `json:"x"`
		Y javascriptNumber `json:"y"`
	}{X: javascriptNumber(point.X), Y: javascriptNumber(point.Y)})
}

func (node *Node) UnmarshalJSON(data []byte) error {
	type plain Node
	var value plain
	extra, fields, err := decodeObject(data, &value,
		"id", "labels", "layoutOptions", "properties", "individualSpacings",
		"x", "y", "width", "height", "children", "ports", "edges")
	if err != nil {
		return err
	}
	*node = Node(value)
	layoutOptionField := "layoutOptions"
	if node.LayoutOptions == nil {
		var legacy struct {
			Properties LayoutOptions `json:"properties"`
		}
		if err := json.Unmarshal(data, &legacy); err != nil {
			return err
		}
		node.LayoutOptions = legacy.Properties
		layoutOptionField = "properties"
	}
	node.layoutOptionOrder, err = orderedObjectKeys(fields[layoutOptionField])
	if err != nil {
		return err
	}
	node.individualOrder, err = orderedObjectKeys(fields["individualSpacings"])
	if err != nil {
		return err
	}
	node.AdditionalFields = extra
	return nil
}

func (node Node) MarshalJSON() ([]byte, error) {
	fields := make(map[string]any, 11)
	addID(fields, "id", node.ID)
	addSlice(fields, "labels", node.Labels)
	addMap(fields, "layoutOptions", node.LayoutOptions)
	addMap(fields, "individualSpacings", node.IndividualSpacings)
	addNumberPointer(fields, "x", node.X)
	addNumberPointer(fields, "y", node.Y)
	addNumberPointer(fields, "width", node.Width)
	addNumberPointer(fields, "height", node.Height)
	addSlice(fields, "children", node.Children)
	addSlice(fields, "ports", node.Ports)
	addSlice(fields, "edges", node.Edges)
	return encodeObject(node.AdditionalFields, fields)
}

func (port *Port) UnmarshalJSON(data []byte) error {
	type plain Port
	var value plain
	extra, fields, err := decodeObject(data, &value,
		"id", "labels", "layoutOptions", "properties", "x", "y", "width", "height")
	if err != nil {
		return err
	}
	*port = Port(value)
	layoutOptionField := "layoutOptions"
	if port.LayoutOptions == nil {
		var legacy struct {
			Properties LayoutOptions `json:"properties"`
		}
		if err := json.Unmarshal(data, &legacy); err != nil {
			return err
		}
		port.LayoutOptions = legacy.Properties
		layoutOptionField = "properties"
	}
	port.layoutOptionOrder, err = orderedObjectKeys(fields[layoutOptionField])
	if err != nil {
		return err
	}
	port.AdditionalFields = extra
	return nil
}

func (port Port) MarshalJSON() ([]byte, error) {
	fields := make(map[string]any, 8)
	addID(fields, "id", port.ID)
	addSlice(fields, "labels", port.Labels)
	addMap(fields, "layoutOptions", port.LayoutOptions)
	addNumberPointer(fields, "x", port.X)
	addNumberPointer(fields, "y", port.Y)
	addNumberPointer(fields, "width", port.Width)
	addNumberPointer(fields, "height", port.Height)
	return encodeObject(port.AdditionalFields, fields)
}

func (label *Label) UnmarshalJSON(data []byte) error {
	type plain Label
	var value plain
	extra, fields, err := decodeObject(data, &value,
		"id", "text", "labels", "layoutOptions", "properties", "x", "y", "width", "height")
	if err != nil {
		return err
	}
	*label = Label(value)
	layoutOptionField := "layoutOptions"
	if label.LayoutOptions == nil {
		var legacy struct {
			Properties LayoutOptions `json:"properties"`
		}
		if err := json.Unmarshal(data, &legacy); err != nil {
			return err
		}
		label.LayoutOptions = legacy.Properties
		layoutOptionField = "properties"
	}
	label.layoutOptionOrder, err = orderedObjectKeys(fields[layoutOptionField])
	if err != nil {
		return err
	}
	label.AdditionalFields = extra
	return nil
}

func (label Label) MarshalJSON() ([]byte, error) {
	fields := make(map[string]any, 9)
	addPointer(fields, "id", label.ID)
	addPointer(fields, "text", label.Text)
	addSlice(fields, "labels", label.Labels)
	addMap(fields, "layoutOptions", label.LayoutOptions)
	addNumberPointer(fields, "x", label.X)
	addNumberPointer(fields, "y", label.Y)
	addNumberPointer(fields, "width", label.Width)
	addNumberPointer(fields, "height", label.Height)
	return encodeObject(label.AdditionalFields, fields)
}

func (edge *Edge) UnmarshalJSON(data []byte) error {
	type plain Edge
	var value plain
	extra, fields, err := decodeObject(data, &value,
		"id", "labels", "layoutOptions", "properties", "sources", "targets", "sections",
		"junctionPoints", "source", "sourcePort", "target", "targetPort", "sourcePoint",
		"targetPoint", "bendPoints", "container")
	if err != nil {
		return err
	}
	*edge = Edge(value)
	layoutOptionField := "layoutOptions"
	if edge.LayoutOptions == nil {
		var legacy struct {
			Properties LayoutOptions `json:"properties"`
		}
		if err := json.Unmarshal(data, &legacy); err != nil {
			return err
		}
		edge.LayoutOptions = legacy.Properties
		layoutOptionField = "properties"
	}
	edge.layoutOptionOrder, err = orderedObjectKeys(fields[layoutOptionField])
	if err != nil {
		return err
	}
	edge.AdditionalFields = extra
	return nil
}

func (edge Edge) MarshalJSON() ([]byte, error) {
	fields := make(map[string]any, 18)
	addID(fields, "id", edge.ID)
	addSlice(fields, "labels", edge.Labels)
	addMap(fields, "layoutOptions", edge.LayoutOptions)
	addSlice(fields, "sources", edge.Sources)
	addSlice(fields, "targets", edge.Targets)
	addSlice(fields, "sections", edge.Sections)
	addSlice(fields, "junctionPoints", edge.JunctionPoints)
	addPointer(fields, "source", edge.Source)
	addPointer(fields, "sourcePort", edge.SourcePort)
	addPointer(fields, "target", edge.Target)
	addPointer(fields, "targetPort", edge.TargetPort)
	addPointer(fields, "sourcePoint", edge.SourcePoint)
	addPointer(fields, "targetPoint", edge.TargetPoint)
	addSlice(fields, "bendPoints", edge.BendPoints)
	addPointer(fields, "container", edge.Container)
	return encodeObject(edge.AdditionalFields, fields)
}

func (section *EdgeSection) UnmarshalJSON(data []byte) error {
	type plain EdgeSection
	var value plain
	extra, fields, err := decodeObject(data, &value,
		"id", "layoutOptions", "properties", "startPoint", "endPoint", "bendPoints",
		"incomingShape", "outgoingShape", "incomingSections", "outgoingSections")
	if err != nil {
		return err
	}
	*section = EdgeSection(value)
	layoutOptionField := "layoutOptions"
	if section.LayoutOptions == nil {
		var legacy struct {
			Properties LayoutOptions `json:"properties"`
		}
		if err := json.Unmarshal(data, &legacy); err != nil {
			return err
		}
		section.LayoutOptions = legacy.Properties
		layoutOptionField = "properties"
	}
	section.layoutOptionOrder, err = orderedObjectKeys(fields[layoutOptionField])
	if err != nil {
		return err
	}
	section.AdditionalFields = extra
	return nil
}

func (section EdgeSection) MarshalJSON() ([]byte, error) {
	fields := make(map[string]any, 10)
	addID(fields, "id", section.ID)
	addMap(fields, "layoutOptions", section.LayoutOptions)
	addPointer(fields, "startPoint", section.StartPoint)
	addPointer(fields, "endPoint", section.EndPoint)
	addSlice(fields, "bendPoints", section.BendPoints)
	addPointer(fields, "incomingShape", section.IncomingShape)
	addPointer(fields, "outgoingShape", section.OutgoingShape)
	addSlice(fields, "incomingSections", section.IncomingSections)
	addSlice(fields, "outgoingSections", section.OutgoingSections)
	return encodeObject(section.AdditionalFields, fields)
}
