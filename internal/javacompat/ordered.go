// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package javacompat

// OrderedMap is an insertion-ordered map matching LinkedHashMap's default
// (accessOrder=false) behavior. Replacing a value does not move its key;
// removing and reinserting a key appends it at the end.
//
// The zero value is ready to use.
type OrderedMap[K comparable, V any] struct {
	entries []orderedEntry[K, V]
	index   map[K]int
}

type orderedEntry[K comparable, V any] struct {
	key   K
	value V
}

// NewOrderedMap constructs an empty insertion-ordered map.
func NewOrderedMap[K comparable, V any]() *OrderedMap[K, V] {
	return &OrderedMap[K, V]{index: make(map[K]int)}
}

// Len returns the number of entries.
func (m *OrderedMap[K, V]) Len() int { return len(m.entries) }

// IsEmpty reports whether the map has no entries.
func (m *OrderedMap[K, V]) IsEmpty() bool { return len(m.entries) == 0 }

// Put inserts or replaces a value. It returns the old value and whether the
// key was already present.
func (m *OrderedMap[K, V]) Put(key K, value V) (V, bool) {
	if i, ok := m.index[key]; ok {
		old := m.entries[i].value
		m.entries[i].value = value
		return old, true
	}
	if m.index == nil {
		m.index = make(map[K]int)
	}
	m.index[key] = len(m.entries)
	m.entries = append(m.entries, orderedEntry[K, V]{key: key, value: value})
	var zero V
	return zero, false
}

// PutIfAbsent inserts only when key is absent and returns the existing or new
// value plus whether an existing entry was found.
func (m *OrderedMap[K, V]) PutIfAbsent(key K, value V) (V, bool) {
	if old, ok := m.Get(key); ok {
		return old, true
	}
	m.Put(key, value)
	return value, false
}

// Get looks up a value.
func (m *OrderedMap[K, V]) Get(key K) (V, bool) {
	if i, ok := m.index[key]; ok {
		return m.entries[i].value, true
	}
	var zero V
	return zero, false
}

// ContainsKey reports whether key is present.
func (m *OrderedMap[K, V]) ContainsKey(key K) bool {
	_, ok := m.index[key]
	return ok
}

// Remove deletes key, preserving the relative order of all remaining keys.
func (m *OrderedMap[K, V]) Remove(key K) (V, bool) {
	i, ok := m.index[key]
	if !ok {
		var zero V
		return zero, false
	}
	old := m.entries[i].value
	delete(m.index, key)
	copy(m.entries[i:], m.entries[i+1:])
	var zeroEntry orderedEntry[K, V]
	m.entries[len(m.entries)-1] = zeroEntry
	m.entries = m.entries[:len(m.entries)-1]
	for j := i; j < len(m.entries); j++ {
		m.index[m.entries[j].key] = j
	}
	return old, true
}

// Clear removes all entries.
func (m *OrderedMap[K, V]) Clear() {
	clear(m.entries)
	m.entries = m.entries[:0]
	clear(m.index)
}

// Keys returns keys in insertion order.
func (m *OrderedMap[K, V]) Keys() []K {
	keys := make([]K, len(m.entries))
	for i, entry := range m.entries {
		keys[i] = entry.key
	}
	return keys
}

// Values returns values in key insertion order.
func (m *OrderedMap[K, V]) Values() []V {
	values := make([]V, len(m.entries))
	for i, entry := range m.entries {
		values[i] = entry.value
	}
	return values
}

// Range visits entries in insertion order until visit returns false.
// Structural mutation during Range is unsupported, matching Java iterator
// fail-fast semantics without exposing a concurrent-modification exception.
func (m *OrderedMap[K, V]) Range(visit func(K, V) bool) {
	for _, entry := range m.entries {
		if !visit(entry.key, entry.value) {
			return
		}
	}
}

// First returns the oldest entry.
func (m *OrderedMap[K, V]) First() (K, V, bool) {
	if len(m.entries) == 0 {
		var key K
		var value V
		return key, value, false
	}
	entry := m.entries[0]
	return entry.key, entry.value, true
}

// Last returns the newest entry.
func (m *OrderedMap[K, V]) Last() (K, V, bool) {
	if len(m.entries) == 0 {
		var key K
		var value V
		return key, value, false
	}
	entry := m.entries[len(m.entries)-1]
	return entry.key, entry.value, true
}

// OrderedSet is an insertion-ordered set matching LinkedHashSet.
// The zero value is ready to use.
type OrderedSet[T comparable] struct {
	values OrderedMap[T, struct{}]
}

// NewOrderedSet constructs an empty insertion-ordered set.
func NewOrderedSet[T comparable]() *OrderedSet[T] { return &OrderedSet[T]{} }

// Add inserts value and reports whether the set changed.
func (s *OrderedSet[T]) Add(value T) bool {
	if s.values.ContainsKey(value) {
		return false
	}
	s.values.Put(value, struct{}{})
	return true
}

// Contains reports whether value is present.
func (s *OrderedSet[T]) Contains(value T) bool { return s.values.ContainsKey(value) }

// Remove deletes value and reports whether the set changed.
func (s *OrderedSet[T]) Remove(value T) bool {
	_, ok := s.values.Remove(value)
	return ok
}

// Len returns the number of values.
func (s *OrderedSet[T]) Len() int { return s.values.Len() }

// IsEmpty reports whether the set is empty.
func (s *OrderedSet[T]) IsEmpty() bool { return s.values.IsEmpty() }

// Clear removes every value.
func (s *OrderedSet[T]) Clear() { s.values.Clear() }

// Values returns values in insertion order.
func (s *OrderedSet[T]) Values() []T { return s.values.Keys() }

// Range visits values in insertion order until visit returns false.
func (s *OrderedSet[T]) Range(visit func(T) bool) {
	s.values.Range(func(value T, _ struct{}) bool { return visit(value) })
}
