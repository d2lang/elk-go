// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package main

import (
	"fmt"
	"io"
	"os"

	elk "github.com/d2lang/elk-go"
)

func main() {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fatal(err)
	}
	output, err := elk.LayoutJSON(input)
	if err != nil {
		fatal(err)
	}
	if _, err := os.Stdout.Write(append(output, '\n')); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
