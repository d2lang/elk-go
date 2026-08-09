// Copyright (c) 2009, 2015 Kiel University and others.
// Copyright (c) 2009, 2017 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package graph

import (
	"fmt"
	"strconv"
)

// Property identifies a typed graph property. Identity follows ELK and is
// based solely on ID.
type Property[T any] struct {
	ID                 string
	Default            func() T
	MaterializeDefault bool
}

func NewProperty[T any](id string) Property[T] { return Property[T]{ID: id} }

func PropertyWithDefault[T any](id string, defaultValue func() T) Property[T] {
	return Property[T]{ID: id, Default: defaultValue}
}

func CloneablePropertyWithDefault[T any](id string, defaultValue func() T) Property[T] {
	return Property[T]{ID: id, Default: defaultValue, MaterializeDefault: true}
}

// PropertyEntry is one insertion-ordered property map entry.
type PropertyEntry struct {
	ID    string
	Value any
}

// PropertyMap preserves Java LinkedHashMap insertion order while retaining
// constant-time lookup.
type PropertyMap struct {
	entries []PropertyEntry
	index   map[string]int
}

func (properties *PropertyMap) ensureIndex() {
	if properties.index == nil {
		properties.index = make(map[string]int)
	}
}

func (properties *PropertyMap) Len() int    { return len(properties.entries) }
func (properties *PropertyMap) Empty() bool { return len(properties.entries) == 0 }

func (properties *PropertyMap) Has(id string) bool {
	if properties.index == nil {
		return false
	}
	_, found := properties.index[id]
	return found
}

func (properties *PropertyMap) Raw(id string) (any, bool) {
	if properties.index == nil {
		return nil, false
	}
	index, found := properties.index[id]
	if !found {
		return nil, false
	}
	return properties.entries[index].Value, true
}

func (properties *PropertyMap) SetRaw(id string, value any) {
	properties.ensureIndex()
	if index, found := properties.index[id]; found {
		properties.entries[index].Value = value
		return
	}
	properties.index[id] = len(properties.entries)
	properties.entries = append(properties.entries, PropertyEntry{ID: id, Value: value})
}

func (properties *PropertyMap) Delete(id string) {
	if properties.index == nil {
		return
	}
	index, found := properties.index[id]
	if !found {
		return
	}
	copy(properties.entries[index:], properties.entries[index+1:])
	properties.entries = properties.entries[:len(properties.entries)-1]
	delete(properties.index, id)
	for next := index; next < len(properties.entries); next++ {
		properties.index[properties.entries[next].ID] = next
	}
}

func (properties *PropertyMap) Entries() []PropertyEntry {
	return append([]PropertyEntry(nil), properties.entries...)
}

func (properties *PropertyMap) CopyFrom(other *PropertyMap) {
	for _, entry := range other.entries {
		properties.SetRaw(entry.ID, cloneValue(entry.Value))
	}
}

func (properties *PropertyMap) Clone() PropertyMap {
	var result PropertyMap
	result.CopyFrom(properties)
	return result
}

// Get returns an explicitly stored value, then the property's default, then
// T's zero value. Cloneable Java defaults are materialized on first read.
func Get[T any](properties *PropertyMap, property Property[T]) T {
	if value, found := TryGet(properties, property); found {
		return value
	}
	if property.Default != nil {
		value := property.Default()
		if property.MaterializeDefault {
			properties.SetRaw(property.ID, cloneValue(value))
		}
		return value
	}
	var zero T
	return zero
}

func TryGet[T any](properties *PropertyMap, property Property[T]) (T, bool) {
	value, found := properties.Raw(property.ID)
	if found {
		if typed, ok := value.(T); ok {
			return typed, true
		}
	}
	var zero T
	return zero, false
}

func Set[T any](properties *PropertyMap, property Property[T], value T) {
	properties.SetRaw(property.ID, value)
}

func Has[T any](properties *PropertyMap, property Property[T]) bool {
	return properties.Has(property.ID)
}
func Unset[T any](properties *PropertyMap, property Property[T]) { properties.Delete(property.ID) }

type cloneable interface{ Clone() any }

func cloneValue(value any) any {
	if value == nil {
		return nil
	}
	if cloner, ok := value.(cloneable); ok {
		return cloner.Clone()
	}
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []int:
		return append([]int(nil), typed...)
	case []float64:
		return append([]float64(nil), typed...)
	default:
		return value
	}
}

// JavaString produces the value spelling consumed and emitted by ELK's
// layout option metadata layer.
func JavaString(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case int:
		return strconv.Itoa(typed)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case int64:
		return strconv.FormatInt(typed, 10)
	case float32:
		return strconv.FormatFloat(float64(typed), 'g', -1, 32)
	case float64:
		return strconv.FormatFloat(typed, 'g', -1, 64)
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(value)
	}
}
