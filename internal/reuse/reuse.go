// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package reuse keeps released working storage for later layouts.
//
// Storage goes back for reuse (into a Pool, or into the buffer a processor
// keeps for its next call) on the normal return paths of the call that used
// it, never from a deferred call: a call that panics drops what it used, and
// under TinyGo (d2's WebAssembly build) extra deferred calls in the functions
// a panic unwinds have made a caller's recover fail or return a different
// value.
package reuse

import "sync"

// Pool holds released values of *T for reuse: one in a slot that garbage
// collection does not empty, so that layouts run one after another always
// find it (a sync.Pool loses its values at collections, and a goroutine that
// moved to another processor does not see the value it put into the
// previous processor's private slot), and more in a sync.Pool for layouts
// running concurrently. Only values of bounded size go to the slot. A value
// above its bound goes to the sync.Pool only, which garbage collection
// empties, under the gc toolchain, and is dropped under TinyGo (d2's
// WebAssembly build), whose sync.Pool is a plain list that never frees what
// it holds (see OversizedToPool). Whatever a user takes from a Pool must be
// indistinguishable from fresh storage for it; the Pool only stores values.
type Pool[T any] struct {
	mutex sync.Mutex
	slot  *T
	// large is set when a value above its bound went to the sync.Pool, and
	// cleared when Get then finds the sync.Pool empty (gc only). While it is
	// set, Get tries the sync.Pool first: a large layout run after a smaller
	// one would otherwise take the slot's small value and grow it again,
	// leaving the large one unused until the collector frees it. Which value
	// Get returns never matters to its user, so the flag is only a hint.
	large bool
	pool  sync.Pool
}

// Get returns a released value, or nil when there is none.
func (pool *Pool[T]) Get() *T {
	pool.mutex.Lock()
	if pool.large {
		pool.mutex.Unlock()
		if value, _ := pool.pool.Get().(*T); value != nil {
			return value
		}
		pool.mutex.Lock()
		pool.large = false
	}
	value := pool.slot
	pool.slot = nil
	pool.mutex.Unlock()
	if value != nil {
		return value
	}
	value, _ = pool.pool.Get().(*T)
	return value
}

// Put releases value for reuse. keep says whether value may be kept in the
// slot (and so stay alive indefinitely); callers pass false for values
// holding more than a bounded amount of storage, which go to the sync.Pool
// only (gc) or are dropped (TinyGo).
func (pool *Pool[T]) Put(value *T, keep bool) {
	if value == nil {
		return
	}
	if !keep {
		if OversizedToPool {
			pool.mutex.Lock()
			pool.large = true
			pool.mutex.Unlock()
			pool.pool.Put(value)
		}
		return
	}
	pool.mutex.Lock()
	if pool.slot == nil {
		pool.slot = value
		value = nil
	}
	pool.mutex.Unlock()
	if value != nil {
		pool.pool.Put(value)
	}
}
