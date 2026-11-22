// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package core

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/d2lang/elk-go/graphjson"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

// A typed import builds the model JSONCodec.Import builds for a graph whose
// elements are given one by one as Go values instead of as a graphjson.Graph:
// the caller adds them in the order in which Import visits the graph (a node
// with its options, identifier and shape; then its ports, each with its
// labels; then its labels; then its children, each completely; and after all
// nodes the edges, each with its labels), and every step runs the same code
// as Import's (bindNode, addPort, addLabel, addEdge, addSource, ...). Options
// come prepared (PreparedOptions): resolved and parsed once by the code that
// imports them, and stored without resolving or parsing again. The layout is
// read back with the computations TransferLayout uses (TypedCoordinates).

// PreparedOptions is what importing an element's layout options stores in
// the element's property map: the (property, value) pairs that
// importNodeOptions (for a node) or importProperties (for any other element)
// sets, in order, and the error importNodeOptions reports, if any.
type PreparedOptions struct {
	entries []preparedEntry
	// copyable is set when every value is shared and there are more pairs
	// than an empty map's first growth holds: then properties is (or, for
	// a lazy set, becomes at its second use) the map setting them leaves
	// behind, which an element's empty map takes with one CopyFrom (one
	// allocation instead of several growths).
	copyable   bool
	lazy       bool
	built      bool
	applied    int
	properties graph.PropertyMap
	nodeErr    error
}

type preparedEntry struct {
	key   graph.PropertyKey
	value any
	// option and raw are set instead of value when the parsed value may be
	// changed in place: every element then parses raw itself, as Import
	// does, and gets a value of its own.
	option *OptionData
	raw    any
}

// sharedParseResult reports whether many elements may store the same parsed
// value: a boxed value of a type without references cannot be changed, and
// []string property values are never written in place (see
// graph.cloneValue). Anything else is parsed per element.
func sharedParseResult(value any) bool {
	switch value.(type) {
	case nil, string, bool, int, float64, geometry.Point, geometry.Spacing, []string:
		return true
	}
	return false
}

// preparedKey is what one key with a raw value prepares (see prepareKey).
type preparedKey struct {
	entry preparedEntry
	valid bool
	err   error
}

// prepareKey is one step of importNodeOptions and importProperties for an
// option (not nil) and its raw value: the check for a value elkjs cannot
// instantiate, then the parse. Both are functions of the option and the raw
// value.
func (codec *Codec) prepareKey(option *OptionData, raw any) preparedKey {
	return codec.prepareKeyOwned(option, raw, false)
}

// prepareKeyOwned is prepareKey; owned reports that raw is the codec's own
// copy (see ownedRaw).
func (codec *Codec) prepareKeyOwned(option *OptionData, raw any, owned bool) preparedKey {
	result := preparedKey{err: unconstructable(option, raw)}
	value, valid := codec.parseOwned(option, raw, owned)
	if !valid {
		return result
	}
	result.valid = true
	result.entry = preparedEntry{key: option.propertyKey(), value: value}
	if !sharedParseResult(value) {
		result.entry.value, result.entry.option, result.entry.raw = nil, option, raw
	}
	return result
}

// keyMemo is the last raw value a key of a known order was prepared with,
// and what it prepared.
type keyMemo struct {
	set bool
	raw any
	preparedKey
}

// maxOwnedText bounds the texts (strings, JSON numbers) the codec's caches
// copy and keep: longer ones are not remembered.
const maxOwnedText = 128

// ownedRaw returns a copy of raw that refers to nothing of the caller's (a
// copy of a string or JSON number; booleans and float64 numbers as they are),
// and false for values the memo does not keep: texts longer than
// maxOwnedText and other types (which sameRaw never matches).
func ownedRaw(raw any) (any, bool) {
	switch value := raw.(type) {
	case string:
		if len(value) <= maxOwnedText {
			return strings.Clone(value), true
		}
	case json.Number:
		if len(value) <= maxOwnedText {
			return json.Number(strings.Clone(string(value))), true
		}
	case bool, float64:
		return raw, true
	}
	return nil, false
}

// sameRaw reports whether a and b are the same decoded value for parsing:
// strings, booleans and JSON numbers by value, float64 numbers by their bits.
// Other values are never considered the same.
func sameRaw(a, b any) bool {
	switch x := a.(type) {
	case string:
		y, ok := b.(string)
		return ok && x == y
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case float64:
		y, ok := b.(float64)
		return ok && math.Float64bits(x) == math.Float64bits(y)
	case json.Number:
		y, ok := b.(json.Number)
		return ok && x == y
	}
	return false
}

// PrepareOptions prepares the layout options of an element whose options
// map holds values[index] under keys[index], with keys as its member order
// (as SetLayoutOptionKeys records it; a repeated key keeps its first
// position and its last value, as decoding JSON gives them). keys and values
// are not retained. The result is valid for every codec of the same option
// registry state.
//
// A key of a remembered order (see knownOrder) whose raw value is the one it
// was last prepared with takes that preparation again: preparing a key is a
// function of its option and raw value.
func (codec *Codec) PrepareOptions(keys []string, values []any) PreparedOptions {
	var prepared PreparedOptions
	codec.prepareInto(&prepared, keys, values, nil)
	return prepared
}

// PrepareRequestOptions is PrepareOptions for the elements of the codec's
// current typed import: the result is stored in the codec's blocks (valid
// until the codec is reset or released), and builds its copy map only when
// a second element applies it, so that a set applied once costs no map.
// Applying it changes it: it is for one import at a time.
func (codec *Codec) PrepareRequestOptions(keys []string, values []any) *PreparedOptions {
	prepared := codec.preparedSets.new()
	codec.prepareInto(prepared, keys, values, &codec.preparedEntries)
	prepared.lazy = true
	return prepared
}

// prepareInto prepares into prepared (zero), taking the entries' storage
// from entries when it is not nil.
func (codec *Codec) prepareInto(prepared *PreparedOptions, keys []string, values []any, entries *chunkLists[preparedEntry]) {
	if len(keys) != len(values) {
		panic("core.PrepareOptions: keys and values differ in length")
	}
	if len(keys) == 0 {
		return
	}
	var results []preparedKey
	if known := codec.knownOrder(keys); known != nil && known.unique {
		if known.memo == nil {
			known.memo = make([]keyMemo, len(keys))
		}
		results = codec.preparedKeys[:0]
		for index, option := range known.options {
			if option == nil {
				continue
			}
			memo := &known.memo[index]
			raw := values[index]
			if memo.set && sameRaw(memo.raw, raw) {
				results = append(results, memo.preparedKey)
				continue
			}
			// The memo keeps a copy of raw (see ownedRaw), and the
			// preparation of the copy, which is the preparation of raw.
			if owned, ok := ownedRaw(raw); ok {
				*memo = keyMemo{set: true, raw: owned, preparedKey: codec.prepareKeyOwned(option, owned, true)}
				results = append(results, memo.preparedKey)
			} else {
				results = append(results, codec.prepareKey(option, raw))
			}
		}
	} else {
		// Without a remembered order: the options map these pairs decode
		// to, applied in the order layoutOptionKeys gives for it.
		options := make(graphjson.LayoutOptions, len(keys))
		for index, key := range keys {
			options[key] = values[index]
		}
		results = codec.preparedKeys[:0]
		for _, key := range codec.layoutOptionKeys(options, keys) {
			if option := codec.Options.BySuffix(key); option != nil {
				results = append(results, codec.prepareKey(option, options[key]))
			}
		}
	}
	codec.preparedKeys = results
	shared, valid := true, 0
	for index := range results {
		if prepared.nodeErr == nil {
			prepared.nodeErr = results[index].err
		}
		if results[index].valid {
			valid++
			if results[index].entry.option != nil {
				shared = false
			}
		}
	}
	if entries != nil {
		prepared.entries = entries.take(valid)[:0]
	} else {
		prepared.entries = make([]preparedEntry, 0, valid)
	}
	for index := range results {
		if results[index].valid {
			prepared.entries = append(prepared.entries, results[index].entry)
		}
	}
	if prepared.copyable = shared && valid > copyThreshold; prepared.copyable && entries == nil {
		prepared.buildCopy()
	}
	clear(results)
}

// buildCopy makes the copy map of a copyable set.
func (prepared *PreparedOptions) buildCopy() {
	for index := range prepared.entries {
		prepared.properties.SetRawKey(prepared.entries[index].key, prepared.entries[index].value)
	}
	prepared.built = true
}

// copyThreshold is the capacity of an empty property map's first growth.
const copyThreshold = 6

// apply stores the prepared options in properties (an element's map, just
// created and empty) as importProperties stores them. Into an empty map,
// CopyFrom appends the prepared map's entries in order (their values are
// shared ones, which it does not copy): what setting them one by one does.
func (codec *Codec) apply(properties *graph.PropertyMap, prepared *PreparedOptions) {
	if prepared == nil {
		return
	}
	if prepared.copyable && properties.Empty() {
		if prepared.lazy && !prepared.built {
			if prepared.applied++; prepared.applied > 1 {
				prepared.buildCopy()
			}
		}
		if prepared.built {
			properties.CopyFrom(&prepared.properties)
			return
		}
	}
	for index := range prepared.entries {
		entry := &prepared.entries[index]
		value := entry.value
		if entry.option != nil {
			parsed, valid := codec.parse(entry.option, entry.raw)
			if !valid {
				continue
			}
			value = parsed
		}
		properties.SetRawKey(entry.key, value)
	}
}

// applyNode stores the prepared options of a node as importNodeOptions does:
// on error nothing need be stored, since the import fails.
func (codec *Codec) applyNode(properties *graph.PropertyMap, prepared *PreparedOptions) error {
	if prepared == nil {
		return nil
	}
	if prepared.nodeErr != nil {
		return prepared.nodeErr
	}
	codec.apply(properties, prepared)
	return nil
}

// BeginTyped starts a typed import of at most the given numbers of elements
// (more only cost growth): the codec forgets its previous import, as Import
// does, and the new model holds the root node, which TypedNode imports next.
func (codec *Codec) BeginTyped(nodes, ports, labels, edges int) *graph.Model {
	codec.reset()
	model := codec.newModel()
	codec.reserveCounts(model, importCounts{nodes: nodes, ports: ports, labels: labels, edges: edges})
	codec.recycleInto(&model.Node(model.Root).Properties, codec.recycledNodes, int(model.Root))
	return model
}

// TypedShape is the position and size of an element, all four present.
type TypedShape struct {
	X, Y, Width, Height float64
}

// importFullShape is importShape with all four values present.
func importFullShape(shape *graph.Shape, values *TypedShape) {
	x, y, width, height := values.X, values.Y, values.Width, values.Height
	importShape(shape, &x, &y, &width, &height)
}

// TypedNode imports node id (the root, or a node just added by TypedChild)
// as importNode imports a node with these options, the string identifier and
// shape (nil: no shape member), without individual spacings.
func (codec *Codec) TypedNode(model *graph.Model, id graph.NodeID, identifier string, shape *TypedShape, options *PreparedOptions) error {
	node := model.Node(id)
	if err := codec.applyNode(&node.Properties, options); err != nil {
		return err
	}
	if err := codec.bindNode(model, id, graphjson.StringID(identifier)); err != nil {
		return err
	}
	if shape != nil {
		importFullShape(&node.Shape, shape)
	}
	return nil
}

// TypedChild adds a child to parent, which TypedNode imports next.
func (codec *Codec) TypedChild(model *graph.Model, parent graph.NodeID) graph.NodeID {
	return codec.addChild(model, parent)
}

// TypedPort imports a port of node as importNode and importPort import a
// port with these values (no labels follow in d2's requests, but a caller may
// add them with TypedLabel).
func (codec *Codec) TypedPort(model *graph.Model, node graph.NodeID, identifier string, shape *TypedShape, options *PreparedOptions) (graph.PortID, error) {
	id := codec.addPort(model, node)
	if err := codec.bindPort(model, id, graphjson.StringID(identifier)); err != nil {
		return id, err
	}
	port := model.Port(id)
	importFullShape(&port.Shape, shape)
	codec.apply(&port.Properties, options)
	return id, nil
}

// TypedLabel imports a label of parent as importLabel imports a label with
// text (present), without an identifier.
func (codec *Codec) TypedLabel(model *graph.Model, parent graph.ElementID, text string, shape *TypedShape, options *PreparedOptions) graph.LabelID {
	// The parent's first label list has room for one label (more grow it).
	switch parent.Kind {
	case graph.NodeElement:
		codec.reserveLabel(&model.Node(parent.Node).Labels)
	case graph.PortElement:
		codec.reserveLabel(&model.Port(parent.Port).Labels)
	case graph.EdgeElement:
		codec.reserveLabel(&model.Edge(parent.Edge).Labels)
	}
	id := codec.addLabel(model, parent, text)
	label := model.Label(id)
	label.TextPresent = true
	importFullShape(&label.Shape, shape)
	codec.apply(&label.Properties, options)
	return id
}

// TypedEdge imports an edge contained in parent as importEdge imports one
// with these values, given in sources/targets form (nil and nil is the
// primitive form, which needs a source) without sections. Its labels follow
// (TypedLabel): importEdge imports them before it updates the edge's
// containment, which neither reads nor changes anything labels do, so the
// order of the two does not matter.
func (codec *Codec) TypedEdge(model *graph.Model, parent graph.NodeID, identifier string, sources, targets []string, options *PreparedOptions) (graph.EdgeID, error) {
	edgeIdentifier := graphjson.StringID(identifier)
	id := codec.addEdge(model, parent, edgeIdentifier)
	edge := model.Edge(id)
	codec.apply(&edge.Properties, options)
	// Room for the sources and targets (taken from the codec's blocks).
	if len(sources) > 0 {
		edge.Sources = codec.shapeLists.take(len(sources))[:0]
	}
	if len(targets) > 0 {
		edge.Targets = codec.shapeLists.take(len(targets))[:0]
	}
	if sources == nil && targets == nil {
		return id, fmt.Errorf("an edge must have a source node (edge id: %q)", identifier)
	}
	for _, source := range sources {
		if err := codec.addSource(model, id, graphjson.StringID(source)); err != nil {
			return id, err
		}
	}
	for _, target := range targets {
		if err := codec.addTarget(model, id, graphjson.StringID(target)); err != nil {
			return id, err
		}
	}
	if err := checkEndpoints(model.Edge(id), edgeIdentifier); err != nil {
		return id, err
	}
	model.UpdateContainment(id)
	return id, nil
}

// reserveLabel gives an element's label list, while it has none, room for
// one label from the codec's blocks. The model's lists are only appended to
// by the model's own Add methods during the import, and nothing reads their
// capacity, so where their room comes from is not observable; each list's
// room is its own (capped), so no list writes into another's.
func (codec *Codec) reserveLabel(labels *[]graph.LabelID) {
	if *labels == nil {
		*labels = codec.labelLists.take(1)[:0]
	}
}

// TypedCoordinates computes, once the engine has laid out a typed import's
// model, the coordinate context TransferLayout computes, for NodeOutput,
// EdgeOutputOffset and EdgeOutputContainer.
func (codec *Codec) TypedCoordinates(model *graph.Model) error {
	// Only node tables: the read-back below reads nothing else.
	_, err := codec.nodeCoordinates(model, false)
	return err
}

// NodeOutput returns the position and size TransferLayout writes for node
// index.
func (codec *Codec) NodeOutput(model *graph.Model, index int) (x, y, width, height float64) {
	offset := nodeOutputOffset(model, &codec.context, index)
	shape := &model.Nodes[index].Shape
	return shape.X + offset.X, shape.Y + offset.Y, shape.Width, shape.Height
}

// EdgeOutputOffset returns what TransferLayout adds to every point of the
// sections of edge.
func (codec *Codec) EdgeOutputOffset(model *graph.Model, edge graph.EdgeID) geometry.Point {
	return codec.edgeOutputOffset(model, &codec.context, edge)
}

// EdgeOutputContainer returns the node TransferLayout names as the container
// of edge, and false when it leaves the edge's container as it is.
func (codec *Codec) EdgeOutputContainer(model *graph.Model, edge graph.EdgeID) (graph.NodeID, bool) {
	originalParent := int(codec.edgeParents[int(edge)])
	if codec.context.nodeEdgeMode[originalParent] != "CONTAINER" {
		return 0, false
	}
	containing := model.Edge(edge).ContainingNode
	if containing == nil {
		return 0, false
	}
	return *containing, true
}

// chunkSlab hands out pointers to zero values from blocks that never move
// and are kept across imports; reset zeroes the values handed out.
type chunkSlab[T any] struct {
	blocks [][]T
	block  int
	used   int
}

func (slab *chunkSlab[T]) new() *T {
	for slab.block < len(slab.blocks) && slab.used == len(slab.blocks[slab.block]) {
		slab.block++
		slab.used = 0
	}
	if slab.block == len(slab.blocks) {
		size := 16
		if n := len(slab.blocks); n > 0 {
			size = 2 * len(slab.blocks[n-1])
		}
		slab.blocks = append(slab.blocks, make([]T, size))
		slab.used = 0
	}
	value := &slab.blocks[slab.block][slab.used]
	slab.used++
	return value
}

func (slab *chunkSlab[T]) reset() {
	for index := 0; index < len(slab.blocks) && index <= slab.block; index++ {
		used := len(slab.blocks[index])
		if index == slab.block {
			used = slab.used
		}
		clear(slab.blocks[index][:used])
	}
	slab.block, slab.used = 0, 0
}

// capacity is the number of values the blocks hold.
func (slab *chunkSlab[T]) capacity() int {
	return blocksCapacity(slab.blocks)
}

func blocksCapacity[T any](blocks [][]T) int {
	total := 0
	for _, block := range blocks {
		total += cap(block)
	}
	return total
}

// chunkLists hands out zeroed slices, each capped at its length, from blocks
// kept across imports; reset zeroes the elements handed out.
type chunkLists[T any] struct {
	blocks [][]T
	block  int
	used   int
}

func (lists *chunkLists[T]) take(n int) []T {
	if n == 0 {
		return nil
	}
	for lists.block < len(lists.blocks) && lists.used+n > len(lists.blocks[lists.block]) {
		lists.block++
		lists.used = 0
	}
	if lists.block == len(lists.blocks) {
		size := 64
		if count := len(lists.blocks); count > 0 {
			size = 2 * len(lists.blocks[count-1])
		}
		lists.blocks = append(lists.blocks, make([]T, max(size, n)))
		lists.used = 0
	}
	taken := lists.blocks[lists.block][lists.used : lists.used+n : lists.used+n]
	lists.used += n
	return taken
}

func (lists *chunkLists[T]) reset() {
	for index := 0; index < len(lists.blocks) && index <= lists.block; index++ {
		used := len(lists.blocks[index])
		if index == lists.block {
			used = lists.used
		}
		clear(lists.blocks[index][:used])
	}
	lists.block, lists.used = 0, 0
}

// capacity is the number of elements the blocks hold.
func (lists *chunkLists[T]) capacity() int {
	return blocksCapacity(lists.blocks)
}
