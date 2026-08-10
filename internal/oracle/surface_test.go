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
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/d2lang/elk-go/internal/builtins"
	"github.com/d2lang/elk-go/internal/core"
)

type oracleAlgorithmMetadata struct {
	ID string `json:"id"`
}

func TestCandidateAlgorithmSurface(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_SURFACE") != "1" {
		t.Skip("set ELK_GO_ORACLE_SURFACE=1 to compare the native provider registry")
	}
	registry := core.NewAlgorithmRegistry()
	if err := builtins.Register(registry); err != nil {
		t.Fatal(err)
	}
	algorithms := registry.Algorithms()
	ids := make([]string, len(algorithms))
	for index := range algorithms {
		ids[index] = algorithms[index].ID
	}
	if !reflect.DeepEqual(ids, OfficialAlgorithmIDs) {
		t.Fatalf("native algorithm IDs = %v, want official elkjs %s surface %v", ids, OracleVersion, OfficialAlgorithmIDs)
	}
}

func TestPinnedOracleAlgorithmSurface(t *testing.T) {
	runner := DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Skipf("elkjs differential oracle unavailable: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	raw, err := runner.KnownLayoutAlgorithms(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var metadata []oracleAlgorithmMetadata
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatalf("decode algorithm metadata: %v", err)
	}
	ids := make([]string, len(metadata))
	for index := range metadata {
		ids[index] = metadata[index].ID
	}
	if !reflect.DeepEqual(ids, OfficialAlgorithmIDs) {
		t.Fatalf("elkjs %s algorithm IDs = %v, want %v", OracleVersion, ids, OfficialAlgorithmIDs)
	}
	for _, id := range ids {
		if id == "org.eclipse.elk.disco" {
			t.Fatal("elkjs 0.12.0 unexpectedly exposes removed DisCo provider")
		}
	}
}
