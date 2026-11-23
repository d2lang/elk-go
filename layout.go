// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package elk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/d2lang/elk-go/graphjson"
	"github.com/d2lang/elk-go/internal/builtins"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/selfloops"
)

// Layout computes layout in place. Like elkjs, it preserves the input graph's
// element and slice order and adds computed shape and edge-route fields.
func Layout(input *Graph) error {
	options, algorithms := sharedRegistries()
	// The codec and the model's arenas are reused by later calls (see
	// core.ReleaseJSONCodec); nothing of either is referenced after the call.
	codec := core.AcquireJSONCodec(options)
	model, err := codec.Import(input)
	if err != nil {
		core.ReleaseJSONCodec(codec, nil)
		return fmt.Errorf("import ELK graph: %w", err)
	}
	if err := codec.NormalizeSuppliedCoordinates(model, selfloops.SuppliedExteriorLoops(model, algorithms), algorithms); err != nil {
		core.ReleaseJSONCodec(codec, model)
		return fmt.Errorf("import ELK graph: %w", err)
	}
	if err := layoutModel(model); err != nil {
		core.ReleaseJSONCodec(codec, model)
		return fmt.Errorf("layout ELK graph: %w", err)
	}
	if err := codec.TransferLayout(model); err != nil {
		core.ReleaseJSONCodec(codec, model)
		return fmt.Errorf("export ELK layout: %w", err)
	}
	core.ReleaseJSONCodec(codec, model)
	return nil
}

// layoutModel runs the layout engine on an imported model (for Layout and
// Request.Layout).
func layoutModel(model *graph.Model) error {
	options, algorithms := sharedRegistries()
	engine := core.Engine{Options: options, Algorithms: algorithms, Monitor: core.NullProgressMonitor}
	return engine.Layout(model)
}

// LayoutJSON is the byte-oriented equivalent of Layout.
func LayoutJSON(input []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()
	var value graphjson.Graph
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode ELK graph: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("decode ELK graph: unexpected trailing JSON value")
		}
		return nil, fmt.Errorf("decode ELK graph: %w", err)
	}
	if err := Layout(&value); err != nil {
		return nil, err
	}
	output, err := json.Marshal(&value)
	if err != nil {
		return nil, fmt.Errorf("encode ELK graph: %w", err)
	}
	return output, nil
}

var (
	sharedRegistriesOnce sync.Once
	sharedOptions        *core.OptionRegistry
	sharedAlgorithms     *core.AlgorithmRegistry
)

// sharedRegistries returns the default registries, built on first use. Layout
// only reads them (option and algorithm lookup; every provider instance comes
// from a fresh Create call), so one copy serves every call, concurrent calls
// included, with the same results as building them per call.
func sharedRegistries() (*core.OptionRegistry, *core.AlgorithmRegistry) {
	sharedRegistriesOnce.Do(func() {
		sharedOptions, sharedAlgorithms = defaultRegistries()
	})
	return sharedOptions, sharedAlgorithms
}

// defaultRegistries is the single integration point for native providers.
// Algorithm packages register D2's eleven ELK.js 0.12.0 provider IDs and the
// official option metadata here. The executable compatibility contract is
// D2's documented profile, not every expert option combination.
func defaultRegistries() (*core.OptionRegistry, *core.AlgorithmRegistry) {
	options := core.NewOptionRegistry()
	builtins.RegisterOptions(options)
	algorithms := core.NewAlgorithmRegistry()
	if err := builtins.Register(algorithms); err != nil {
		panic(err)
	}
	return options, algorithms
}
