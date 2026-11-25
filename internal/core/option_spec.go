// Copyright (c) 2016, 2020 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package core

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/d2lang/elk-go/internal/geometry"
)

// OptionSpec declares one layout option in a package's table of options: an
// OptionData written without slices or function values. A package keeps its
// options in an array of these and registers them with a loop that passes
// each element by value, as in
//
//	for index := range optionSpecs {
//		registry.RegisterMissing(optionSpecs[index])
//	}
//
// Such a table is constant data: nothing builds it when a registry is made.
// Building the same options as a slice of OptionData values in the function
// that registers them took 42 KB of code in D2's WebAssembly module; as
// tables they add 10 KB of data instead.
type OptionSpec struct {
	ID    string
	Group string
	// LegacyIDs and EnumValues list an OptionData's LegacyIDs and EnumValues,
	// separated by single spaces.
	LegacyIDs  string
	EnumValues string
	Kind       OptionKind
	Targets    Targets
	// MetadataOnly is OptionData.MetadataOnly.
	MetadataOnly bool
	// Parse names the option's OptionData.Parse.
	Parse OptionParser
}

// OptionParser names a function that parses the JSON spelling of an option's
// value (OptionData.Parse).
type OptionParser uint8

const (
	// NoParser leaves Parse nil: the option's Kind decides how it is parsed.
	NoParser OptionParser = iota
	// PointParser, SpacingParser and ChainParser parse with
	// geometry.ParsePoint, geometry.ParseSpacing and geometry.ParseChain.
	PointParser
	SpacingParser
	ChainParser
	// IntegerListParser parses a list of integers like "[1, 2, 3]".
	IntegerListParser
	// IndividualSpacingsParser parses the options of an IndividualSpacings,
	// resolving them in the registry the option is registered in.
	IndividualSpacingsParser
	// purePointParser and pureSpacingParser are PointParser and
	// SpacingParser for an option whose parse results import may reuse (see
	// OptionData.pureParse).
	purePointParser
	pureSpacingParser
)

func parsePointOption(value string) (any, error)   { return geometry.ParsePoint(value) }
func parseSpacingOption(value string) (any, error) { return geometry.ParseSpacing(value) }
func parseChainOption(value string) (any, error)   { return geometry.ParseChain(value) }

// option returns the OptionData spec declares for registry. Its LegacyIDs
// and EnumValues are new slices.
func (spec OptionSpec) option(registry *OptionRegistry) OptionData {
	option := OptionData{
		ID:           spec.ID,
		Group:        spec.Group,
		Kind:         spec.Kind,
		Targets:      spec.Targets,
		MetadataOnly: spec.MetadataOnly,
		LegacyIDs:    splitWords(spec.LegacyIDs),
		EnumValues:   splitWords(spec.EnumValues),
	}
	switch spec.Parse {
	case PointParser:
		option.Parse = parsePointOption
	case purePointParser:
		option.Parse, option.pureParse = parsePointOption, true
	case SpacingParser:
		option.Parse = parseSpacingOption
	case pureSpacingParser:
		option.Parse, option.pureParse = parseSpacingOption, true
	case ChainParser:
		option.Parse = parseChainOption
	case IntegerListParser:
		option.Parse = parseIntegerList
	case IndividualSpacingsParser:
		option.Parse = parseIndividualSpacings(registry)
	}
	return option
}

// splitWords returns the words of text, which single spaces separate; nil
// for an empty text.
func splitWords(text string) []string {
	if text == "" {
		return nil
	}
	words := make([]string, 0, strings.Count(text, " ")+1)
	for {
		end := strings.IndexByte(text, ' ')
		if end < 0 {
			return append(words, text)
		}
		words = append(words, text[:end])
		text = text[end+1:]
	}
}

// RegisterSpec is Register for the option spec declares.
func (registry *OptionRegistry) RegisterSpec(spec OptionSpec) error {
	if spec.ID == "" {
		return fmt.Errorf("layout option id must not be empty")
	}
	if _, exists := registry.byID[spec.ID]; exists {
		return fmt.Errorf("layout option %q is already registered", spec.ID)
	}
	registry.register(spec.option(registry))
	return nil
}

// RegisterMissing registers the option spec declares unless an option with
// its ID is registered already (algorithm packages declare some options that
// the core, or an algorithm registered earlier, declares too). It panics if
// the option cannot be registered.
func (registry *OptionRegistry) RegisterMissing(spec OptionSpec) {
	if registry.ByID(spec.ID) != nil {
		return
	}
	if err := registry.RegisterSpec(spec); err != nil {
		panic(err)
	}
}

func parseIntegerList(value string) (any, error) {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "[")
	trimmed = strings.TrimSuffix(trimmed, "]")
	if strings.TrimSpace(trimmed) == "" {
		return []int{}, nil
	}
	fields := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\r' || r == '\n'
	})
	result := make([]int, 0, len(fields))
	for _, field := range fields {
		parsed, err := strconv.Atoi(field)
		if err != nil {
			return nil, fmt.Errorf("invalid integer list %q: %w", value, err)
		}
		result = append(result, parsed)
	}
	return result, nil
}

// parseIndividualSpacings mirrors IndividualSpacings.parse in ELK 0.8.1.
// The unusual separator is part of the public ELK option string format.
func parseIndividualSpacings(registry *OptionRegistry) func(string) (any, error) {
	return func(value string) (any, error) {
		individual := &IndividualSpacings{}
		if value == "" {
			return individual, nil
		}
		for _, serialized := range strings.Split(value, ";,;") {
			parts := strings.Split(serialized, ":")
			if len(parts) < 2 {
				return nil, fmt.Errorf("invalid individual spacing %q", serialized)
			}
			option := registry.BySuffix(parts[0])
			if option == nil || option.ID == SpacingIndividual.ID {
				return nil, fmt.Errorf("invalid individual spacing option %q", parts[0])
			}
			parsed, ok := option.ParseValue(parts[1])
			if !ok {
				return nil, fmt.Errorf("invalid value %q for individual spacing %q", parts[1], parts[0])
			}
			individual.Properties.SetRaw(option.ID, parsed)
		}
		return individual, nil
	}
}
