// Copyright (c) 2017, 2020 Kiel University and others.
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
	"sort"
	"strconv"
	"strings"

	"github.com/d2lang/elk-go/graphjson"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/jsonorder"
	"github.com/d2lang/elk-go/internal/reuse"
)

// Codec imports ELK JSON into the algorithm graph and transfers computed
// layout back into the same public graph objects.
type Codec struct {
	Options     *OptionRegistry
	nodeIDs     idTable[graph.NodeID]
	portIDs     idTable[graph.PortID]
	nodeJSON    []*graphjson.Node
	portJSON    []*graphjson.Port
	edgeJSON    []*graphjson.Edge
	edgeParents []graph.NodeID
	labelJSON   []*graphjson.Label
	sectionJSON []*graphjson.EdgeSection

	keyLists    [keyListsKept]keyListInfo
	keyListNext int
	// scratchUsed is set when keyLists or rawBuffer were written since
	// ReleaseCodec last zeroed them (only JSON imports use them), so that
	// releasing a codec that did not use them leaves them alone.
	scratchUsed bool
	// knownOrders outlive imports (they hold copies of the keys).
	knownOrders    [knownOrdersKept]knownOrder
	knownOrderNext int
	rawBuffer      [rawBufferSize]any
	// preparedKeys is PrepareOptions' buffer; preparedSets and
	// preparedEntries hold PrepareRequestOptions' results.
	preparedKeys    []preparedKey
	preparedSets    chunkSlab[PreparedOptions]
	preparedEntries chunkLists[preparedEntry]
	// shapeLists and labelLists give a typed import's edges and elements
	// room for their sources and targets and their first label.
	shapeLists chunkLists[graph.ShapeID]
	labelLists chunkLists[graph.LabelID]
	// parsed remembers parse results of pureParse options by text; it
	// outlives imports (results depend only on the option and the text).
	parsed map[parsedKey]parsedValue

	// Storage handed back by ReleaseCodec for the next import: the emptied
	// element arenas of the released model (all zero) and the coordinate
	// tables of its transfer.
	spareNodes  []graph.Node
	sparePorts  []graph.Port
	spareLabels []graph.Label
	spareEdges  []graph.Edge
	sparePoints []geometry.Point
	spareModes  []string
	// recycled holds the emptied property storage of the released model's
	// elements, by arena position; an element added at that position takes
	// it (see recycleInto).
	recycledNodes  []graph.PropertyMap
	recycledPorts  []graph.PropertyMap
	recycledLabels []graph.PropertyMap
	recycledEdges  []graph.PropertyMap
	// recycledEntries is the number of entries the recycled storage holds
	// (the sum of its capacities).
	recycledEntries int

	// context holds the coordinate tables of the last transfer (or typed
	// import read back, see TypedCoordinates).
	context coordinateContext
	// spareModel is the released model's struct, for the next import.
	spareModel *graph.Model
}

// idTable binds the IDs of imported elements to their arena positions. Two
// graphjson IDs are equal exactly when they have the same kind and value, so
// textual IDs are kept by text and numeric IDs by number; invalid IDs are
// never bound (importers reject them first).
type idTable[T any] struct {
	text   map[string]T
	number map[int64]T
	// hint sizes the text map when it is made.
	hint int
	// textSize and numberSize bound the bindings each map's storage was made
	// or grown for (maps never shrink): its size hint or the most bindings it
	// held, whichever is larger.
	textSize, numberSize int
}

func (table *idTable[T]) lookup(id graphjson.ID) (T, bool) {
	if !id.IsValid() {
		var zero T
		return zero, false
	}
	if number, isNumber := id.Int64(); isNumber {
		value, found := table.number[number]
		return value, found
	}
	value, found := table.text[id.String()]
	return value, found
}

// bindNew records id (valid) for value and reports whether id was not bound
// yet; when it was, its binding is overwritten, which only an import that
// fails right away does.
func (table *idTable[T]) bindNew(id graphjson.ID, value T) bool {
	if number, isNumber := id.Int64(); isNumber {
		if table.number == nil {
			table.number = make(map[int64]T)
		}
		n := len(table.number)
		table.number[number] = value
		return len(table.number) > n
	}
	if table.text == nil {
		table.text = make(map[string]T, table.hint)
		table.textSize = max(table.textSize, table.hint)
	}
	n := len(table.text)
	table.text[id.String()] = value
	return len(table.text) > n
}

// clear forgets every binding. A map whose storage was made or grown for
// more than maxRetainedIDs bindings is dropped (the next binding makes a new
// one): clearing a map costs time in proportion to its storage, and a map
// kept across imports never gives its storage back.
func (table *idTable[T]) clear() {
	if table.textSize = max(table.textSize, len(table.text)); table.textSize > maxRetainedIDs {
		table.text, table.textSize = nil, 0
	} else {
		clear(table.text)
	}
	if table.numberSize = max(table.numberSize, len(table.number)); table.numberSize > maxRetainedIDs {
		table.number, table.numberSize = nil, 0
	} else {
		clear(table.number)
	}
}

// bytes bounds the storage of the table's maps.
func (table *idTable[T]) bytes() int {
	return textBindingBytes*table.textSize + numberBindingBytes*table.numberSize
}

func NewCodec(options *OptionRegistry) *Codec {
	if options == nil {
		options = NewOptionRegistry()
	}
	return &Codec{Options: options}
}

// releasedCodecs keeps released codecs for reuse: one in a slot that garbage
// collection does not empty, so that sequential layouts always find it
// (sync.Pool loses values at collections and when a goroutine changes
// processors), and more in a sync.Pool for concurrent layouts; a codec above
// maxRetainedBytes goes to the sync.Pool only (see reuse.Pool).
var releasedCodecs reuse.Pool[Codec]

// AcquireCodec is NewCodec, reusing the storage of a codec given to
// ReleaseCodec when there is one.
func AcquireCodec(options *OptionRegistry) *Codec {
	codec := releasedCodecs.Get()
	if codec == nil {
		return NewCodec(options)
	}
	if options == nil {
		options = NewOptionRegistry()
	}
	codec.Options = options
	return codec
}

// Bounds of what released codecs keep for reuse.
const (
	// maxPooledElements bounds the elements of a released model whose arena
	// and property storage a codec takes for reuse.
	maxPooledElements = 1 << 14
	// maxRetainedBytes bounds the storage a released codec in the slot keeps
	// (see retainedBytes): a codec holding more goes to the sync.Pool only
	// (gc), which garbage collection empties, so that one large layout does
	// not keep its storage for the life of the process (the slot is never
	// emptied); under TinyGo, whose sync.Pool never drops anything, it is
	// dropped (see reuse.OversizedToPool).
	maxRetainedBytes = 2 << 20
	// maxRetainedIDs bounds the bindings whose map storage a codec keeps (see
	// idTable.clear).
	maxRetainedIDs = 1 << 12
)

// ReleaseCodec empties codec and offers its storage, and that of the element
// arenas of model (the model codec imported last, or nil), for reuse by a
// later AcquireCodec. Neither codec nor model, nor anything obtained from
// them, may be used afterwards.
//
// Reuse is unobservable: the bindings are emptied, the cached member orders
// are forgotten, and an import appends every element to its arena before it
// is read (only the importer appends to the node, port, label and edge
// arenas, so their capacity cannot be observed either). A released codec
// refers to nothing of the caller's (the caches that outlive imports hold
// copies of short keys and values only) and keeps at most maxRetainedBytes;
// a larger one goes to the sync.Pool only (gc) or is emptied and dropped
// (TinyGo).
func ReleaseCodec(codec *Codec, model *graph.Model) {
	if codec == nil {
		return
	}
	if model != nil {
		if cap(model.Nodes)+cap(model.Ports)+cap(model.Labels)+cap(model.Edges) <= maxPooledElements {
			// Positions beyond this model's elements keep the storage of
			// earlier models (this import took none of it).
			for index := range model.Nodes {
				codec.recycledNodes = codec.recycleAt(codec.recycledNodes, index, &model.Nodes[index].Properties)
			}
			for index := range model.Ports {
				codec.recycledPorts = codec.recycleAt(codec.recycledPorts, index, &model.Ports[index].Properties)
			}
			for index := range model.Labels {
				codec.recycledLabels = codec.recycleAt(codec.recycledLabels, index, &model.Labels[index].Properties)
			}
			for index := range model.Edges {
				codec.recycledEdges = codec.recycleAt(codec.recycledEdges, index, &model.Edges[index].Properties)
			}
			codec.spareNodes = emptied(model.Nodes)
			codec.sparePorts = emptied(model.Ports)
			codec.spareLabels = emptied(model.Labels)
			codec.spareEdges = emptied(model.Edges)
		}
		*model = graph.Model{}
		codec.spareModel = model
	}
	codec.reset()
	clear(codec.spareModes)
	if codec.scratchUsed {
		codec.keyLists = [keyListsKept]keyListInfo{}
		codec.keyListNext = 0
		codec.rawBuffer = [rawBufferSize]any{}
		codec.scratchUsed = false
	}
	codec.Options = nil
	keep := codec.retainedBytes() <= maxRetainedBytes
	if !keep && !reuse.OversizedToPool {
		// Dropped, and emptied in case something (TinyGo's sync.Pool keeps
		// the items it handed out in its list) still refers to it.
		*codec = Codec{}
		return
	}
	releasedCodecs.Put(codec, keep)
}

// Upper bounds of the bytes one element of the codec's storage takes (64-bit
// sizes; TestRetainedByteBounds checks them).
const (
	pointerBytes         = 8
	stringHeaderBytes    = 16
	pointBytes           = 16
	nodeBytes            = 264
	portBytes            = 192
	labelBytes           = 184
	edgeBytes            = 176
	propertyMapBytes     = 48
	preparedOptionsBytes = 104
	preparedEntryBytes   = 48
	preparedKeyBytes     = 72
	shapeIDBytes         = 12
	labelIDBytes         = 4
	nodeIDBytes          = 4
	// A map binding (key and value) with its share of the map's tables.
	textBindingBytes   = 64
	numberBindingBytes = 32
)

// retainedBytes bounds the storage a released codec keeps: everything whose
// size follows the imports' sizes. The caches that outlive imports
// (knownOrders, parsed, keyLists, rawBuffer) are bounded by construction.
func (codec *Codec) retainedBytes() int {
	bytes := codec.nodeIDs.bytes() + codec.portIDs.bytes()
	bytes += pointerBytes * (cap(codec.nodeJSON) + cap(codec.portJSON) + cap(codec.edgeJSON) + cap(codec.labelJSON) + cap(codec.sectionJSON))
	bytes += nodeIDBytes * cap(codec.edgeParents)
	bytes += preparedKeyBytes * cap(codec.preparedKeys)
	bytes += preparedOptionsBytes*codec.preparedSets.capacity() + preparedEntryBytes*codec.preparedEntries.capacity()
	bytes += shapeIDBytes*codec.shapeLists.capacity() + labelIDBytes*codec.labelLists.capacity()
	bytes += nodeBytes*cap(codec.spareNodes) + portBytes*cap(codec.sparePorts) + labelBytes*cap(codec.spareLabels) + edgeBytes*cap(codec.spareEdges)
	bytes += pointBytes*cap(codec.sparePoints) + stringHeaderBytes*cap(codec.spareModes)
	bytes += propertyMapBytes * (cap(codec.recycledNodes) + cap(codec.recycledPorts) + cap(codec.recycledLabels) + cap(codec.recycledEdges))
	bytes += graph.PropertyEntryBytes * codec.recycledEntries
	return bytes
}

// recycleAt keeps the emptied storage of properties for arena position index.
func (codec *Codec) recycleAt(recycled []graph.PropertyMap, index int, properties *graph.PropertyMap) []graph.PropertyMap {
	storage := properties.Recycled()
	codec.recycledEntries += storage.EntryCapacity()
	if index < len(recycled) {
		codec.recycledEntries -= recycled[index].EntryCapacity()
		recycled[index] = storage
		return recycled
	}
	return append(recycled, storage)
}

// newModel returns the released model's struct, empty (reserveCounts then
// adds the root, node 0, as graph.NewModel has it), or a new model. Nothing
// refers to a released model any more.
func (codec *Codec) newModel() *graph.Model {
	if model := codec.spareModel; model != nil {
		codec.spareModel = nil
		return model
	}
	return graph.NewModel()
}

// emptied zeroes the used part of an arena and returns it with length 0;
// the part beyond its length is zero already (arenas only grow by append).
func emptied[T any](arena []T) []T {
	clear(arena)
	return arena[:0]
}

// reset forgets the previous import, keeping the storage of its bindings
// (emptied) for the next one.
func (codec *Codec) reset() {
	codec.nodeIDs.clear()
	codec.portIDs.clear()
	codec.preparedSets.reset()
	codec.preparedEntries.reset()
	codec.shapeLists.reset()
	codec.labelLists.reset()
	codec.nodeJSON = emptied(codec.nodeJSON)
	codec.portJSON = emptied(codec.portJSON)
	codec.edgeJSON = emptied(codec.edgeJSON)
	codec.edgeParents = emptied(codec.edgeParents)
	codec.labelJSON = emptied(codec.labelJSON)
	codec.sectionJSON = emptied(codec.sectionJSON)
}

// recycleInto gives properties (an element's empty property map, just
// added at position index of its arena) the emptied storage kept for that
// position, if any. An empty map with storage behaves exactly as an empty
// map without: the storage is private to the map, and only its capacity
// differs.
func (codec *Codec) recycleInto(properties *graph.PropertyMap, recycled []graph.PropertyMap, index int) {
	if index < len(recycled) {
		*properties = recycled[index]
		codec.recycledEntries -= properties.EntryCapacity()
		recycled[index] = graph.PropertyMap{}
	}
}

// arena returns an empty slice with capacity for n elements: spare (all
// zero) when it is large enough, else a new one.
func arena[T any](spare *[]T, n int) []T {
	if cap(*spare) >= n {
		reused := (*spare)[:0]
		*spare = nil
		return reused
	}
	return make([]T, 0, n)
}

// binding returns slice emptied with capacity for n elements.
func binding[T any](slice []T, n int) []T {
	if cap(slice) >= n {
		return slice[:0]
	}
	return make([]T, 0, n)
}

// importCounts bounds the number of elements an import creates.
type importCounts struct {
	nodes, ports, labels, edges int
}

func countLabels(labels []*graphjson.Label) int {
	count := 0
	for _, label := range labels {
		if label != nil {
			count++
		}
	}
	return count
}

func (counts *importCounts) add(node *graphjson.Node) {
	if node == nil {
		return
	}
	counts.nodes++
	counts.labels += countLabels(node.Labels)
	for _, port := range node.Ports {
		if port != nil {
			counts.ports++
			counts.labels += countLabels(port.Labels)
		}
	}
	for _, edge := range node.Edges {
		if edge != nil {
			counts.edges++
			counts.labels += countLabels(edge.Labels)
		}
	}
	for _, child := range node.Children {
		counts.add(child)
	}
}

// reserve sizes the model's element arenas and the codec's bindings for an
// import. Only the importer appends to these arenas, and it holds no element
// pointer across an append, so their capacity is unobservable.
func (codec *Codec) reserve(model *graph.Model, input *graphjson.Graph) {
	var counts importCounts
	counts.add(input)
	codec.reserveCounts(model, counts)
}

// reserveCounts is reserve for an import of at most the given numbers of
// elements (more only cost growth).
func (codec *Codec) reserveCounts(model *graph.Model, counts importCounts) {
	codec.nodeIDs.hint = counts.nodes
	codec.nodeJSON = binding(codec.nodeJSON, counts.nodes)
	if model.Nodes == nil {
		// A reused model struct (see newModel): its root, node 0, is a zero
		// node, as graph.NewModel's.
		model.Nodes = append(arena(&codec.spareNodes, max(counts.nodes, 1)), graph.Node{})
	} else {
		model.Nodes = append(arena(&codec.spareNodes, max(counts.nodes, len(model.Nodes))), model.Nodes...)
	}
	if counts.ports > 0 {
		codec.portIDs.hint = counts.ports
		codec.portJSON = binding(codec.portJSON, counts.ports)
		model.Ports = arena(&codec.sparePorts, counts.ports)
	}
	if counts.labels > 0 {
		codec.labelJSON = binding(codec.labelJSON, counts.labels)
		model.Labels = arena(&codec.spareLabels, counts.labels)
	}
	if counts.edges > 0 {
		codec.edgeJSON = binding(codec.edgeJSON, counts.edges)
		codec.edgeParents = binding(codec.edgeParents, counts.edges)
		model.Edges = arena(&codec.spareEdges, counts.edges)
	}
}

// Import validates graph references and constructs the native arena.
func (codec *Codec) Import(input *graphjson.Graph) (*graph.Model, error) {
	if input == nil {
		return nil, fmt.Errorf("ELK graph must not be nil")
	}
	codec.reset()
	model := codec.newModel()
	codec.reserve(model, input)
	codec.recycleInto(&model.Node(model.Root).Properties, codec.recycledNodes, int(model.Root))
	// importNode checks every node for unconstructable properties before it
	// imports the node. Those errors take precedence over every import error,
	// as ELK checks the whole tree first: after any import error, a full scan
	// reports the first such property in tree order, if there is one.
	if err := codec.importNode(model, model.Root, input); err != nil {
		if rejected := codec.rejectUnconstructableJSONProperties(input); rejected != nil {
			return nil, rejected
		}
		return nil, err
	}
	if err := codec.importEdges(model, model.Root, input); err != nil {
		return nil, err
	}
	return model, nil
}

func (codec *Codec) rejectUnconstructableJSONProperties(node *graphjson.Node) error {
	if node == nil {
		return nil
	}
	if err := codec.rejectUnconstructable(node.LayoutOptions, codec.optionKeys(node.LayoutOptions, jsonorder.LayoutOptionKeys(node))); err != nil {
		return err
	}
	for _, child := range node.Children {
		if err := codec.rejectUnconstructableJSONProperties(child); err != nil {
			return err
		}
	}
	return nil
}

// rejectUnconstructable reports the options of one node that elkjs cannot
// instantiate from JSON, taking keys in import order.
func (codec *Codec) rejectUnconstructable(options graphjson.LayoutOptions, keys []string) error {
	for _, key := range keys {
		option := codec.Options.BySuffix(key)
		if option == nil {
			continue
		}
		if err := unconstructable(option, options[key]); err != nil {
			return err
		}
	}
	return nil
}

// unconstructable is the error for a non-null value of an option elkjs cannot
// instantiate from JSON, if option is one.
func unconstructable(option *OptionData, raw any) error {
	if raw == nil {
		return nil
	}
	switch option.ID {
	case TopdownSizeApproximator.ID:
		return fmt.Errorf("Couldn't create new instance of property '%s'. Make sure its type is registered with the ElkReflect utility class.",
			TopdownSizeApproximator.ID)
	case "org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cmEnforcedGroupOrders":
		return fmt.Errorf("Severe implementation error in the Json to ElkGraph importer.")
	}
	return nil
}

// importNodeOptions imports a node's options as importOptions does, checking
// each one for rejectUnconstructable on the way: the same keys in the same
// order, so the first error is the same, and an error discards the model.
func (codec *Codec) importNodeOptions(properties *graph.PropertyMap, options graphjson.LayoutOptions, decodedOrder []string) error {
	keys, resolved, raws := codec.orderedOptions(options, decodedOrder)
	for index, key := range keys {
		option := codec.resolve(resolved, index, key)
		if option == nil {
			continue
		}
		raw := codec.raw(raws, index, options, key)
		if err := unconstructable(option, raw); err != nil {
			return err
		}
		value, valid := codec.parse(option, raw)
		if valid {
			properties.SetRawKey(option.propertyKey(), value)
		}
	}
	return nil
}

type parsedKey struct {
	option *OptionData
	text   string
}

type parsedValue struct {
	value any
	valid bool
}

// maxParsed bounds the parse results a codec remembers.
const maxParsed = 1024

// parse is option.parseRaw(raw), reusing the result of an earlier parse of
// the same text for options whose parse is pure and whose values are
// immutable (see OptionData.pureParse): the same value, shared. Enum sets
// parsed by parseEnumSet qualify too: the result is a function of the text
// and the option's enum values (copied at registration, never changed), and
// a []string property value is never written in place (see
// graph.cloneValue), so sharing one is the same as making a new one.
//
// Only texts of at most maxOwnedText bytes are remembered, by a copy (the
// parse of the copy is the parse of the text), so that the results kept
// across imports are bounded and refer to nothing of the caller's.
func (codec *Codec) parse(option *OptionData, raw any) (any, bool) {
	return codec.parseOwned(option, raw, false)
}

// parseOwned is parse; owned reports that raw is the codec's own copy
// already (see ownedRaw), which is remembered as it is.
func (codec *Codec) parseOwned(option *OptionData, raw any, owned bool) (any, bool) {
	text, isText := raw.(string)
	if !isText || len(text) > maxOwnedText || !(option.pureParse || option.Kind == EnumSetOption && option.Parse == nil) {
		return option.parseRaw(raw)
	}
	key := parsedKey{option: option, text: text}
	if result, found := codec.parsed[key]; found {
		return result.value, result.valid
	}
	if !owned {
		key.text = strings.Clone(text)
	}
	value, valid := option.parseRaw(key.text)
	if codec.parsed == nil || len(codec.parsed) >= maxParsed {
		codec.parsed = make(map[parsedKey]parsedValue)
	}
	codec.parsed[key] = parsedValue{value: value, valid: valid}
	return value, valid
}

// keyListInfo is what an import has learned about one member-order slice.
// Slices are identified by their first element and length; all of them stay
// referenced by the input during the import, so the identity is stable.
type keyListInfo struct {
	first  *string
	length int
	unique bool
	// options resolves every key, made when the slice is seen a second time
	// (slices shared by many elements; decoded JSON has one per element).
	options []*OptionData
}

const keyListsKept = 8

// keyList returns what is known about keys (not empty), recording it.
func (codec *Codec) keyList(keys []string) *keyListInfo {
	codec.scratchUsed = true
	first := &keys[0]
	for index := range codec.keyLists {
		info := &codec.keyLists[index]
		if info.first == first && info.length == len(keys) {
			if info.options == nil {
				info.options = make([]*OptionData, len(keys))
				for position, key := range keys {
					info.options[position] = codec.Options.BySuffix(key)
				}
			}
			return info
		}
	}
	info := &codec.keyLists[codec.keyListNext]
	codec.keyListNext = (codec.keyListNext + 1) % keyListsKept
	if known := codec.knownOrder(keys); known != nil {
		*info = keyListInfo{first: first, length: len(keys), unique: known.unique, options: known.options}
		return info
	}
	*info = keyListInfo{first: first, length: len(keys), unique: noRepeatedKeys(keys)}
	return info
}

// knownOrder is a member order seen by an earlier import of this codec: its
// keys (copied), whether they are unique, and BySuffix of each key for the
// registry state it was resolved in. Orders with equal keys have the same
// answers while the registry is unchanged (see OptionRegistry.version), so
// a later import takes them instead of checking and resolving again.
type knownOrder struct {
	registry *OptionRegistry
	version  uint64
	keys     []string
	unique   bool
	options  []*OptionData
	// memo is, per key, its last preparation by PrepareOptions.
	memo []keyMemo
}

const (
	knownOrdersKept = 8
	maxKnownOrder   = 64
	// maxKnownOrderBytes bounds the text of the keys of a remembered order.
	maxKnownOrderBytes = 1 << 11
)

// knownOrder returns the remembered order with keys' content, recording keys
// (resolving them now) when there is none; nil when keys are too many or too
// long to remember (callers then check and resolve them themselves).
func (codec *Codec) knownOrder(keys []string) *knownOrder {
	if len(keys) > maxKnownOrder {
		return nil
	}
	for index := range codec.knownOrders {
		known := &codec.knownOrders[index]
		if known.registry == codec.Options && known.version == codec.Options.version &&
			len(known.keys) == len(keys) && known.keys[0] == keys[0] && sameStrings(known.keys, keys) {
			return known
		}
	}
	textBytes := 0
	for _, key := range keys {
		textBytes += len(key)
	}
	if textBytes > maxKnownOrderBytes {
		return nil
	}
	known := &codec.knownOrders[codec.knownOrderNext]
	codec.knownOrderNext = (codec.knownOrderNext + 1) % knownOrdersKept
	options := make([]*OptionData, len(keys))
	for position, key := range keys {
		options[position] = codec.Options.BySuffix(key)
	}
	// The keys are copied, text included: the order outlives the import and
	// must not keep the caller's strings alive.
	owned := make([]string, len(keys))
	for position, key := range keys {
		owned[position] = strings.Clone(key)
	}
	*known = knownOrder{
		registry: codec.Options,
		version:  codec.Options.version,
		keys:     owned,
		unique:   noRepeatedKeys(keys),
		options:  options,
	}
	return known
}

func sameStrings(left, right []string) bool {
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func noRepeatedKeys(keys []string) bool {
	for index, key := range keys {
		for _, earlier := range keys[:index] {
			if earlier == key {
				return false
			}
		}
	}
	return true
}

// orderedOptions returns the order in which options are applied (optionKeys)
// and, when that is the decoded order itself, the keys resolved so far and
// the values (in a buffer reused by the next call).
func (codec *Codec) orderedOptions(options graphjson.LayoutOptions, decodedOrder []string) ([]string, []*OptionData, []any) {
	if len(options) == 0 {
		return nil, nil, nil
	}
	if count := len(decodedOrder); count == len(options) && count <= rawBufferSize {
		info := codec.keyList(decodedOrder)
		if info.unique {
			codec.scratchUsed = true
			raws := codec.rawBuffer[:0]
			for _, key := range decodedOrder {
				raw, found := options[key]
				if !found {
					break
				}
				raws = append(raws, raw)
			}
			if len(raws) == count {
				// Every key of options listed once: optionKeys returns the order.
				return decodedOrder, info.options, raws
			}
		}
	}
	return codec.layoutOptionKeys(options, decodedOrder), nil, nil
}

func (codec *Codec) resolve(resolved []*OptionData, index int, key string) *OptionData {
	if resolved != nil {
		return resolved[index]
	}
	return codec.Options.BySuffix(key)
}

func (codec *Codec) raw(raws []any, index int, options graphjson.LayoutOptions, key string) any {
	if raws != nil {
		return raws[index]
	}
	return options[key]
}

func (codec *Codec) importNode(model *graph.Model, id graph.NodeID, input *graphjson.Node) error {
	if input == nil {
		return nil
	}
	// Options first: their check for unconstructable values precedes every
	// other check of the node, and setting properties before the identifier
	// and shape changes nothing (independent fields).
	node := model.Node(id)
	if err := codec.importNodeOptions(&node.Properties, input.LayoutOptions, jsonorder.LayoutOptionKeys(input)); err != nil {
		return err
	}
	if err := codec.bindNode(model, id, input.ID, input); err != nil {
		return err
	}
	importShape(&node.Shape, input.X, input.Y, input.Width, input.Height)
	codec.importIndividualSpacings(&node.Properties, input.IndividualSpacings, jsonorder.IndividualSpacingKeys(input))
	for _, portInput := range input.Ports {
		if portInput == nil {
			continue
		}
		portID := codec.addPort(model, id)
		if err := codec.importPort(model, portID, portInput); err != nil {
			return err
		}
	}
	for _, labelInput := range input.Labels {
		if labelInput == nil {
			continue
		}
		codec.importLabel(model, graph.NodeElementRef(id), labelInput)
	}
	for _, childInput := range input.Children {
		if childInput == nil {
			continue
		}
		childID := codec.addChild(model, id)
		if err := codec.importNode(model, childID, childInput); err != nil {
			return err
		}
	}
	return nil
}

// The element steps below are shared by Import and the typed import (see
// typed.go), which builds the same model from the same elements given as Go
// values; input is the JSON element bound to the new element, nil for a
// typed import.

// addChild adds an empty child node to parent.
func (codec *Codec) addChild(model *graph.Model, parent graph.NodeID) graph.NodeID {
	childID := model.AddNode(&parent)
	codec.recycleInto(&model.Node(childID).Properties, codec.recycledNodes, int(childID))
	return childID
}

// bindNode checks and records the identifier of node id.
func (codec *Codec) bindNode(model *graph.Model, id graph.NodeID, identifier graphjson.ID, input *graphjson.Node) error {
	if !identifier.IsValid() {
		return fmt.Errorf("every node must have an id")
	}
	if !codec.nodeIDs.bindNew(identifier, id) {
		// The import fails: nothing reads the overwritten binding.
		return fmt.Errorf("duplicate node id: %s", identifier.String())
	}
	for len(codec.nodeJSON) <= int(id) {
		codec.nodeJSON = append(codec.nodeJSON, nil)
	}
	codec.nodeJSON[int(id)] = input
	node := model.Node(id)
	node.Identifier, node.IdentifierPresent = identifier.String(), true
	return nil
}

// addPort adds an empty port to node.
func (codec *Codec) addPort(model *graph.Model, node graph.NodeID) graph.PortID {
	portID := model.AddPort(node)
	codec.recycleInto(&model.Port(portID).Properties, codec.recycledPorts, int(portID))
	return portID
}

// bindPort checks and records the identifier of port id.
func (codec *Codec) bindPort(model *graph.Model, id graph.PortID, identifier graphjson.ID, input *graphjson.Port) error {
	if !identifier.IsValid() {
		return fmt.Errorf("every port must have an id")
	}
	if !codec.portIDs.bindNew(identifier, id) {
		// The import fails: nothing reads the overwritten binding.
		return fmt.Errorf("duplicate port id: %s", identifier.String())
	}
	for len(codec.portJSON) <= int(id) {
		codec.portJSON = append(codec.portJSON, nil)
	}
	codec.portJSON[int(id)] = input
	port := model.Port(id)
	port.Identifier, port.IdentifierPresent = identifier.String(), true
	return nil
}

// addLabel adds a label with text to parent.
func (codec *Codec) addLabel(model *graph.Model, parent graph.ElementID, text string, input *graphjson.Label) graph.LabelID {
	id := model.AddLabel(parent, text)
	codec.recycleInto(&model.Label(id).Properties, codec.recycledLabels, int(id))
	for len(codec.labelJSON) <= int(id) {
		codec.labelJSON = append(codec.labelJSON, nil)
	}
	codec.labelJSON[int(id)] = input
	return id
}

// addEdge adds an edge contained in parent with a valid identifier.
func (codec *Codec) addEdge(model *graph.Model, parent graph.NodeID, identifier graphjson.ID, input *graphjson.Edge) graph.EdgeID {
	id := model.AddEdge(&parent)
	codec.recycleInto(&model.Edge(id).Properties, codec.recycledEdges, int(id))
	for len(codec.edgeJSON) <= int(id) {
		codec.edgeJSON = append(codec.edgeJSON, nil)
		codec.edgeParents = append(codec.edgeParents, 0)
	}
	codec.edgeJSON[int(id)] = input
	codec.edgeParents[int(id)] = parent
	edge := model.Edge(id)
	edge.Identifier, edge.IdentifierPresent = identifier.String(), true
	return id
}

// addSource and addTarget add the shape with the given ID to the edge's
// sources or targets.
func (codec *Codec) addSource(model *graph.Model, edge graph.EdgeID, identifier graphjson.ID) error {
	shape, err := codec.shapeByID(identifier)
	if err != nil {
		return err
	}
	model.AddSource(edge, shape)
	return nil
}

func (codec *Codec) addTarget(model *graph.Model, edge graph.EdgeID, identifier graphjson.ID) error {
	shape, err := codec.shapeByID(identifier)
	if err != nil {
		return err
	}
	model.AddTarget(edge, shape)
	return nil
}

// checkEndpoints is the check of an edge given in sources/targets form once
// they are added.
func checkEndpoints(edge *graph.Edge, identifier graphjson.ID) error {
	if len(edge.Sources) == 0 || len(edge.Targets) == 0 {
		return fmt.Errorf("an edge must have at least one source and one target (edge id: %q)", identifier.String())
	}
	return nil
}

func (codec *Codec) importPort(model *graph.Model, id graph.PortID, input *graphjson.Port) error {
	if err := codec.bindPort(model, id, input.ID, input); err != nil {
		return err
	}
	port := model.Port(id)
	importShape(&port.Shape, input.X, input.Y, input.Width, input.Height)
	codec.importProperties(&port.Properties, input.LayoutOptions, jsonorder.LayoutOptionKeys(input))
	for _, labelInput := range input.Labels {
		if labelInput != nil {
			codec.importLabel(model, graph.PortElementRef(id), labelInput)
		}
	}
	return nil
}

func (codec *Codec) importLabel(model *graph.Model, parent graph.ElementID, input *graphjson.Label) graph.LabelID {
	text := ""
	textPresent := input.Text != nil
	if input.Text != nil {
		text = *input.Text
	}
	id := codec.addLabel(model, parent, text, input)
	label := model.Label(id)
	label.TextPresent = textPresent
	if input.ID != nil && input.ID.IsValid() {
		label.Identifier, label.IdentifierPresent = input.ID.String(), true
	}
	importShape(&label.Shape, input.X, input.Y, input.Width, input.Height)
	codec.importProperties(&label.Properties, input.LayoutOptions, jsonorder.LayoutOptionKeys(input))
	return id
}

func (codec *Codec) importEdges(model *graph.Model, parent graph.NodeID, input *graphjson.Node) error {
	for _, edgeInput := range input.Edges {
		if edgeInput == nil {
			continue
		}
		if _, err := codec.importEdge(model, parent, edgeInput); err != nil {
			return err
		}
	}
	children := model.Node(parent).Children
	childIndex := 0
	for _, childInput := range input.Children {
		if childInput == nil {
			continue
		}
		if childIndex >= len(children) {
			return fmt.Errorf("internal child order mismatch")
		}
		if err := codec.importEdges(model, children[childIndex], childInput); err != nil {
			return err
		}
		childIndex++
	}
	return nil
}

func (codec *Codec) importEdge(model *graph.Model, parent graph.NodeID, input *graphjson.Edge) (graph.EdgeID, error) {
	if !input.ID.IsValid() {
		return 0, fmt.Errorf("every edge must have an id")
	}
	id := codec.addEdge(model, parent, input.ID, input)
	edge := model.Edge(id)
	codec.importProperties(&edge.Properties, input.LayoutOptions, jsonorder.LayoutOptionKeys(input))
	if input.Sources != nil || input.Targets != nil {
		for _, source := range input.Sources {
			if err := codec.addSource(model, id, source); err != nil {
				return 0, err
			}
		}
		for _, target := range input.Targets {
			if err := codec.addTarget(model, id, target); err != nil {
				return 0, err
			}
		}
		if err := checkEndpoints(edge, input.ID); err != nil {
			return 0, err
		}
		if err := codec.importSections(model, id, input); err != nil {
			return 0, err
		}
	} else {
		if err := codec.importPrimitiveEdge(model, id, input); err != nil {
			return 0, err
		}
	}
	for _, labelInput := range input.Labels {
		if labelInput != nil {
			codec.importLabel(model, graph.EdgeElementRef(id), labelInput)
		}
	}
	model.UpdateContainment(id)
	return id, nil
}

func (codec *Codec) importPrimitiveEdge(model *graph.Model, id graph.EdgeID, input *graphjson.Edge) error {
	if input.Source == nil {
		return fmt.Errorf("an edge must have a source node (edge id: %q)", input.ID.String())
	}
	sourceNode, found := codec.nodeIDs.lookup(*input.Source)
	if !found {
		return fmt.Errorf("an edge must have a source node (edge id: %q)", input.ID.String())
	}
	source := graph.NodeRef(sourceNode)
	if input.SourcePort != nil {
		port, found := codec.portIDs.lookup(*input.SourcePort)
		if !found || model.Port(port).Parent != sourceNode {
			return fmt.Errorf("source port must belong to the source node (edge id: %q)", input.ID.String())
		}
		source = graph.PortRef(port)
	}
	if input.Target == nil {
		return fmt.Errorf("an edge must have a target node (edge id: %q)", input.ID.String())
	}
	targetNode, found := codec.nodeIDs.lookup(*input.Target)
	if !found {
		return fmt.Errorf("an edge must have a target node (edge id: %q)", input.ID.String())
	}
	target := graph.NodeRef(targetNode)
	if input.TargetPort != nil {
		port, found := codec.portIDs.lookup(*input.TargetPort)
		if !found || model.Port(port).Parent != targetNode {
			return fmt.Errorf("target port must belong to the target node (edge id: %q)", input.ID.String())
		}
		target = graph.PortRef(port)
	}
	model.AddSource(id, source)
	model.AddTarget(id, target)
	if input.SourcePoint != nil || input.TargetPoint != nil || input.BendPoints != nil {
		sectionID := model.AddSection(id)
		section := model.Section(sectionID)
		if input.SourcePoint != nil {
			section.Start = fromJSONPoint(*input.SourcePoint)
			section.StartPresent = true
		}
		if input.TargetPoint != nil {
			section.End = fromJSONPoint(*input.TargetPoint)
			section.EndPresent = true
		}
		if input.BendPoints != nil {
			section.BendPointsPresent = true
			for _, point := range input.BendPoints {
				section.BendPoints = append(section.BendPoints, fromJSONPoint(point))
			}
		}
	}
	return nil
}

func (codec *Codec) importSections(model *graph.Model, edgeID graph.EdgeID, input *graphjson.Edge) error {
	sectionIDs := make(map[graphjson.ID]graph.SectionID)
	type pendingReference struct {
		section    graph.SectionID
		identifier graphjson.ID
	}
	var incoming, outgoing []pendingReference
	for _, sectionInput := range input.Sections {
		if sectionInput == nil {
			continue
		}
		if !sectionInput.ID.IsValid() {
			return fmt.Errorf("every edge section must have an id")
		}
		if _, duplicate := sectionIDs[sectionInput.ID]; duplicate {
			return fmt.Errorf("duplicate edge section id: %s", sectionInput.ID.String())
		}
		if sectionInput.StartPoint == nil {
			return fmt.Errorf("all edge sections need a start point")
		}
		if sectionInput.EndPoint == nil {
			return fmt.Errorf("all edge sections need an end point")
		}
		id := model.AddSection(edgeID)
		sectionIDs[sectionInput.ID] = id
		for len(codec.sectionJSON) <= int(id) {
			codec.sectionJSON = append(codec.sectionJSON, nil)
		}
		codec.sectionJSON[int(id)] = sectionInput
		section := model.Section(id)
		section.Identifier, section.IdentifierPresent = sectionInput.ID.String(), true
		section.Start, section.End = fromJSONPoint(*sectionInput.StartPoint), fromJSONPoint(*sectionInput.EndPoint)
		section.StartPresent, section.EndPresent = true, true
		codec.importProperties(&section.Properties, sectionInput.LayoutOptions, jsonorder.LayoutOptionKeys(sectionInput))
		if sectionInput.BendPoints != nil {
			section.BendPointsPresent = true
		}
		for _, point := range sectionInput.BendPoints {
			section.BendPoints = append(section.BendPoints, fromJSONPoint(point))
		}
		if sectionInput.IncomingShape != nil {
			shape, err := codec.shapeByID(*sectionInput.IncomingShape)
			if err != nil {
				return err
			}
			section.IncomingShape = &shape
		}
		if sectionInput.OutgoingShape != nil {
			shape, err := codec.shapeByID(*sectionInput.OutgoingShape)
			if err != nil {
				return err
			}
			section.OutgoingShape = &shape
		}
		for _, reference := range sectionInput.IncomingSections {
			incoming = append(incoming, pendingReference{id, reference})
		}
		for _, reference := range sectionInput.OutgoingSections {
			outgoing = append(outgoing, pendingReference{id, reference})
		}
	}
	resolve := func(pending []pendingReference, isIncoming bool) error {
		for _, reference := range pending {
			resolved, found := sectionIDs[reference.identifier]
			if !found {
				return fmt.Errorf("referenced edge section does not exist: %s (edge id: %q)", reference.identifier.String(), input.ID.String())
			}
			if isIncoming {
				model.Section(reference.section).IncomingSections = append(model.Section(reference.section).IncomingSections, resolved)
			} else {
				model.Section(reference.section).OutgoingSections = append(model.Section(reference.section).OutgoingSections, resolved)
			}
		}
		return nil
	}
	if err := resolve(incoming, true); err != nil {
		return err
	}
	if err := resolve(outgoing, false); err != nil {
		return err
	}
	edge := model.Edge(edgeID)
	if len(edge.Sources) == 1 && len(edge.Targets) == 1 && len(edge.Sections) == 1 {
		section := model.Section(edge.Sections[0])
		if section.IncomingShape == nil && section.OutgoingShape == nil {
			source, target := edge.Sources[0], edge.Targets[0]
			section.IncomingShape, section.OutgoingShape = &source, &target
		}
	}
	return nil
}

func (codec *Codec) shapeByID(identifier graphjson.ID) (graph.ShapeID, error) {
	if node, found := codec.nodeIDs.lookup(identifier); found {
		return graph.NodeRef(node), nil
	}
	if port, found := codec.portIDs.lookup(identifier); found {
		return graph.PortRef(port), nil
	}
	return graph.ShapeID{}, fmt.Errorf("referenced shape does not exist: %s", identifier.String())
}

// importProperties parses and stores options in the order optionKeys gives.
func (codec *Codec) importProperties(
	properties *graph.PropertyMap,
	options graphjson.LayoutOptions,
	decodedOrder []string,
) {
	keys, resolved, raws := codec.orderedOptions(options, decodedOrder)
	for index, key := range keys {
		option := codec.resolve(resolved, index, key)
		if option == nil {
			continue
		}
		value, valid := codec.parse(option, codec.raw(raws, index, options, key))
		if valid {
			properties.SetRawKey(option.propertyKey(), value)
		}
	}
}

// maxDirectOrder bounds the quadratic duplicate check of optionKeys.
const maxDirectOrder = 64

// rawBufferSize bounds the orders whose values orderedOptions keeps in the
// codec; longer orders take the general path, which has the same result.
const rawBufferSize = 32

// optionKeys returns the order in which options are applied: layoutOptionKeys's
// result. When the decoded member order lists every key of options exactly
// once, that result is the decoded order itself, returned without copying.
func (codec *Codec) optionKeys(options graphjson.LayoutOptions, decodedOrder []string) []string {
	if len(options) == 0 {
		return nil
	}
	if len(decodedOrder) == len(options) && len(decodedOrder) <= maxDirectOrder && listsEachKeyOnce(options, decodedOrder) {
		return decodedOrder
	}
	return codec.layoutOptionKeys(options, decodedOrder)
}

// listsEachKeyOnce reports whether every entry of order is a key of options
// and no entry repeats; with len(order) == len(options) the entries are then
// exactly the keys of options.
func listsEachKeyOnce(options graphjson.LayoutOptions, order []string) bool {
	for index, key := range order {
		if _, found := options[key]; !found {
			return false
		}
		for _, earlier := range order[:index] {
			if earlier == key {
				return false
			}
		}
	}
	return true
}

func (codec *Codec) layoutOptionKeys(options graphjson.LayoutOptions, decodedOrder []string) []string {
	keys := make([]string, 0, len(options))
	seen := make(map[string]struct{}, len(options))
	for _, key := range decodedOrder {
		if _, found := options[key]; !found {
			continue
		}
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	remainder := make([]string, 0, len(options)-len(keys))
	for key := range options {
		if _, found := seen[key]; !found {
			remainder = append(remainder, key)
		}
	}
	if decodedOrder == nil {
		sort.Slice(remainder, func(left, right int) bool {
			leftKey, rightKey := remainder[left], remainder[right]
			leftOption, rightOption := codec.Options.BySuffix(leftKey), codec.Options.BySuffix(rightKey)
			if leftOption != nil && leftOption == rightOption {
				leftRank := optionSpellingRank(leftOption, leftKey)
				rightRank := optionSpellingRank(rightOption, rightKey)
				if leftRank != rightRank {
					return leftRank < rightRank
				}
			}
			return leftKey < rightKey
		})
	} else {
		sort.Strings(remainder)
	}
	return append(keys, remainder...)
}

func optionSpellingRank(option *OptionData, key string) int {
	if key == option.ID {
		return 4
	}
	for _, legacy := range option.LegacyIDs {
		if key == legacy {
			return 1
		}
	}
	if dotSuffix(option.ID, key) {
		return 3
	}
	for _, legacy := range option.LegacyIDs {
		if dotSuffix(legacy, key) {
			return 0
		}
	}
	return 0
}

func (codec *Codec) importIndividualSpacings(
	properties *graph.PropertyMap,
	options graphjson.LayoutOptions,
	decodedOrder []string,
) {
	if options == nil {
		return
	}
	individual := &IndividualSpacings{}
	codec.importProperties(&individual.Properties, options, decodedOrder)
	graph.Set(properties, SpacingIndividual, individual)
}

func optionString(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'g', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'g', -1, 32)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprint(value)
		}
		return string(encoded)
	}
}

func importShape(shape *graph.Shape, x, y, width, height *float64) {
	if x != nil {
		shape.X, shape.Presence = finite(*x), shape.Presence|graph.HasX
	}
	if y != nil {
		shape.Y, shape.Presence = finite(*y), shape.Presence|graph.HasY
	}
	if width != nil {
		shape.Width, shape.Presence = finite(*width), shape.Presence|graph.HasWidth
	}
	if height != nil {
		shape.Height, shape.Presence = finite(*height), shape.Presence|graph.HasHeight
	}
}

func finite(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}
func fromJSONPoint(point graphjson.Point) geometry.Point {
	return geometry.Point{X: finite(point.X), Y: finite(point.Y)}
}
func toJSONPoint(point geometry.Point) graphjson.Point {
	return graphjson.Point{X: point.X, Y: point.Y}
}

type coordinateContext struct {
	nodeGlobal     []geometry.Point
	portGlobal     []geometry.Point
	labelGlobal    []geometry.Point
	nodeShapeMode  []string
	portShapeMode  []string
	edgeShapeMode  []string
	labelShapeMode []string
	nodeEdgeMode   []string
	portEdgeMode   []string
	edgeEdgeMode   []string
	labelEdgeMode  []string
}

// table returns n zero elements: from spare (kept by the codec for the next
// transfer, zeroed first) when it has room, else new.
func table[T any](spare *[]T, n int) []T {
	if cap(*spare) < n {
		*spare = make([]T, n)
		return *spare
	}
	values := (*spare)[:n]
	clear(values)
	return values
}

func resolveCoordinateMode(value, inherited, fallback string) string {
	if value == "" || value == "INHERIT" {
		if inherited != "" {
			return inherited
		}
		return fallback
	}
	return value
}

func (codec *Codec) coordinates(model *graph.Model) (*coordinateContext, error) {
	return codec.coordinatesOf(model, true)
}

// coordinateMode is graph.Get of one of the JSON coordinate options: the
// stored string, else the default (those defaults are not materialized).
func coordinateMode(properties *graph.PropertyMap, property graph.Property[string]) string {
	if value, found := graph.TryGet(properties, property); found {
		return value
	}
	return "INHERIT"
}

// coordinatesOf computes the coordinate context, or with all false only its
// node tables, which are computed first and from nodes alone.
func (codec *Codec) coordinatesOf(model *graph.Model, all bool) (*coordinateContext, error) {
	if len(codec.edgeParents) != len(model.Edges) {
		return nil, fmt.Errorf("edge parent bindings do not match imported model")
	}
	// Two blocks cut into the eleven tables (each capped at its length).
	nodes, ports, edges, labels := len(model.Nodes), len(model.Ports), len(model.Edges), len(model.Labels)
	if !all {
		ports, edges, labels = 0, 0, 0
	}
	points := table(&codec.sparePoints, nodes+ports+labels)
	modes := table(&codec.spareModes, 2*(nodes+ports+edges+labels))
	cutPoints := func(length int) []geometry.Point {
		cut := points[:length:length]
		points = points[length:]
		return cut
	}
	cutModes := func(length int) []string {
		cut := modes[:length:length]
		modes = modes[length:]
		return cut
	}
	// The context is the codec's (one transfer at a time), like its tables.
	context := &codec.context
	*context = coordinateContext{
		nodeGlobal:     cutPoints(nodes),
		portGlobal:     cutPoints(ports),
		labelGlobal:    cutPoints(labels),
		nodeShapeMode:  cutModes(nodes),
		portShapeMode:  cutModes(ports),
		edgeShapeMode:  cutModes(edges),
		labelShapeMode: cutModes(labels),
		nodeEdgeMode:   cutModes(nodes),
		portEdgeMode:   cutModes(ports),
		edgeEdgeMode:   cutModes(edges),
		labelEdgeMode:  cutModes(labels),
	}

	// Arena creation order is parent-before-child. Record absolute coordinates
	// independently of the requested JSON output modes, matching JsonImporter.
	for index := range model.Nodes {
		node := model.Node(graph.NodeID(index))
		var parentGlobal geometry.Point
		parentShapeMode, parentEdgeMode := "", ""
		if node.Parent != nil {
			parent := int(*node.Parent)
			parentGlobal = context.nodeGlobal[parent]
			parentShapeMode = context.nodeShapeMode[parent]
			parentEdgeMode = context.nodeEdgeMode[parent]
		}
		context.nodeGlobal[index] = node.Shape.Position().Add(parentGlobal)
		context.nodeShapeMode[index] = resolveCoordinateMode(
			coordinateMode(&node.Properties, JSONShapeCoords), parentShapeMode, "PARENT")
		context.nodeEdgeMode[index] = resolveCoordinateMode(
			coordinateMode(&node.Properties, JSONEdgeCoords), parentEdgeMode, "CONTAINER")
	}
	if !all {
		return context, nil
	}

	for index := range model.Ports {
		port := model.Port(graph.PortID(index))
		parent := int(port.Parent)
		context.portGlobal[index] = port.Shape.Position().Add(context.nodeGlobal[parent])
		context.portShapeMode[index] = resolveCoordinateMode(
			graph.Get(&port.Properties, JSONShapeCoords), context.nodeShapeMode[parent], "PARENT")
		context.portEdgeMode[index] = resolveCoordinateMode(
			graph.Get(&port.Properties, JSONEdgeCoords), context.nodeEdgeMode[parent], "CONTAINER")
	}

	for index := range model.Edges {
		edge := model.Edge(graph.EdgeID(index))
		parent := int(codec.edgeParents[index])
		context.edgeShapeMode[index] = resolveCoordinateMode(
			graph.Get(&edge.Properties, JSONShapeCoords), context.nodeShapeMode[parent], "PARENT")
		context.edgeEdgeMode[index] = resolveCoordinateMode(
			graph.Get(&edge.Properties, JSONEdgeCoords), context.nodeEdgeMode[parent], "CONTAINER")
	}

	for index := range model.Labels {
		label := model.Label(graph.LabelID(index))
		parentGlobal, parentShapeMode, parentEdgeMode, err := codec.labelParentCoordinates(context, label.Parent)
		if err != nil {
			return nil, err
		}
		context.labelGlobal[index] = label.Shape.Position().Add(parentGlobal)
		context.labelShapeMode[index] = resolveCoordinateMode(
			graph.Get(&label.Properties, JSONShapeCoords), parentShapeMode, "PARENT")
		context.labelEdgeMode[index] = resolveCoordinateMode(
			graph.Get(&label.Properties, JSONEdgeCoords), parentEdgeMode, "CONTAINER")
	}
	return context, nil
}

func (codec *Codec) labelParentCoordinates(
	context *coordinateContext,
	parent graph.ElementID,
) (geometry.Point, string, string, error) {
	switch parent.Kind {
	case graph.NodeElement:
		index := int(parent.Node)
		return context.nodeGlobal[index], context.nodeShapeMode[index], context.nodeEdgeMode[index], nil
	case graph.PortElement:
		index := int(parent.Port)
		return context.portGlobal[index], context.portShapeMode[index], context.portEdgeMode[index], nil
	case graph.EdgeElement:
		index := int(parent.Edge)
		if index >= len(codec.edgeParents) {
			return geometry.Point{}, "", "", fmt.Errorf("edge parent binding %d is missing", index)
		}
		originalParent := int(codec.edgeParents[index])
		return context.nodeGlobal[originalParent], context.edgeShapeMode[index], context.edgeEdgeMode[index], nil
	case graph.LabelElement:
		index := int(parent.Label)
		return context.labelGlobal[index], context.labelShapeMode[index], context.labelEdgeMode[index], nil
	default:
		return geometry.Point{}, "", "", fmt.Errorf("unknown label parent kind: %d", parent.Kind)
	}
}

// nodeOutputOffset is what TransferLayout adds to the position of node
// index: its parent's absolute position when the parent's shape coordinates
// are ROOT.
func nodeOutputOffset(model *graph.Model, context *coordinateContext, index int) geometry.Point {
	if parent := model.Nodes[index].Parent; parent != nil && context.nodeShapeMode[int(*parent)] == "ROOT" {
		return context.nodeGlobal[int(*parent)]
	}
	return geometry.Point{}
}

func (codec *Codec) edgeOutputOffset(model *graph.Model, context *coordinateContext, edgeID graph.EdgeID) geometry.Point {
	index := int(edgeID)
	originalParent := int(codec.edgeParents[index])
	containingNode := model.Edge(edgeID).ContainingNode
	if containingNode == nil {
		return geometry.Point{}
	}
	containingGlobal := context.nodeGlobal[int(*containingNode)]
	switch context.nodeEdgeMode[originalParent] {
	case "ROOT":
		return containingGlobal
	case "PARENT":
		return containingGlobal.Sub(context.nodeGlobal[originalParent])
	default:
		return geometry.Point{}
	}
}

func (codec *Codec) labelOutputOffset(model *graph.Model, context *coordinateContext, label *graph.Label) geometry.Point {
	switch label.Parent.Kind {
	case graph.NodeElement:
		index := int(label.Parent.Node)
		if context.nodeShapeMode[index] == "ROOT" {
			return context.nodeGlobal[index]
		}
	case graph.PortElement:
		index := int(label.Parent.Port)
		if context.portShapeMode[index] == "ROOT" {
			return context.portGlobal[index]
		}
	case graph.EdgeElement:
		return codec.edgeOutputOffset(model, context, label.Parent.Edge)
	case graph.LabelElement:
		index := int(label.Parent.Label)
		if context.labelShapeMode[index] == "ROOT" {
			return context.labelGlobal[index]
		}
	}
	return geometry.Point{}
}

// TransferLayout copies all layout output into the public objects imported by
// the most recent Import call.
func (codec *Codec) TransferLayout(model *graph.Model) error {
	if model == nil || len(codec.nodeJSON) != len(model.Nodes) {
		return fmt.Errorf("model was not imported by this codec")
	}
	coordinates, err := codec.coordinates(model)
	if err != nil {
		return err
	}
	// The values written below come from blocks allocated once per transfer
	// instead of one allocation each; every pointer and slice still refers to
	// storage of its own (slices are capped at their length).
	var store transferStore
	store.floats = make([]float64, 4*(len(model.Nodes)+len(model.Ports)+len(model.Labels)))
	for index := range model.Nodes {
		output := codec.nodeJSON[index]
		if output == nil {
			return fmt.Errorf("node binding %d is missing", index)
		}
		offset := nodeOutputOffset(model, coordinates, index)
		store.transferShape(&model.Nodes[index].Shape, offset, &output.X, &output.Y, &output.Width, &output.Height)
	}
	for index := range model.Ports {
		output := codec.portJSON[index]
		if output == nil {
			return fmt.Errorf("port binding %d is missing", index)
		}
		parent := int(model.Ports[index].Parent)
		var offset geometry.Point
		if coordinates.nodeShapeMode[parent] == "ROOT" {
			offset = coordinates.nodeGlobal[parent]
		}
		store.transferShape(&model.Ports[index].Shape, offset, &output.X, &output.Y, &output.Width, &output.Height)
	}
	for index := range model.Labels {
		output := codec.labelJSON[index]
		if output == nil {
			return fmt.Errorf("label binding %d is missing", index)
		}
		offset := codec.labelOutputOffset(model, coordinates, &model.Labels[index])
		store.transferShape(&model.Labels[index].Shape, offset, &output.X, &output.Y, &output.Width, &output.Height)
	}
	store.reserveEdges(model)
	for edgeIndex := range model.Edges {
		edgeID := graph.EdgeID(edgeIndex)
		edge := model.Edge(edgeID)
		offset := codec.edgeOutputOffset(model, coordinates, edgeID)
		output := codec.edgeJSON[edgeIndex]
		if output == nil {
			return fmt.Errorf("edge binding %d is missing", edgeIndex)
		}
		sections := store.sectionSlice(len(edge.Sections))
		for position, sectionID := range edge.Sections {
			var sectionOutput *graphjson.EdgeSection
			if int(sectionID) < len(codec.sectionJSON) {
				sectionOutput = codec.sectionJSON[int(sectionID)]
			}
			if sectionOutput == nil {
				sectionOutput = store.newSection()
			}
			if !sectionOutput.ID.IsValid() {
				section := model.Section(sectionID)
				if section.IdentifierPresent {
					sectionOutput.ID = graphjson.StringID(section.Identifier)
				} else {
					sectionOutput.ID = graphjson.StringID(output.ID.String() + "_s" + strconv.Itoa(position))
				}
			}
			sections[position] = sectionOutput
		}
		for position, sectionID := range edge.Sections {
			section := model.Section(sectionID)
			sectionOutput := sections[position]
			sectionOutput.StartPoint = store.point(toJSONPoint(section.Start.Add(offset)))
			sectionOutput.EndPoint = store.point(toJSONPoint(section.End.Add(offset)))
			sectionOutput.BendPoints = store.points(len(section.BendPoints))
			for index, point := range section.BendPoints {
				sectionOutput.BendPoints[index] = toJSONPoint(point.Add(offset))
			}
			if len(section.BendPoints) == 0 {
				sectionOutput.BendPoints = nil
			}
			sectionOutput.IncomingShape = codec.publicShapeID(&store, section.IncomingShape)
			sectionOutput.OutgoingShape = codec.publicShapeID(&store, section.OutgoingShape)
			sectionOutput.IncomingSections = sectionReferenceIDs(section.IncomingSections, edge.Sections, sections)
			sectionOutput.OutgoingSections = sectionReferenceIDs(section.OutgoingSections, edge.Sections, sections)
		}
		output.Sections = sections
		if len(sections) == 0 {
			output.Sections = nil
		}
		if points, found := graph.TryGet(&edge.Properties, JunctionPoints); found {
			output.JunctionPoints = make([]graphjson.Point, len(points))
			for index, point := range points {
				output.JunctionPoints[index] = toJSONPoint(point.Add(offset))
			}
		}
		originalParent := int(codec.edgeParents[edgeIndex])
		if coordinates.nodeEdgeMode[originalParent] == "CONTAINER" {
			if edge.ContainingNode != nil {
				output.Container = store.id(codec.nodeJSON[int(*edge.ContainingNode)].ID)
			} else {
				output.Container = nil
			}
		}
	}
	return nil
}

// transferStore hands out the storage TransferLayout writes into.
type transferStore struct {
	floats       []float64
	sections     []*graphjson.EdgeSection
	newBlock     []graphjson.EdgeSection
	sectionCount int
	pointsAt     []graphjson.Point
	ids          []graphjson.ID
}

func (store *transferStore) reserveEdges(model *graph.Model) {
	sections, bends := 0, 0
	for index := range model.Edges {
		sections += len(model.Edges[index].Sections)
		for _, sectionID := range model.Edges[index].Sections {
			bends += len(model.Section(sectionID).BendPoints)
		}
	}
	store.sectionCount = sections
	store.sections = make([]*graphjson.EdgeSection, sections)
	store.pointsAt = make([]graphjson.Point, 2*sections+bends)
	store.ids = make([]graphjson.ID, 2*sections+len(model.Edges))
}

func (store *transferStore) transferShape(shape *graph.Shape, offset geometry.Point, x, y, width, height **float64) {
	slots := store.floats[:4:4]
	store.floats = store.floats[4:]
	slots[0] = shape.X + offset.X
	slots[1] = shape.Y + offset.Y
	slots[2] = shape.Width
	slots[3] = shape.Height
	*x, *y, *width, *height = &slots[0], &slots[1], &slots[2], &slots[3]
}

func (store *transferStore) sectionSlice(length int) []*graphjson.EdgeSection {
	if length > len(store.sections) {
		return make([]*graphjson.EdgeSection, length)
	}
	slice := store.sections[:length:length]
	store.sections = store.sections[length:]
	return slice
}

func (store *transferStore) newSection() *graphjson.EdgeSection {
	if len(store.newBlock) == 0 {
		// At most one new section per listed section.
		store.newBlock = make([]graphjson.EdgeSection, max(store.sectionCount, 1))
		store.sectionCount = 1
	}
	section := &store.newBlock[0]
	store.newBlock = store.newBlock[1:]
	return section
}

// point and id return a pointer to a copy of value: a slot of the block, or a
// new variable once the block is used up (allocated in that branch only, so
// that value itself does not move to the heap on every call).
func (store *transferStore) point(value graphjson.Point) *graphjson.Point {
	if len(store.pointsAt) == 0 {
		allocated := new(graphjson.Point)
		*allocated = value
		return allocated
	}
	slot := &store.pointsAt[0]
	store.pointsAt = store.pointsAt[1:]
	*slot = value
	return slot
}

func (store *transferStore) points(length int) []graphjson.Point {
	if length > len(store.pointsAt) {
		return make([]graphjson.Point, length)
	}
	slice := store.pointsAt[:length:length]
	store.pointsAt = store.pointsAt[length:]
	return slice
}

func (store *transferStore) id(value graphjson.ID) *graphjson.ID {
	if len(store.ids) == 0 {
		allocated := new(graphjson.ID)
		*allocated = value
		return allocated
	}
	slot := &store.ids[0]
	store.ids = store.ids[1:]
	*slot = value
	return slot
}

func (codec *Codec) publicShapeID(store *transferStore, shape *graph.ShapeID) *graphjson.ID {
	if shape == nil {
		return nil
	}
	if shape.Kind == graph.NodeShape {
		return store.id(codec.nodeJSON[int(shape.Node)].ID)
	}
	return store.id(codec.portJSON[int(shape.Port)].ID)
}

// sectionReferenceIDs maps section references to the IDs of the edge's
// output sections; a reference to a section that is not the edge's is
// dropped. A section listed twice resolves to its last position, as a map
// filled in position order would.
func sectionReferenceIDs(references []graph.SectionID, edgeSections []graph.SectionID, outputs []*graphjson.EdgeSection) []graphjson.ID {
	if len(references) == 0 {
		return nil
	}
	result := make([]graphjson.ID, 0, len(references))
	for _, reference := range references {
		for position := len(edgeSections) - 1; position >= 0; position-- {
			if edgeSections[position] == reference {
				result = append(result, outputs[position].ID)
				break
			}
		}
	}
	return result
}
