// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package oracle

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	elk "github.com/d2lang/elk-go"
)

func TestUpgradeManifestCoverage(t *testing.T) {
	manifest, err := LoadUpgradeManifest()
	if err != nil {
		t.Fatal(err)
	}
	var algorithms []string
	var fixtures []string
	for _, stage := range manifest.Stages {
		algorithms = append(algorithms, stage.Algorithms...)
		fixtures = append(fixtures, stage.Fixtures...)
	}
	// Stage order expresses porting dependencies, not the public provider
	// registration order. Compare the two surfaces as sets while retaining
	// duplicate detection through the slice length.
	wantAlgorithms := append([]string(nil), OfficialAlgorithmSuffixes...)
	sort.Strings(algorithms)
	sort.Strings(wantAlgorithms)
	if !reflect.DeepEqual(algorithms, wantAlgorithms) {
		t.Fatalf("staged algorithms = %v, want official surface %v", algorithms, wantAlgorithms)
	}
	paths, err := CuratedInputs()
	if err != nil {
		t.Fatal(err)
	}
	wantFixtures := make([]string, len(paths))
	for index, path := range paths {
		wantFixtures[index] = filepath.ToSlash(filepath.Join("curated", filepath.Base(path)))
	}
	sort.Strings(fixtures)
	sort.Strings(wantFixtures)
	if !reflect.DeepEqual(fixtures, wantFixtures) {
		t.Fatalf("staged fixtures = %v, want all curated inputs %v", fixtures, wantFixtures)
	}
}

// TestCandidateUpgradeStages is the single baseline command for a staged
// native-port inventory. Provider smoke and curated D2-shaped cases require
// exact deterministic output. Every case is a sibling subtest so one mismatch
// does not hide later provider or phase failures.
func TestCandidateUpgradeStages(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_UPGRADE") != "1" {
		t.Skip("set ELK_GO_ORACLE_UPGRADE=1 to run the staged elkjs 0.12 differential")
	}
	manifest, err := LoadUpgradeManifest()
	if err != nil {
		t.Fatal(err)
	}
	runner := DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	for _, stage := range manifest.Stages {
		stage := stage
		t.Run(stage.ID, func(t *testing.T) {
			for _, algorithm := range stage.Algorithms {
				algorithm := algorithm
				t.Run("algorithm/"+algorithm, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					input := providerMatrixInput(algorithm)
					if err := Differential(ctx, runner, input, elk.LayoutJSON, ExactTolerance); err != nil {
						t.Fatal(err)
					}
				})
			}
			for _, fixture := range stage.Fixtures {
				fixture := fixture
				t.Run(fixture, func(t *testing.T) {
					input, err := ReadInput(filepath.Join(moduleRoot(), "testdata", fixture))
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
		})
	}
}
