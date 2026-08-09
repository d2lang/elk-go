// Copyright (c) 2012, 2015 Kiel University and others.
// Copyright (c) 2015, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package core implements ELK metadata, JSON conversion, and recursive layout.
package core

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/d2lang/elk-go/internal/graph"
)

type Targets uint8

const (
	Parents Targets = 1 << iota
	Nodes
	Edges
	Ports
	Labels
)

type OptionKind uint8

const (
	StringOption OptionKind = iota
	BooleanOption
	IntegerOption
	DoubleOption
	EnumOption
	EnumSetOption
	ObjectOption
	UndefinedOption
)

// OptionData describes one layout option and how its JSON spelling is parsed.
type OptionData struct {
	ID      string
	Group   string
	Kind    OptionKind
	Targets Targets
	// MetadataOnly marks UI or programmatic metadata that the headless JSON
	// layout engine preserves but does not interpret during layout.
	MetadataOnly bool
	LegacyIDs    []string
	EnumValues   []string
	Parse        func(string) (any, error)
	Format       func(any) string
}

func (option *OptionData) ParseValue(value string) (any, bool) {
	if value == "null" || (value == "" && option.Kind != EnumSetOption) {
		return nil, false
	}
	if option.Parse != nil {
		parsed, err := option.Parse(value)
		return parsed, err == nil
	}
	switch option.Kind {
	case StringOption:
		return value, true
	case BooleanOption:
		if strings.EqualFold(value, "true") {
			return true, true
		}
		if strings.EqualFold(value, "false") {
			return false, true
		}
	case IntegerOption:
		parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 32)
		if err == nil {
			return int(parsed), true
		}
	case DoubleOption:
		trimmed := strings.TrimSpace(value)
		trimmed = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(trimmed, "d"), "D"), "f"), "F")
		parsed, err := strconv.ParseFloat(trimmed, 64)
		if err == nil {
			return parsed, true
		}
	case EnumOption:
		if parsed, ok := parseEnum(value, option.EnumValues); ok {
			return parsed, true
		}
	case EnumSetOption:
		parts := strings.FieldsFunc(value, func(r rune) bool { return strings.ContainsRune("[] ,\t\r\n", r) })
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			parsed, ok := parseEnum(part, option.EnumValues)
			if !ok {
				return nil, false
			}
			result = append(result, parsed)
		}
		return result, true
	case ObjectOption:
		// Programmatic metadata objects have no portable native Go type. Keep
		// their JSON spelling so the option is registered and preserved instead
		// of silently disappearing at the codec boundary.
		return value, true
	}
	return nil, false
}

func (option *OptionData) FormatValue(value any) string {
	if option.Format != nil {
		return option.Format(value)
	}
	if values, ok := value.([]string); ok {
		return "[" + strings.Join(values, ", ") + "]"
	}
	return graph.JavaString(value)
}

func parseEnum(value string, values []string) (string, bool) {
	for _, candidate := range values {
		if value == candidate {
			return candidate, true
		}
	}
	ordinal, err := strconv.Atoi(value)
	if err == nil && ordinal >= 0 && ordinal < len(values) {
		return values[ordinal], true
	}
	return "", false
}

// OptionRegistry is insertion ordered and performs ELK's unique dot-boundary
// suffix lookup.
type OptionRegistry struct {
	options []*OptionData
	byID    map[string]*OptionData
}

func NewOptionRegistry() *OptionRegistry {
	registry := &OptionRegistry{byID: make(map[string]*OptionData)}
	RegisterCoreOptions(registry)
	return registry
}

func (registry *OptionRegistry) Register(option OptionData) error {
	if option.ID == "" {
		return fmt.Errorf("layout option id must not be empty")
	}
	if _, exists := registry.byID[option.ID]; exists {
		return fmt.Errorf("layout option %q is already registered", option.ID)
	}
	copyOfOption := option
	copyOfOption.LegacyIDs = append([]string(nil), option.LegacyIDs...)
	copyOfOption.EnumValues = append([]string(nil), option.EnumValues...)
	pointer := &copyOfOption
	registry.options = append(registry.options, pointer)
	registry.byID[pointer.ID] = pointer
	for _, legacy := range pointer.LegacyIDs {
		registry.byID[legacy] = pointer
	}
	return nil
}

func (registry *OptionRegistry) ByID(id string) *OptionData { return registry.byID[id] }
func (registry *OptionRegistry) Options() []*OptionData {
	return append([]*OptionData(nil), registry.options...)
}

func (registry *OptionRegistry) BySuffix(suffix string) *OptionData {
	if suffix == "" {
		return nil
	}
	if exact := registry.byID[suffix]; exact != nil {
		return exact
	}
	var match *OptionData
	for _, option := range registry.options {
		if dotSuffix(option.ID, suffix) {
			if match != nil && match != option {
				return nil
			}
			match = option
		}
	}
	if match != nil {
		return match
	}
	for _, option := range registry.options {
		for _, legacy := range option.LegacyIDs {
			if dotSuffix(legacy, suffix) {
				if match != nil && match != option {
					return nil
				}
				match = option
			}
		}
	}
	return match
}

func dotSuffix(id, suffix string) bool {
	if !strings.HasSuffix(id, suffix) {
		return false
	}
	return len(id) == len(suffix) || id[len(id)-len(suffix)-1] == '.'
}

type GraphFeature uint32

const (
	FeatureSelfLoops GraphFeature = 1 << iota
	FeatureInsideSelfLoops
	FeatureMultiEdges
	FeatureEdgeLabels
	FeaturePorts
	FeatureCompound
	FeatureClusters
	FeatureDisconnected
	FeatureHyperedges
)

func (features GraphFeature) Has(feature GraphFeature) bool { return features&feature == feature }

// LayoutProvider lays out one parent node and its children.
type LayoutProvider interface {
	Layout(model *graph.Model, parent graph.NodeID, monitor ProgressMonitor) error
}

type AlgorithmData struct {
	ID           string
	Features     GraphFeature
	Create       func() LayoutProvider
	KnownOptions []string
}

type AlgorithmRegistry struct {
	algorithms []*AlgorithmData
	byID       map[string]*AlgorithmData
}

func NewAlgorithmRegistry() *AlgorithmRegistry {
	return &AlgorithmRegistry{byID: make(map[string]*AlgorithmData)}
}

func (registry *AlgorithmRegistry) Register(algorithm AlgorithmData) error {
	if algorithm.ID == "" || algorithm.Create == nil {
		return fmt.Errorf("layout algorithm id and provider are required")
	}
	if _, found := registry.byID[algorithm.ID]; found {
		return fmt.Errorf("layout algorithm %q is already registered", algorithm.ID)
	}
	copyOfAlgorithm := algorithm
	copyOfAlgorithm.KnownOptions = append([]string(nil), algorithm.KnownOptions...)
	pointer := &copyOfAlgorithm
	registry.algorithms = append(registry.algorithms, pointer)
	registry.byID[pointer.ID] = pointer
	return nil
}

func (registry *AlgorithmRegistry) ByID(id string) *AlgorithmData { return registry.byID[id] }
func (registry *AlgorithmRegistry) BySuffix(suffix string) *AlgorithmData {
	if suffix == "" {
		return nil
	}
	if exact := registry.byID[suffix]; exact != nil {
		return exact
	}
	var match *AlgorithmData
	for _, algorithm := range registry.algorithms {
		if dotSuffix(algorithm.ID, suffix) {
			if match != nil {
				return nil
			}
			match = algorithm
		}
	}
	return match
}

func (registry *AlgorithmRegistry) Algorithms() []*AlgorithmData {
	return append([]*AlgorithmData(nil), registry.algorithms...)
}

// ProgressMonitor is deliberately small; providers can report work while
// callers that do not need progress use NullProgressMonitor.
type ProgressMonitor interface {
	Begin(task string, totalWork float64)
	Worked(work float64)
	Done()
	Canceled() bool
	Subtask(work float64) ProgressMonitor
}

type nullProgressMonitor struct{}

func (nullProgressMonitor) Begin(string, float64)                   {}
func (nullProgressMonitor) Worked(float64)                          {}
func (nullProgressMonitor) Done()                                   {}
func (nullProgressMonitor) Canceled() bool                          { return false }
func (monitor nullProgressMonitor) Subtask(float64) ProgressMonitor { return monitor }

var NullProgressMonitor ProgressMonitor = nullProgressMonitor{}
