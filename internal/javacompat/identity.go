// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package javacompat

// IdentityMap provides IdentityHashMap equality semantics for identity-bearing
// comparable keys. K should be a pointer or an arena handle, never a value
// object whose == operation represents structural equality. Iteration is made
// deterministic in first-insertion order; ELK does not rely on the unspecified
// table order of java.util.IdentityHashMap.
//
// The zero value is ready to use.
type IdentityMap[K comparable, V any] struct {
	values OrderedMap[K, V]
}

func NewIdentityMap[K comparable, V any]() *IdentityMap[K, V] {
	return &IdentityMap[K, V]{}
}

func (m *IdentityMap[K, V]) Len() int                     { return m.values.Len() }
func (m *IdentityMap[K, V]) IsEmpty() bool                { return m.values.IsEmpty() }
func (m *IdentityMap[K, V]) Get(key K) (V, bool)          { return m.values.Get(key) }
func (m *IdentityMap[K, V]) ContainsKey(key K) bool       { return m.values.ContainsKey(key) }
func (m *IdentityMap[K, V]) Put(key K, value V) (V, bool) { return m.values.Put(key, value) }
func (m *IdentityMap[K, V]) Remove(key K) (V, bool)       { return m.values.Remove(key) }
func (m *IdentityMap[K, V]) Clear()                       { m.values.Clear() }
func (m *IdentityMap[K, V]) Keys() []K                    { return m.values.Keys() }
func (m *IdentityMap[K, V]) Values() []V                  { return m.values.Values() }
func (m *IdentityMap[K, V]) Range(visit func(K, V) bool)  { m.values.Range(visit) }

// IdentitySet is IdentityMap's set counterpart. K should be a pointer or an
// arena handle so Go equality is object identity.
type IdentitySet[K comparable] struct {
	values OrderedSet[K]
}

func NewIdentitySet[K comparable]() *IdentitySet[K] { return &IdentitySet[K]{} }

func (s *IdentitySet[K]) Len() int                 { return s.values.Len() }
func (s *IdentitySet[K]) IsEmpty() bool            { return s.values.IsEmpty() }
func (s *IdentitySet[K]) Add(value K) bool         { return s.values.Add(value) }
func (s *IdentitySet[K]) Contains(value K) bool    { return s.values.Contains(value) }
func (s *IdentitySet[K]) Remove(value K) bool      { return s.values.Remove(value) }
func (s *IdentitySet[K]) Clear()                   { s.values.Clear() }
func (s *IdentitySet[K]) Values() []K              { return s.values.Values() }
func (s *IdentitySet[K]) Range(visit func(K) bool) { s.values.Range(visit) }
