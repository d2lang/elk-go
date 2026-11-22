// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package orthogonal

// DrainGenerators empties the generator pool and returns how many generators
// it held (for the external tests in this directory).
func DrainGenerators() int {
	count := 0
	for generators.Get() != nil {
		count++
	}
	return count
}
