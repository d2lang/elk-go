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
	"github.com/d2lang/elk-go/internal/reuse"
)

// Codec builds the algorithm graph of a layout from its elements, with the
// element steps that JSONCodec's import of an ELK JSON graph and the typed
// import (typed.go) share, and computes where the layout is read back from.
// It holds no JSON element: a program that imports only typed requests does
// not link the JSON import (see JSONCodec).
type Codec struct {
	Options     *OptionRegistry
	nodeIDs     idTable[graph.NodeID]
	portIDs     idTable[graph.PortID]
	edgeParents []graph.NodeID

	// knownOrders outlive imports (they hold copies of the keys).
	knownOrders    [knownOrdersKept]knownOrder
	knownOrderNext int
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
// Reuse is unobservable: the bindings are emptied, and an import appends
// every element to its arena before it is read (only the importer appends to
// the node, port, label and edge arenas, so their capacity cannot be
// observed either). A released codec refers to nothing of the caller's (the
// caches that outlive imports hold copies of short keys and values only) and
// keeps at most maxRetainedBytes; a larger one goes to the sync.Pool only
// (gc) or is emptied and dropped (TinyGo).
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
// (knownOrders, parsed) are bounded by construction.
func (codec *Codec) retainedBytes() int {
	bytes := codec.nodeIDs.bytes() + codec.portIDs.bytes()
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
	codec.edgeParents = emptied(codec.edgeParents)
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

// reserveCounts sizes the model's element arenas and the codec's bindings
// for an import of at most the given numbers of elements (more only cost
// growth). Only the importer appends to these arenas, and it holds no element
// pointer across an append, so their capacity is unobservable.
func (codec *Codec) reserveCounts(model *graph.Model, counts importCounts) {
	codec.nodeIDs.hint = counts.nodes
	if model.Nodes == nil {
		// A reused model struct (see newModel): its root, node 0, is a zero
		// node, as graph.NewModel's.
		model.Nodes = append(arena(&codec.spareNodes, max(counts.nodes, 1)), graph.Node{})
	} else {
		model.Nodes = append(arena(&codec.spareNodes, max(counts.nodes, len(model.Nodes))), model.Nodes...)
	}
	if counts.ports > 0 {
		codec.portIDs.hint = counts.ports
		model.Ports = arena(&codec.sparePorts, counts.ports)
	}
	if counts.labels > 0 {
		model.Labels = arena(&codec.spareLabels, counts.labels)
	}
	if counts.edges > 0 {
		codec.edgeParents = binding(codec.edgeParents, counts.edges)
		model.Edges = arena(&codec.spareEdges, counts.edges)
	}
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

// The element steps below are shared by JSONCodec.Import and the typed
// import (see typed.go), which builds the same model from the same elements
// given as Go values.

// addChild adds an empty child node to parent.
func (codec *Codec) addChild(model *graph.Model, parent graph.NodeID) graph.NodeID {
	childID := model.AddNode(&parent)
	codec.recycleInto(&model.Node(childID).Properties, codec.recycledNodes, int(childID))
	return childID
}

// bindNode checks and records the identifier of node id.
func (codec *Codec) bindNode(model *graph.Model, id graph.NodeID, identifier graphjson.ID) error {
	if !identifier.IsValid() {
		return fmt.Errorf("every node must have an id")
	}
	if !codec.nodeIDs.bindNew(identifier, id) {
		// The import fails: nothing reads the overwritten binding.
		return fmt.Errorf("duplicate node id: %s", identifier.String())
	}
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
func (codec *Codec) bindPort(model *graph.Model, id graph.PortID, identifier graphjson.ID) error {
	if !identifier.IsValid() {
		return fmt.Errorf("every port must have an id")
	}
	if !codec.portIDs.bindNew(identifier, id) {
		// The import fails: nothing reads the overwritten binding.
		return fmt.Errorf("duplicate port id: %s", identifier.String())
	}
	port := model.Port(id)
	port.Identifier, port.IdentifierPresent = identifier.String(), true
	return nil
}

// addLabel adds a label with text to parent.
func (codec *Codec) addLabel(model *graph.Model, parent graph.ElementID, text string) graph.LabelID {
	id := model.AddLabel(parent, text)
	codec.recycleInto(&model.Label(id).Properties, codec.recycledLabels, int(id))
	return id
}

// addEdge adds an edge contained in parent with a valid identifier.
func (codec *Codec) addEdge(model *graph.Model, parent graph.NodeID, identifier graphjson.ID) graph.EdgeID {
	id := model.AddEdge(&parent)
	codec.recycleInto(&model.Edge(id).Properties, codec.recycledEdges, int(id))
	for len(codec.edgeParents) <= int(id) {
		codec.edgeParents = append(codec.edgeParents, 0)
	}
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

func (codec *Codec) shapeByID(identifier graphjson.ID) (graph.ShapeID, error) {
	if node, found := codec.nodeIDs.lookup(identifier); found {
		return graph.NodeRef(node), nil
	}
	if port, found := codec.portIDs.lookup(identifier); found {
		return graph.PortRef(port), nil
	}
	return graph.ShapeID{}, fmt.Errorf("referenced shape does not exist: %s", identifier.String())
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

// coordinateMode is graph.Get of one of the JSON coordinate options: the
// stored string, else the default (those defaults are not materialized).
func coordinateMode(properties *graph.PropertyMap, property graph.Property[string]) string {
	if value, found := graph.TryGet(properties, property); found {
		return value
	}
	return "INHERIT"
}

// nodeCoordinates computes the node tables of the coordinate context, which
// come first and from nodes alone. The other tables are cut with room for
// every port, edge and label of the model when all is set (for
// JSONCodec.coordinates to fill), else empty.
func (codec *Codec) nodeCoordinates(model *graph.Model, all bool) (*coordinateContext, error) {
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
	return context, nil
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
