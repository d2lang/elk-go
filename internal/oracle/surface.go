// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package oracle

// OfficialAlgorithmIDs is the ordered provider surface returned by
// knownLayoutAlgorithms() from the exact elkjs 0.12.0 bundle. DisCo was the
// twelfth provider in 0.8.2 and is intentionally absent.
var OfficialAlgorithmIDs = []string{
	"org.eclipse.elk.fixed",
	"org.eclipse.elk.box",
	"org.eclipse.elk.random",
	"org.eclipse.elk.layered",
	"org.eclipse.elk.stress",
	"org.eclipse.elk.mrtree",
	"org.eclipse.elk.radial",
	"org.eclipse.elk.force",
	"org.eclipse.elk.sporeOverlap",
	"org.eclipse.elk.sporeCompaction",
	"org.eclipse.elk.rectpacking",
}

// OfficialAlgorithmSuffixes is the corresponding unambiguous public suffix
// surface accepted in elk.algorithm JSON options.
var OfficialAlgorithmSuffixes = []string{
	"fixed",
	"box",
	"random",
	"layered",
	"stress",
	"mrtree",
	"radial",
	"force",
	"sporeOverlap",
	"sporeCompaction",
	"rectpacking",
}
