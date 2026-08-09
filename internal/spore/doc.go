// Copyright (c) 2017 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package spore is a native Go port of ELK 0.8.1's SPOrE overlap-removal and
// compaction providers.
//
// The implementation is translated from these EPL-2.0 sources at ELK commit
// 5dc0f65c78778beb74a36e0f470c6fe6ecdcef67:
//
//   - plugins/org.eclipse.elk.alg.spore/src/org/eclipse/elk/alg/spore/**
//   - plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/
//     {BowyerWatsonTriangulation,NaiveMinST,TEdge,TTriangle,Tree}.java
//   - plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/spore/**
//   - the relevant geometry methods in ElkMath, KVector, and Utils
//
// The frozen elkjs 0.8.2 bundle embedded by D2 is the behavioral oracle. No
// code from elkrs or OpenJDK is used by this package.
package spore
