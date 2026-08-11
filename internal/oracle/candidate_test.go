// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0/.
//
// SPDX-License-Identifier: EPL-2.0

package oracle

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
	"time"

	elk "github.com/d2lang/elk-go"
)

// TestCandidateCuratedParity compares the native implementation with the
// official ELK.js 0.12.0 oracle. It is opt-in because a normal source checkout
// need not contain Node.js or the reference JavaScript artifact.
func TestCandidateCuratedParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_PARITY") != "1" {
		t.Skip("set ELK_GO_ORACLE_PARITY=1 to run the ELK.js differential")
	}
	runner := DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	paths, err := CuratedInputs()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			input, err := ReadInput(path)
			if err != nil {
				t.Fatal(err)
			}
			input, err = PrepareD2ELK012Input(input)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := Differential(ctx, runner, input, elk.LayoutJSON, ExactTolerance); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestCandidateRandomParity differentially checks deterministic generated
// graphs with D2's exact ELK option profile. Set ELK_GO_ORACLE_RANDOM to the
// number of seeds to run.
func TestCandidateRandomParity(t *testing.T) {
	testCandidateRandomParity(t, "ELK_GO_ORACLE_RANDOM", true)
}

// TestCandidateGenericRandomParity checks the same graph stream with ELK's
// normal Layered defaults. It is a broader provider-development gate than the
// D2 compatibility contract.
func TestCandidateGenericRandomParity(t *testing.T) {
	testCandidateRandomParity(t, "ELK_GO_ORACLE_RANDOM_GENERIC", false)
}

// genericLayeredRandomTolerance is deliberately much narrower than the
// general-purpose DefaultTolerance. In the first 100 deterministic generic
// Layered seeds, 67 are bit-exact and the other 33 differ only at clipped edge
// endpoints, by at most 1.13686837722e-13. D2-profile seeds remain bit-exact.
var genericLayeredRandomTolerance = Tolerance{Absolute: 2e-13}

func testCandidateRandomParity(t *testing.T, environment string, d2Options bool) {
	value := os.Getenv(environment)
	if value == "" {
		t.Skipf("set %s to the number of generated differential cases", environment)
	}
	count, err := strconv.Atoi(value)
	if err != nil || count < 1 {
		t.Fatalf("%s must be a positive integer, got %q", environment, value)
	}
	runner := DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	tolerance := ExactTolerance
	if !d2Options {
		tolerance = genericLayeredRandomTolerance
	}
	for index := 0; index < count; index++ {
		seed := uint64(index)
		input, err := (RandomCase{
			Seed:      seed,
			NodeCount: uint8(seed*17 + 3),
			EdgeCount: uint8(seed*29 + 5),
			D2Options: d2Options,
		}).JSON()
		if err != nil {
			t.Fatal(err)
		}
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := Differential(ctx, runner, input, elk.LayoutJSON, tolerance); err != nil {
				t.Fatalf("%v\ninput: %s", err, input)
			}
		})
	}
}

// TestCandidateD2CorpusParity runs the captured, pre-layout JSON corpus from
// D2's complete E2E suite. The corpus remains outside this module because its
// inputs duplicate D2 fixtures. ELK_GO_D2_CORPUS points either at the capture
// directory (which contains raw/) or directly at its raw directory.
func TestCandidateD2CorpusParity(t *testing.T) {
	corpus := os.Getenv("ELK_GO_D2_CORPUS")
	if corpus == "" {
		t.Skip("set ELK_GO_D2_CORPUS to run the complete D2 ELK differential")
	}
	if info, err := os.Stat(filepath.Join(corpus, "raw")); err == nil && info.IsDir() {
		corpus = filepath.Join(corpus, "raw")
	}
	paths, err := filepath.Glob(filepath.Join(corpus, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		t.Fatalf("no JSON inputs found in %q", corpus)
	}
	runner := DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			input, err := ReadInput(path)
			if err != nil {
				t.Fatal(err)
			}
			input, err = PrepareD2ELK012Input(input)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			if err := Differential(ctx, runner, input, elk.LayoutJSON, ExactTolerance); err != nil {
				t.Fatal(err)
			}
		})
	}
}
