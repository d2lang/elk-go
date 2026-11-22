// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package p4nodes

// SetBKFastPathsDisabled switches the dense Brandes-Koepf path and the
// single-node shortcut off (true) or on, and returns a function restoring the
// previous settings.
func SetBKFastPathsDisabled(dense, single bool) (restore func()) {
	savedDense, savedSingle := bkDenseDisabled, bkSingleNodeDisabled
	bkDenseDisabled, bkSingleNodeDisabled = dense, single
	return func() { bkDenseDisabled, bkSingleNodeDisabled = savedDense, savedSingle }
}
