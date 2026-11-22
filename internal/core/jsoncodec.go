// Copyright (c) 2017, 2020 Kiel University and others.
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

	"github.com/d2lang/elk-go/graphjson"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/jsonorder"
	"github.com/d2lang/elk-go/internal/reuse"
)

// JSONCodec imports an ELK JSON graph (graphjson) into the algorithm graph
// and transfers the computed layout back into the same graphjson objects. It
// builds the model with the element steps of its Codec, which the typed
// import shares, and keeps what only JSON needs: the JSON element each model
// element was imported from, and the scratch of importing option maps. A
// program that imports only typed requests links none of it, nor graphjson's
// JSON encoding and decoding.
type JSONCodec struct {
	*Codec
	// The JSON element of every model element, by arena position.
	nodeJSON    []*graphjson.Node
	portJSON    []*graphjson.Port
	edgeJSON    []*graphjson.Edge
	labelJSON   []*graphjson.Label
	sectionJSON []*graphjson.EdgeSection

	// keyLists and rawBuffer are orderedOptions' scratch.
	keyLists    [keyListsKept]keyListInfo
	keyListNext int
	rawBuffer   [rawBufferSize]any
}

// NewJSONCodec returns a JSON codec over NewCodec(options).
func NewJSONCodec(options *OptionRegistry) *JSONCodec {
	return &JSONCodec{Codec: NewCodec(options)}
}

// releasedJSONCodecs keeps released JSON codecs for reuse, as releasedCodecs
// keeps codecs (their Codec goes to releasedCodecs).
var releasedJSONCodecs reuse.Pool[JSONCodec]

// AcquireJSONCodec is NewJSONCodec, reusing the storage of JSON codecs given
// to ReleaseJSONCodec and of codecs given to ReleaseCodec.
func AcquireJSONCodec(options *OptionRegistry) *JSONCodec {
	codec := releasedJSONCodecs.Get()
	if codec == nil {
		codec = new(JSONCodec)
	}
	codec.Codec = AcquireCodec(options)
	return codec
}

// ReleaseJSONCodec gives codec's Codec and model to ReleaseCodec and offers
// the rest of codec, emptied, for reuse by a later AcquireJSONCodec. Neither
// codec nor model, nor anything obtained from them, may be used afterwards.
// As with ReleaseCodec, reuse is unobservable (the JSON element tables are
// emptied and the member orders learned from the input forgotten), and a
// released JSON codec refers to nothing of the caller's and keeps at most
// maxRetainedBytes; a larger one goes to the sync.Pool only (gc) or is
// emptied and dropped (TinyGo).
func ReleaseJSONCodec(codec *JSONCodec, model *graph.Model) {
	if codec == nil {
		return
	}
	ReleaseCodec(codec.Codec, model)
	codec.Codec = nil
	codec.emptyTables()
	codec.keyLists = [keyListsKept]keyListInfo{}
	codec.keyListNext = 0
	codec.rawBuffer = [rawBufferSize]any{}
	keep := codec.tableBytes() <= maxRetainedBytes
	if !keep && !reuse.OversizedToPool {
		// Dropped, and emptied (see ReleaseCodec).
		*codec = JSONCodec{}
		return
	}
	releasedJSONCodecs.Put(codec, keep)
}

// reset forgets the previous import, as Codec.reset does, with the JSON
// elements bound to its elements.
func (codec *JSONCodec) reset() {
	codec.Codec.reset()
	codec.emptyTables()
}

// emptyTables empties the JSON element tables, keeping their storage.
func (codec *JSONCodec) emptyTables() {
	codec.nodeJSON = emptied(codec.nodeJSON)
	codec.portJSON = emptied(codec.portJSON)
	codec.edgeJSON = emptied(codec.edgeJSON)
	codec.labelJSON = emptied(codec.labelJSON)
	codec.sectionJSON = emptied(codec.sectionJSON)
}

// tableBytes bounds the storage of the JSON element tables (the scratch is
// bounded by construction).
func (codec *JSONCodec) tableBytes() int {
	return pointerBytes * (cap(codec.nodeJSON) + cap(codec.portJSON) + cap(codec.edgeJSON) + cap(codec.labelJSON) + cap(codec.sectionJSON))
}

// recordJSON returns table with element recorded at index, the arena
// position of the model element imported from it, growing table as needed.
func recordJSON[T any](table []*T, index int, element *T) []*T {
	for len(table) <= index {
		table = append(table, nil)
	}
	table[index] = element
	return table
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

// reserve sizes the model's element arenas, the codec's bindings and the
// JSON element tables for importing input (see reserveCounts).
func (codec *JSONCodec) reserve(model *graph.Model, input *graphjson.Graph) {
	var counts importCounts
	counts.add(input)
	codec.reserveCounts(model, counts)
	codec.nodeJSON = binding(codec.nodeJSON, counts.nodes)
	if counts.ports > 0 {
		codec.portJSON = binding(codec.portJSON, counts.ports)
	}
	if counts.labels > 0 {
		codec.labelJSON = binding(codec.labelJSON, counts.labels)
	}
	if counts.edges > 0 {
		codec.edgeJSON = binding(codec.edgeJSON, counts.edges)
	}
}

// Import validates graph references and constructs the native arena.
func (codec *JSONCodec) Import(input *graphjson.Graph) (*graph.Model, error) {
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

func (codec *JSONCodec) rejectUnconstructableJSONProperties(node *graphjson.Node) error {
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
func (codec *JSONCodec) rejectUnconstructable(options graphjson.LayoutOptions, keys []string) error {
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

// importNodeOptions imports a node's options as importProperties does,
// checking each one for rejectUnconstructable on the way: the same keys in
// the same order, so the first error is the same, and an error discards the
// model.
func (codec *JSONCodec) importNodeOptions(properties *graph.PropertyMap, options graphjson.LayoutOptions, decodedOrder []string) error {
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
func (codec *JSONCodec) keyList(keys []string) *keyListInfo {
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

// orderedOptions returns the order in which options are applied (optionKeys)
// and, when that is the decoded order itself, the keys resolved so far and
// the values (in a buffer reused by the next call).
func (codec *JSONCodec) orderedOptions(options graphjson.LayoutOptions, decodedOrder []string) ([]string, []*OptionData, []any) {
	if len(options) == 0 {
		return nil, nil, nil
	}
	if count := len(decodedOrder); count == len(options) && count <= rawBufferSize {
		info := codec.keyList(decodedOrder)
		if info.unique {
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

func (codec *JSONCodec) resolve(resolved []*OptionData, index int, key string) *OptionData {
	if resolved != nil {
		return resolved[index]
	}
	return codec.Options.BySuffix(key)
}

func (codec *JSONCodec) raw(raws []any, index int, options graphjson.LayoutOptions, key string) any {
	if raws != nil {
		return raws[index]
	}
	return options[key]
}

func (codec *JSONCodec) importNode(model *graph.Model, id graph.NodeID, input *graphjson.Node) error {
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
	if err := codec.bindNode(model, id, input.ID); err != nil {
		return err
	}
	codec.nodeJSON = recordJSON(codec.nodeJSON, int(id), input)
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

func (codec *JSONCodec) importPort(model *graph.Model, id graph.PortID, input *graphjson.Port) error {
	if err := codec.bindPort(model, id, input.ID); err != nil {
		return err
	}
	codec.portJSON = recordJSON(codec.portJSON, int(id), input)
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

func (codec *JSONCodec) importLabel(model *graph.Model, parent graph.ElementID, input *graphjson.Label) graph.LabelID {
	text := ""
	textPresent := input.Text != nil
	if input.Text != nil {
		text = *input.Text
	}
	id := codec.addLabel(model, parent, text)
	codec.labelJSON = recordJSON(codec.labelJSON, int(id), input)
	label := model.Label(id)
	label.TextPresent = textPresent
	if input.ID != nil && input.ID.IsValid() {
		label.Identifier, label.IdentifierPresent = input.ID.String(), true
	}
	importShape(&label.Shape, input.X, input.Y, input.Width, input.Height)
	codec.importProperties(&label.Properties, input.LayoutOptions, jsonorder.LayoutOptionKeys(input))
	return id
}

func (codec *JSONCodec) importEdges(model *graph.Model, parent graph.NodeID, input *graphjson.Node) error {
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

func (codec *JSONCodec) importEdge(model *graph.Model, parent graph.NodeID, input *graphjson.Edge) (graph.EdgeID, error) {
	if !input.ID.IsValid() {
		return 0, fmt.Errorf("every edge must have an id")
	}
	id := codec.addEdge(model, parent, input.ID)
	codec.edgeJSON = recordJSON(codec.edgeJSON, int(id), input)
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

func (codec *JSONCodec) importPrimitiveEdge(model *graph.Model, id graph.EdgeID, input *graphjson.Edge) error {
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

func (codec *JSONCodec) importSections(model *graph.Model, edgeID graph.EdgeID, input *graphjson.Edge) error {
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
		codec.sectionJSON = recordJSON(codec.sectionJSON, int(id), sectionInput)
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

// importProperties parses and stores options in the order optionKeys gives.
func (codec *JSONCodec) importProperties(
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
func (codec *JSONCodec) optionKeys(options graphjson.LayoutOptions, decodedOrder []string) []string {
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

func (codec *JSONCodec) importIndividualSpacings(
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

func fromJSONPoint(point graphjson.Point) geometry.Point {
	return geometry.Point{X: finite(point.X), Y: finite(point.Y)}
}
func toJSONPoint(point geometry.Point) graphjson.Point {
	return graphjson.Point{X: point.X, Y: point.Y}
}

// coordinates computes the whole coordinate context: the node tables, then
// those of ports, edges and labels.
func (codec *JSONCodec) coordinates(model *graph.Model) (*coordinateContext, error) {
	context, err := codec.nodeCoordinates(model, true)
	if err != nil {
		return nil, err
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

func (codec *JSONCodec) labelParentCoordinates(
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

func (codec *JSONCodec) labelOutputOffset(model *graph.Model, context *coordinateContext, label *graph.Label) geometry.Point {
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
func (codec *JSONCodec) TransferLayout(model *graph.Model) error {
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

func (codec *JSONCodec) publicShapeID(store *transferStore, shape *graph.ShapeID) *graphjson.ID {
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
