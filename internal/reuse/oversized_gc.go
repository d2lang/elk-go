// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

//go:build !tinygo

package reuse

// OversizedToPool says whether released storage above its keep-bound may go
// to a sync.Pool. Under the gc toolchain it may: garbage collection empties a
// sync.Pool, so such storage lives at most until the next collections, and
// large layouts run one after another keep their speed.
const OversizedToPool = true
