// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

//go:build tinygo

package reuse

// OversizedToPool is false under TinyGo: its sync.Pool is a plain list that
// never frees what it holds, so released storage above its keep-bound is
// dropped.
const OversizedToPool = false
