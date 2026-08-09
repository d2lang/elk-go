// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0/.
//
// SPDX-License-Identifier: EPL-2.0

package oracle

import (
	"fmt"
	"os"
	"path/filepath"
)

// CuratedInputs returns the checked-in, hand-audited differential inputs in
// lexical order.
func CuratedInputs() ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(moduleRoot(), "testdata", "curated", "*.json"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no curated ELK oracle inputs found")
	}
	return paths, nil
}

// ReadInput reads a corpus path and adds the filename to any error.
func ReadInput(path string) ([]byte, error) {
	input, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read oracle input %q: %w", path, err)
	}
	return input, nil
}
