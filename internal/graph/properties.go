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
	"sync"
)

// Property identifies a typed graph property. Identity follows ELK and is
// based solely on ID.
type Property[T any] struct {
	ID                 string
	Default            func() T
	MaterializeDefault bool

	// key is ID's interned PropertyKey, computed once by the constructors so
	// typed lookups never hash or compare strings.
	key PropertyKey

	// constant, when set, is the default computed and boxed once (see
	// CloneablePropertyWithConstantDefault).
	constant *constantDefault[T]
}

type constantDefault[T any] struct {
	value T
	box   any
}

func NewProperty[T any](id string) Property[T] {
	return Property[T]{ID: id, key: InternPropertyKey(id)}
}

func PropertyWithDefault[T any](id string, defaultValue func() T) Property[T] {
	return Property[T]{ID: id, Default: defaultValue, key: InternPropertyKey(id)}
}

func CloneablePropertyWithDefault[T any](id string, defaultValue func() T) Property[T] {
	return Property[T]{ID: id, Default: defaultValue, MaterializeDefault: true, key: InternPropertyKey(id)}
}

// CloneablePropertyWithConstantDefault is CloneablePropertyWithDefault for a
// default function that returns the same value on every call, of a type whose
// boxed values cannot change: a value type without references, or a []string
// (which this module never writes in place, see cloneValue). The default is
// computed and boxed once; Get materialises that box, which is the same as
// storing a fresh box of an equal value, and returns the same value Default
// would.
func CloneablePropertyWithConstantDefault[T any](id string, defaultValue func() T) Property[T] {
	property := CloneablePropertyWithDefault(id, defaultValue)
	value := defaultValue()
	property.constant = &constantDefault[T]{value: value, box: cloneValue(any(value))}
	return property
}

// Key returns the interned identity of the property's ID.
func (property Property[T]) Key() PropertyKey {
	if property.key != 0 {
		return property.key
	}
	return InternPropertyKey(property.ID)
}

// PropertyKey is the process-wide interned identity of a property ID. Two
// IDs have the same key exactly when they are the same string. Keys are only
// ever compared for equality, never ordered, so the order in which IDs are
// interned (which may depend on goroutine scheduling) cannot affect results.
type PropertyKey uint32

var propertyKeys struct {
	sync.RWMutex
	byID  map[string]PropertyKey
	names []string
}

// InternPropertyKey returns id's key, assigning a new one on first use.
// Key 0 is never assigned.
func InternPropertyKey(id string) PropertyKey {
	propertyKeys.RLock()
	key, found := propertyKeys.byID[id]
	propertyKeys.RUnlock()
	if found {
		return key
	}
	propertyKeys.Lock()
	defer propertyKeys.Unlock()
	if key, found := propertyKeys.byID[id]; found {
		return key
	}
	if propertyKeys.byID == nil {
		propertyKeys.byID = make(map[string]PropertyKey)
		propertyKeys.names = []string{""}
	}
	key = PropertyKey(len(propertyKeys.names))
	propertyKeys.names = append(propertyKeys.names, id)
	propertyKeys.byID[id] = key
	return key
}

// LookupPropertyKey returns id's key if id was ever interned.
func LookupPropertyKey(id string) (PropertyKey, bool) {
	propertyKeys.RLock()
	key, found := propertyKeys.byID[id]
	propertyKeys.RUnlock()
	return key, found
}

func propertyKeyName(key PropertyKey) string {
	propertyKeys.RLock()
	name := propertyKeys.names[key]
	propertyKeys.RUnlock()
	return name
}

// PropertyEntry is one insertion-ordered property map entry.
type PropertyEntry struct {
	ID    string
	Value any
}

type propertyEntry struct {
	key   PropertyKey
	value any
}

// PropertyMap preserves Java LinkedHashMap insertion order. Maps are small
// (the corpus never exceeds 28 entries), so lookups scan the entries for the
// interned key; mask has bit key%64 set for every stored key, which answers
// most misses (the common case: most reads fall through to a default)
// without scanning.
//
// slots is a position hint per key%16: the position plus one of the entry
// most recently appended with a key of that class (0: none). A lookup checks
// the hinted entry's key before scanning, so a hint only ever saves the scan;
// it never decides an answer by itself.
type PropertyMap struct {
	entries []propertyEntry
	mask    uint64
	slots   [propertySlots]uint8
}

const propertySlots = 16

func keyBit(key PropertyKey) uint64 { return 1 << (uint32(key) & 63) }

func keySlot(key PropertyKey) uint32 { return uint32(key) & (propertySlots - 1) }

func (properties *PropertyMap) indexOf(key PropertyKey) int {
	if properties.mask&keyBit(key) == 0 {
		return -1
	}
	entries := properties.entries
	if hint := int(properties.slots[keySlot(key)]) - 1; hint >= 0 && hint < len(entries) && entries[hint].key == key {
		return hint
	}
	for index := range entries {
		if entries[index].key == key {
			return index
		}
	}
	return -1
}

// hint records that the entry at index holds key.
func (properties *PropertyMap) hint(key PropertyKey, index int) {
	if index < 255 {
		properties.slots[keySlot(key)] = uint8(index + 1)
	}
}

func (properties *PropertyMap) Len() int { return len(properties.entries) }

// EntryCapacity is the number of entries the map's storage holds.
func (properties *PropertyMap) EntryCapacity() int { return cap(properties.entries) }

// PropertyEntryBytes bounds the bytes one entry of a PropertyMap's storage
// takes (a key and an interface value, on 64-bit platforms).
const PropertyEntryBytes = 24

func (properties *PropertyMap) Empty() bool { return len(properties.entries) == 0 }

func (properties *PropertyMap) Has(id string) bool {
	key, found := LookupPropertyKey(id)
	return found && properties.indexOf(key) >= 0
}

// HasKey reports whether a value is stored under key.
func (properties *PropertyMap) HasKey(key PropertyKey) bool {
	return properties.indexOf(key) >= 0
}

func (properties *PropertyMap) Raw(id string) (any, bool) {
	key, found := LookupPropertyKey(id)
	if !found {
		return nil, false
	}
	return properties.RawKey(key)
}

// RawKey returns the value stored under key.
func (properties *PropertyMap) RawKey(key PropertyKey) (any, bool) {
	if index := properties.indexOf(key); index >= 0 {
		return properties.entries[index].value, true
	}
	return nil, false
}

func (properties *PropertyMap) SetRaw(id string, value any) {
	properties.SetRawKey(InternPropertyKey(id), value)
}

// SetRawKey stores value under key, keeping an existing entry's position.
func (properties *PropertyMap) SetRawKey(key PropertyKey, value any) {
	if index := properties.indexOf(key); index >= 0 {
		properties.entries[index].value = value
		return
	}
	if properties.entries == nil {
		// Initial capacities (6 here, the copied length + 11 in the copies)
		// were tuned on d2's corpus: most maps keep growing after creation.
		properties.entries = make([]propertyEntry, 0, 6)
	}
	properties.hint(key, len(properties.entries))
	properties.entries = append(properties.entries, propertyEntry{key: key, value: value})
	properties.mask |= keyBit(key)
}

func (properties *PropertyMap) Delete(id string) {
	key, found := LookupPropertyKey(id)
	if !found {
		return
	}
	properties.DeleteKey(key)
}

// DeleteKey removes key's entry, keeping the order of the others.
func (properties *PropertyMap) DeleteKey(key PropertyKey) {
	index := properties.indexOf(key)
	if index < 0 {
		return
	}
	copy(properties.entries[index:], properties.entries[index+1:])
	properties.entries[len(properties.entries)-1] = propertyEntry{}
	properties.entries = properties.entries[:len(properties.entries)-1]
	properties.mask = 0
	properties.slots = [propertySlots]uint8{}
	for index, entry := range properties.entries {
		properties.mask |= keyBit(entry.key)
		properties.hint(entry.key, index)
	}
}

func (properties *PropertyMap) Entries() []PropertyEntry {
	if len(properties.entries) == 0 {
		return nil
	}
	result := make([]PropertyEntry, len(properties.entries))
	for index, entry := range properties.entries {
		result[index] = PropertyEntry{ID: propertyKeyName(entry.key), Value: entry.value}
	}
	return result
}

func (properties *PropertyMap) CopyFrom(other *PropertyMap) {
	if len(other.entries) == 0 {
		return
	}
	if len(properties.entries) == 0 {
		// A map never holds a key twice, so copying into an empty map appends
		// every entry in order: exactly what SetRawKey would do, without
		// searching for keys that cannot be present yet.
		if cap(properties.entries) < len(other.entries) {
			properties.entries = make([]propertyEntry, 0, len(other.entries)+11)
		}
		for _, entry := range other.entries {
			properties.hint(entry.key, len(properties.entries))
			properties.entries = append(properties.entries, propertyEntry{key: entry.key, value: cloneValue(entry.value)})
		}
		properties.mask |= other.mask
		return
	}
	for _, entry := range other.entries {
		properties.SetRawKey(entry.key, cloneValue(entry.value))
	}
}

// PropertyConverter converts a copied value; ok=false means "store a clone of
// the original value instead".
type PropertyConverter func(value any) (converted any, ok bool)

// CopyFromConverting is CopyFrom where converters[key], when present, decides
// the stored value of the entry with that key.
func (properties *PropertyMap) CopyFromConverting(other *PropertyMap, converters []PropertyConverter) {
	if len(other.entries) == 0 {
		return
	}
	if len(properties.entries) == 0 {
		// Into an empty map every entry is appended in order (see CopyFrom).
		if cap(properties.entries) < len(other.entries) {
			properties.entries = make([]propertyEntry, 0, len(other.entries)+11)
		}
		for _, entry := range other.entries {
			value := entry.value
			converted := false
			if int(entry.key) < len(converters) {
				if convert := converters[entry.key]; convert != nil {
					value, converted = convert(entry.value)
				}
			}
			if !converted {
				value = cloneValue(entry.value)
			}
			properties.hint(entry.key, len(properties.entries))
			properties.entries = append(properties.entries, propertyEntry{key: entry.key, value: value})
		}
		properties.mask |= other.mask
		return
	}
	for _, entry := range other.entries {
		if int(entry.key) < len(converters) {
			if convert := converters[entry.key]; convert != nil {
				if converted, ok := convert(entry.value); ok {
					properties.SetRawKey(entry.key, converted)
					continue
				}
			}
		}
		properties.SetRawKey(entry.key, cloneValue(entry.value))
	}
}

// Recycled empties properties and returns an empty map that reuses its entry
// storage. The old values are cleared so that nothing they reference is kept
// alive. The storage must not be shared with another live map (Clone and
// CopyFrom never share it).
func (properties *PropertyMap) Recycled() PropertyMap {
	entries := properties.entries
	for index := range entries {
		entries[index] = propertyEntry{}
	}
	*properties = PropertyMap{}
	return PropertyMap{entries: entries[:0]}
}

func (properties *PropertyMap) Clone() PropertyMap {
	var result PropertyMap
	result.CopyFrom(properties)
	return result
}

// Get returns an explicitly stored value, then the property's default, then
// T's zero value. Cloneable Java defaults are materialized on first read.
func Get[T any](properties *PropertyMap, property Property[T]) T {
	// TryGet, with the common case (a precomputed key) written out so that
	// a lookup is one call.
	if key := property.key; key != 0 {
		if properties.mask&keyBit(key) != 0 {
			if index := properties.indexOf(key); index >= 0 {
				if typed, ok := properties.entries[index].value.(T); ok {
					return typed
				}
			}
		}
	} else if value, found := TryGet(properties, property); found {
		return value
	}
	if property.Default == nil {
		var zero T
		return zero
	}
	if !property.MaterializeDefault {
		return property.Default()
	}
	return getDefault(properties, property)
}

// getDefault is Get for a property without a stored value of type T.
func getDefault[T any](properties *PropertyMap, property Property[T]) T {
	if property.Default != nil {
		if constant := property.constant; constant != nil && property.MaterializeDefault {
			properties.SetRawKey(property.Key(), constant.box)
			return constant.value
		}
		value := property.Default()
		if property.MaterializeDefault {
			properties.SetRawKey(property.Key(), cloneValue(value))
		}
		return value
	}
	var zero T
	return zero
}

// TryGet returns the value stored for property if it is a T. A property
// without a precomputed key (rare) is looked up by ID, through the map's
// non-generic Raw: an ID that was never interned is stored in no map.
func TryGet[T any](properties *PropertyMap, property Property[T]) (T, bool) {
	var value any
	if key := property.key; key != 0 {
		index := -1
		if properties.mask&keyBit(key) != 0 {
			index = properties.indexOf(key)
		}
		if index < 0 {
			var zero T
			return zero, false
		}
		value = properties.entries[index].value
	} else {
		var found bool
		if value, found = properties.Raw(property.ID); !found {
			var zero T
			return zero, false
		}
	}
	typed, ok := value.(T)
	return typed, ok
}

func Set[T any](properties *PropertyMap, property Property[T], value T) {
	properties.SetRawKey(property.Key(), value)
}

// SetInt is Set for an int property, storing values from -1 to
// maxBoxedInt-1 from boxes made once (a boxed int cannot change, so a shared
// box holding the value is the same as a new one; the runtime itself shares
// boxes of 0 to 255). The boxes are made in chunks of intBoxChunk values on
// first use, so a process makes only the ones it needs.
func SetInt(properties *PropertyMap, property Property[int], value int) {
	properties.SetRawKey(property.Key(), boxInt(value))
}

const maxBoxedInt = 1 << 12

// intBoxChunk is the number of boxes made together, on the first use of one
// of their values: a process pays only for the ranges it uses.
const intBoxChunk = 256

// boxedMinusOne is -1 boxed (a constant: the compiler makes the box
// statically).
var boxedMinusOne any = -1

var intBoxes [maxBoxedInt / intBoxChunk]struct {
	once  sync.Once
	boxes *[intBoxChunk]any
}

func boxInt(value int) any {
	if value == -1 {
		return boxedMinusOne
	}
	if value < 0 || value >= maxBoxedInt || value < 256 {
		return value
	}
	chunk := &intBoxes[value/intBoxChunk]
	chunk.once.Do(func() {
		boxes := new([intBoxChunk]any)
		base := value / intBoxChunk * intBoxChunk
		for index := range boxes {
			boxes[index] = base + index
		}
		chunk.boxes = boxes
	})
	return chunk.boxes[value%intBoxChunk]
}

// Has and Unset are the map's non-generic HasKey and DeleteKey, or Has and
// Delete by ID for a property without a precomputed key.
func Has[T any](properties *PropertyMap, property Property[T]) bool {
	if key := property.key; key != 0 {
		return properties.HasKey(key)
	}
	return properties.Has(property.ID)
}

func Unset[T any](properties *PropertyMap, property Property[T]) {
	if key := property.key; key != 0 {
		properties.DeleteKey(key)
		return
	}
	properties.Delete(property.ID)
}

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
		// A []string property value is never written in place: no code in
		// this module assigns to an element of a []string other than a slice
		// it has just made, copies into one, sorts one or appends to one it
		// did not make (checked with a type-checked scan of the module).
		// Sharing the value is therefore the same as copying it; keep that
		// invariant when adding code.
		return value
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
