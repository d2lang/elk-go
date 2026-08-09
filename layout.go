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

	"github.com/d2lang/elk-go/graphjson"
	"github.com/d2lang/elk-go/internal/builtins"
	"github.com/d2lang/elk-go/internal/core"
)

// Layout computes layout in place. Like elkjs, it preserves the input graph's
// element and slice order and adds computed shape and edge-route fields.
func Layout(input *Graph) error {
	options, algorithms := defaultRegistries()
	codec := core.NewCodec(options)
	model, err := codec.Import(input)
	if err != nil {
		return fmt.Errorf("import ELK graph: %w", err)
	}
	if err := core.NewEngine(options, algorithms).Layout(model); err != nil {
		return fmt.Errorf("layout ELK graph: %w", err)
	}
	if err := codec.TransferLayout(model); err != nil {
		return fmt.Errorf("export ELK layout: %w", err)
	}
	return nil
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

// defaultRegistries is the single integration point for native providers.
// Algorithm packages register exact ELK 0.8.1 metadata here.
func defaultRegistries() (*core.OptionRegistry, *core.AlgorithmRegistry) {
	options := core.NewOptionRegistry()
	builtins.RegisterOptions(options)
	algorithms := core.NewAlgorithmRegistry()
	if err := builtins.Register(algorithms); err != nil {
		panic(err)
	}
	return options, algorithms
}
