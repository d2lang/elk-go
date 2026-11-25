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
	"math"
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
	// boxedEnumValues holds EnumValues as interface values, made once by
	// Register so that parsing an enum value does not allocate.
	boxedEnumValues []any
	// key is ID's interned property key, set by Register, so that import
	// stores values without interning the ID again.
	key graph.PropertyKey
	// pureParse marks options whose Parse is a pure function of the text
	// that returns an immutable value (a geometry value type), so that
	// import may reuse the result of an earlier parse of the same text.
	pureParse bool
}

// propertyKey returns the interned key of the option's ID: the one Register
// recorded, else the ID interned now (the same key: interning is a function
// of the string).
func (option *OptionData) propertyKey() graph.PropertyKey {
	if option.key != 0 {
		return option.key
	}
	return graph.InternPropertyKey(option.ID)
}

// parseRaw is ParseValue(optionString(raw)) for a decoded option value. The
// cases below give that result without formatting the value and parsing the
// text again:
//   - a string for a StringOption is returned as is;
//   - a finite float64 for a DoubleOption: optionString formats it with
//     FormatFloat('g', -1), the shortest text that parses back to the same
//     bits (no suffix or space for ParseValue to trim), so the result is the
//     value itself;
//   - an integral float64 below 1e6 in magnitude for an IntegerOption:
//     FormatFloat('g', -1) writes its plain decimal digits ("-0" for -0),
//     which ParseInt reads as int(value).
func (option *OptionData) parseRaw(raw any) (any, bool) {
	if option.Parse == nil {
		switch typed := raw.(type) {
		case string:
			if option.Kind == StringOption {
				if typed == "null" || typed == "" {
					return nil, false
				}
				return raw, true
			}
			return option.ParseValue(typed)
		case float64:
			switch option.Kind {
			case DoubleOption:
				if !math.IsNaN(typed) && !math.IsInf(typed, 0) {
					return raw, true
				}
			case IntegerOption:
				if typed == math.Trunc(typed) && math.Abs(typed) < 1e6 {
					return int(typed), true
				}
			}
		}
	}
	return option.ParseValue(optionString(raw))
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
		if index, ok := enumIndex(value, option.EnumValues); ok {
			// The value parseEnum returns, from the interface made at
			// registration while it still holds that string.
			if index < len(option.boxedEnumValues) {
				if boxed, isString := option.boxedEnumValues[index].(string); isString && boxed == option.EnumValues[index] {
					return option.boxedEnumValues[index], true
				}
			}
			return option.EnumValues[index], true
		}
	case EnumSetOption:
		return parseEnumSet(value, option.EnumValues)
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

// isEnumSetSeparator reports the bytes that separate enum-set members: the
// runes of "[] ,\t\r\n". They are ASCII, so testing bytes splits a string
// exactly as strings.FieldsFunc with those runes does: no byte of a
// multi-byte or invalid UTF-8 sequence that FieldsFunc decodes as one rune
// is below 0x80.
func isEnumSetSeparator(b byte) bool {
	switch b {
	case '[', ']', ' ', ',', '\t', '\r', '\n':
		return true
	}
	return false
}

// parseEnumSet parses the members of an enum set: the fields of value
// between separators, each an enum value. The result has capacity for
// exactly the fields, as before.
func parseEnumSet(value string, values []string) (any, bool) {
	fields := 0
	for index := 0; index < len(value); index++ {
		if !isEnumSetSeparator(value[index]) && (index == 0 || isEnumSetSeparator(value[index-1])) {
			fields++
		}
	}
	result := make([]string, 0, fields)
	for start := 0; start < len(value); {
		if isEnumSetSeparator(value[start]) {
			start++
			continue
		}
		end := start
		for end < len(value) && !isEnumSetSeparator(value[end]) {
			end++
		}
		parsed, ok := parseEnum(value[start:end], values)
		if !ok {
			return nil, false
		}
		result = append(result, parsed)
		start = end
	}
	return result, true
}

func parseEnum(value string, values []string) (string, bool) {
	if index, ok := enumIndex(value, values); ok {
		return values[index], true
	}
	return "", false
}

// enumIndex finds value among values by name, else as an ordinal.
func enumIndex(value string, values []string) (int, bool) {
	for index, candidate := range values {
		if value == candidate {
			return index, true
		}
	}
	ordinal, err := strconv.Atoi(value)
	if err == nil && ordinal >= 0 && ordinal < len(values) {
		return ordinal, true
	}
	return 0, false
}

// OptionRegistry is insertion ordered and performs ELK's unique dot-boundary
// suffix lookup.
type OptionRegistry struct {
	options []*OptionData
	byID    map[string]*OptionData
	// store is the current block of registered copies (see Register).
	store []OptionData
	// segments lists every registered ID, and separately every legacy ID,
	// under its last segment (the text after its last '.'): an ID ends in a
	// suffix at a dot boundary (see dotSuffix) only when both have the same
	// last segment, so BySuffix checks only the IDs listed under the
	// suffix's. idHeads and legacyHeads give the newest entry of each
	// segment, which links to the older ones.
	segments    []suffixEntry
	idHeads     map[string]int32
	legacyHeads map[string]int32
	// version counts registrations: BySuffix answers can only change when
	// it changes.
	version uint64
}

// suffixEntry is an ID listed under its last segment, with the next older
// entry of that segment (-1: none).
type suffixEntry struct {
	id     string
	option *OptionData
	next   int32
}

// registryCapacity is a little more than the number of options and legacy
// IDs the default registries hold: a new registry's tables have room for
// them, so registering them does not regrow the tables.
const registryCapacity = 256

// optionBlock is the number of registered copies one block holds.
const optionBlock = 64

func NewOptionRegistry() *OptionRegistry {
	registry := &OptionRegistry{
		byID:        make(map[string]*OptionData, registryCapacity),
		segments:    make([]suffixEntry, 0, registryCapacity),
		idHeads:     make(map[string]int32, registryCapacity),
		legacyHeads: make(map[string]int32),
	}
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
	option.LegacyIDs = append([]string(nil), option.LegacyIDs...)
	option.EnumValues = append([]string(nil), option.EnumValues...)
	registry.register(option)
	return nil
}

// register is Register for an option that is valid and whose LegacyIDs and
// EnumValues nothing else refers to.
func (registry *OptionRegistry) register(option OptionData) {
	// The registered copy lives in a block of copies that never moves (it
	// is referred to by pointer): one allocation per block, not per option.
	if len(registry.store) == cap(registry.store) {
		registry.store = make([]OptionData, 0, optionBlock)
	}
	registry.store = append(registry.store, option)
	copyOfOption := &registry.store[len(registry.store)-1]
	copyOfOption.boxedEnumValues = nil
	if len(copyOfOption.EnumValues) > 0 {
		copyOfOption.boxedEnumValues = make([]any, len(copyOfOption.EnumValues))
		for index, value := range copyOfOption.EnumValues {
			copyOfOption.boxedEnumValues[index] = value
		}
	}
	copyOfOption.key = graph.InternPropertyKey(copyOfOption.ID)
	registry.version++
	pointer := copyOfOption
	registry.options = append(registry.options, pointer)
	registry.byID[pointer.ID] = pointer
	for _, legacy := range pointer.LegacyIDs {
		registry.byID[legacy] = pointer
	}
	if registry.idHeads == nil {
		registry.idHeads = make(map[string]int32)
		registry.legacyHeads = make(map[string]int32)
	}
	registry.list(registry.idHeads, pointer.ID, pointer)
	for _, legacy := range pointer.LegacyIDs {
		registry.list(registry.legacyHeads, legacy, pointer)
	}
}

// list adds id, an ID or legacy ID of option, to the entries of its last
// segment in heads.
func (registry *OptionRegistry) list(heads map[string]int32, id string, option *OptionData) {
	segment := lastSegment(id)
	next, found := heads[segment]
	if !found {
		next = -1
	}
	heads[segment] = int32(len(registry.segments))
	registry.segments = append(registry.segments, suffixEntry{id: id, option: option, next: next})
}

// lastSegment is the text of id after its last '.', or id without one. When
// dotSuffix(id, suffix) holds, id and suffix have the same last segment:
// suffix is id, or the end of id after a '.', and either way contains id's
// last segment at its end, after a '.' or as all of suffix (a '.' inside it
// would also be in id after id's last '.').
func lastSegment(id string) string {
	return id[strings.LastIndexByte(id, '.')+1:]
}

func (registry *OptionRegistry) ByID(id string) *OptionData { return registry.byID[id] }
func (registry *OptionRegistry) Options() []*OptionData {
	return append([]*OptionData(nil), registry.options...)
}

// BySuffix resolves an option key as ELK does: an exact ID or legacy ID, else
// the only option whose ID ends in suffix at a dot boundary, else the only
// option with such a legacy ID; nil when there is none or more than one.
func (registry *OptionRegistry) BySuffix(suffix string) *OptionData {
	if suffix == "" {
		return nil
	}
	if exact := registry.byID[suffix]; exact != nil {
		return exact
	}
	segment := lastSegment(suffix)
	if match, found := registry.matchSuffix(registry.idHeads, segment, suffix); found {
		return match
	}
	match, _ := registry.matchSuffix(registry.legacyHeads, segment, suffix)
	return match
}

// matchSuffix looks at the IDs listed under segment in heads that end in
// suffix at a dot boundary: found reports whether there is one, and match is
// their option when they all belong to one option, else nil.
func (registry *OptionRegistry) matchSuffix(heads map[string]int32, segment, suffix string) (match *OptionData, found bool) {
	entry, listed := heads[segment]
	if !listed {
		return nil, false
	}
	for ; entry >= 0; entry = registry.segments[entry].next {
		candidate := &registry.segments[entry]
		if !dotSuffix(candidate.id, suffix) {
			continue
		}
		if found && candidate.option != match {
			return nil, true
		}
		match, found = candidate.option, true
	}
	return match, found
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
