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

	"github.com/d2lang/elk-go/graphjson"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

// Codec imports ELK JSON into the algorithm graph and transfers computed
// layout back into the same public graph objects.
type Codec struct {
	Options     *OptionRegistry
	nodeIDs     map[graphjson.ID]graph.NodeID
	portIDs     map[graphjson.ID]graph.PortID
	nodeJSON    []*graphjson.Node
	portJSON    []*graphjson.Port
	edgeJSON    []*graphjson.Edge
	edgeParents []graph.NodeID
	labelJSON   []*graphjson.Label
	sectionJSON []*graphjson.EdgeSection
}

func NewCodec(options *OptionRegistry) *Codec {
	if options == nil {
		options = NewOptionRegistry()
	}
	return &Codec{Options: options}
}

func (codec *Codec) reset() {
	codec.nodeIDs = make(map[graphjson.ID]graph.NodeID)
	codec.portIDs = make(map[graphjson.ID]graph.PortID)
	codec.nodeJSON = nil
	codec.portJSON = nil
	codec.edgeJSON = nil
	codec.edgeParents = nil
	codec.labelJSON = nil
	codec.sectionJSON = nil
}

// Import validates graph references and constructs the native arena.
func (codec *Codec) Import(input *graphjson.Graph) (*graph.Model, error) {
	if input == nil {
		return nil, fmt.Errorf("ELK graph must not be nil")
	}
	if err := codec.rejectUnconstructableJSONProperties(input); err != nil {
		return nil, err
	}
	codec.reset()
	model := graph.NewModel()
	if err := codec.importNode(model, model.Root, input); err != nil {
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
	for _, key := range codec.layoutOptionKeys(node.LayoutOptions, node.LayoutOptionKeys()) {
		option := codec.Options.BySuffix(key)
		if option == nil || node.LayoutOptions[key] == nil {
			continue
		}
		switch option.ID {
		case TopdownSizeApproximator.ID:
			return fmt.Errorf("Couldn't create new instance of property '%s'. Make sure its type is registered with the ElkReflect utility class.",
				TopdownSizeApproximator.ID)
		case "org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cmEnforcedGroupOrders":
			return fmt.Errorf("Severe implementation error in the Json to ElkGraph importer.")
		}
	}
	for _, child := range node.Children {
		if err := codec.rejectUnconstructableJSONProperties(child); err != nil {
			return err
		}
	}
	return nil
}

func (codec *Codec) importNode(model *graph.Model, id graph.NodeID, input *graphjson.Node) error {
	if input == nil {
		return nil
	}
	if !input.ID.IsValid() {
		return fmt.Errorf("every node must have an id")
	}
	if _, duplicate := codec.nodeIDs[input.ID]; duplicate {
		return fmt.Errorf("duplicate node id: %s", input.ID.String())
	}
	codec.nodeIDs[input.ID] = id
	for len(codec.nodeJSON) <= int(id) {
		codec.nodeJSON = append(codec.nodeJSON, nil)
	}
	codec.nodeJSON[int(id)] = input
	node := model.Node(id)
	node.Identifier, node.IdentifierPresent = input.ID.String(), true
	importShape(&node.Shape, input.X, input.Y, input.Width, input.Height)
	codec.importProperties(&node.Properties, input.LayoutOptions, input.LayoutOptionKeys())
	codec.importIndividualSpacings(&node.Properties, input.IndividualSpacings, input.IndividualSpacingKeys())
	for _, portInput := range input.Ports {
		if portInput == nil {
			continue
		}
		portID := model.AddPort(id)
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
		childID := model.AddNode(&id)
		if err := codec.importNode(model, childID, childInput); err != nil {
			return err
		}
	}
	return nil
}

func (codec *Codec) importPort(model *graph.Model, id graph.PortID, input *graphjson.Port) error {
	if !input.ID.IsValid() {
		return fmt.Errorf("every port must have an id")
	}
	if _, duplicate := codec.portIDs[input.ID]; duplicate {
		return fmt.Errorf("duplicate port id: %s", input.ID.String())
	}
	codec.portIDs[input.ID] = id
	for len(codec.portJSON) <= int(id) {
		codec.portJSON = append(codec.portJSON, nil)
	}
	codec.portJSON[int(id)] = input
	port := model.Port(id)
	port.Identifier, port.IdentifierPresent = input.ID.String(), true
	importShape(&port.Shape, input.X, input.Y, input.Width, input.Height)
	codec.importProperties(&port.Properties, input.LayoutOptions, input.LayoutOptionKeys())
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
	id := model.AddLabel(parent, text)
	for len(codec.labelJSON) <= int(id) {
		codec.labelJSON = append(codec.labelJSON, nil)
	}
	codec.labelJSON[int(id)] = input
	label := model.Label(id)
	label.TextPresent = textPresent
	if input.ID != nil && input.ID.IsValid() {
		label.Identifier, label.IdentifierPresent = input.ID.String(), true
	}
	importShape(&label.Shape, input.X, input.Y, input.Width, input.Height)
	codec.importProperties(&label.Properties, input.LayoutOptions, input.LayoutOptionKeys())
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
	id := model.AddEdge(&parent)
	for len(codec.edgeJSON) <= int(id) {
		codec.edgeJSON = append(codec.edgeJSON, nil)
		codec.edgeParents = append(codec.edgeParents, 0)
	}
	codec.edgeJSON[int(id)] = input
	codec.edgeParents[int(id)] = parent
	edge := model.Edge(id)
	edge.Identifier, edge.IdentifierPresent = input.ID.String(), true
	codec.importProperties(&edge.Properties, input.LayoutOptions, input.LayoutOptionKeys())
	if input.Sources != nil || input.Targets != nil {
		for _, source := range input.Sources {
			shape, err := codec.shapeByID(source)
			if err != nil {
				return 0, err
			}
			model.AddSource(id, shape)
		}
		for _, target := range input.Targets {
			shape, err := codec.shapeByID(target)
			if err != nil {
				return 0, err
			}
			model.AddTarget(id, shape)
		}
		if len(edge.Sources) == 0 || len(edge.Targets) == 0 {
			return 0, fmt.Errorf("an edge must have at least one source and one target (edge id: %q)", input.ID.String())
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
	sourceNode, found := codec.nodeIDs[*input.Source]
	if !found {
		return fmt.Errorf("an edge must have a source node (edge id: %q)", input.ID.String())
	}
	source := graph.NodeRef(sourceNode)
	if input.SourcePort != nil {
		port, found := codec.portIDs[*input.SourcePort]
		if !found || model.Port(port).Parent != sourceNode {
			return fmt.Errorf("source port must belong to the source node (edge id: %q)", input.ID.String())
		}
		source = graph.PortRef(port)
	}
	if input.Target == nil {
		return fmt.Errorf("an edge must have a target node (edge id: %q)", input.ID.String())
	}
	targetNode, found := codec.nodeIDs[*input.Target]
	if !found {
		return fmt.Errorf("an edge must have a target node (edge id: %q)", input.ID.String())
	}
	target := graph.NodeRef(targetNode)
	if input.TargetPort != nil {
		port, found := codec.portIDs[*input.TargetPort]
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
		codec.importProperties(&section.Properties, sectionInput.LayoutOptions, sectionInput.LayoutOptionKeys())
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
	if node, found := codec.nodeIDs[identifier]; found {
		return graph.NodeRef(node), nil
	}
	if port, found := codec.portIDs[identifier]; found {
		return graph.PortRef(port), nil
	}
	return graph.ShapeID{}, fmt.Errorf("referenced shape does not exist: %s", identifier.String())
}

func (codec *Codec) importProperties(
	properties *graph.PropertyMap,
	options graphjson.LayoutOptions,
	decodedOrder []string,
) {
	for _, key := range codec.layoutOptionKeys(options, decodedOrder) {
		raw := options[key]
		option := codec.Options.BySuffix(key)
		if option == nil {
			continue
		}
		value, valid := option.ParseValue(optionString(raw))
		if valid {
			properties.SetRaw(option.ID, value)
		}
	}
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
	if len(codec.edgeParents) != len(model.Edges) {
		return nil, fmt.Errorf("edge parent bindings do not match imported model")
	}
	context := &coordinateContext{
		nodeGlobal:     make([]geometry.Point, len(model.Nodes)),
		portGlobal:     make([]geometry.Point, len(model.Ports)),
		labelGlobal:    make([]geometry.Point, len(model.Labels)),
		nodeShapeMode:  make([]string, len(model.Nodes)),
		portShapeMode:  make([]string, len(model.Ports)),
		edgeShapeMode:  make([]string, len(model.Edges)),
		labelShapeMode: make([]string, len(model.Labels)),
		nodeEdgeMode:   make([]string, len(model.Nodes)),
		portEdgeMode:   make([]string, len(model.Ports)),
		edgeEdgeMode:   make([]string, len(model.Edges)),
		labelEdgeMode:  make([]string, len(model.Labels)),
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
			graph.Get(&node.Properties, JSONShapeCoords), parentShapeMode, "PARENT")
		context.nodeEdgeMode[index] = resolveCoordinateMode(
			graph.Get(&node.Properties, JSONEdgeCoords), parentEdgeMode, "CONTAINER")
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
	for index := range model.Nodes {
		output := codec.nodeJSON[index]
		if output == nil {
			return fmt.Errorf("node binding %d is missing", index)
		}
		var offset geometry.Point
		if parent := model.Nodes[index].Parent; parent != nil && coordinates.nodeShapeMode[int(*parent)] == "ROOT" {
			offset = coordinates.nodeGlobal[int(*parent)]
		}
		transferShape(&model.Nodes[index].Shape, offset, &output.X, &output.Y, &output.Width, &output.Height)
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
		transferShape(&model.Ports[index].Shape, offset, &output.X, &output.Y, &output.Width, &output.Height)
	}
	for index := range model.Labels {
		output := codec.labelJSON[index]
		if output == nil {
			return fmt.Errorf("label binding %d is missing", index)
		}
		offset := codec.labelOutputOffset(model, coordinates, &model.Labels[index])
		transferShape(&model.Labels[index].Shape, offset, &output.X, &output.Y, &output.Width, &output.Height)
	}
	for edgeIndex := range model.Edges {
		edgeID := graph.EdgeID(edgeIndex)
		edge := model.Edge(edgeID)
		offset := codec.edgeOutputOffset(model, coordinates, edgeID)
		output := codec.edgeJSON[edgeIndex]
		if output == nil {
			return fmt.Errorf("edge binding %d is missing", edgeIndex)
		}
		sections := make([]*graphjson.EdgeSection, len(edge.Sections))
		sectionIDs := make(map[graph.SectionID]graphjson.ID, len(edge.Sections))
		for position, sectionID := range edge.Sections {
			var sectionOutput *graphjson.EdgeSection
			if int(sectionID) < len(codec.sectionJSON) {
				sectionOutput = codec.sectionJSON[int(sectionID)]
			}
			if sectionOutput == nil {
				sectionOutput = &graphjson.EdgeSection{}
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
			sectionIDs[sectionID] = sectionOutput.ID
		}
		for position, sectionID := range edge.Sections {
			section := model.Section(sectionID)
			sectionOutput := sections[position]
			start, end := toJSONPoint(section.Start.Add(offset)), toJSONPoint(section.End.Add(offset))
			sectionOutput.StartPoint, sectionOutput.EndPoint = &start, &end
			sectionOutput.BendPoints = make([]graphjson.Point, len(section.BendPoints))
			for index, point := range section.BendPoints {
				sectionOutput.BendPoints[index] = toJSONPoint(point.Add(offset))
			}
			if len(section.BendPoints) == 0 {
				sectionOutput.BendPoints = nil
			}
			sectionOutput.IncomingShape = codec.publicShapeID(model, section.IncomingShape)
			sectionOutput.OutgoingShape = codec.publicShapeID(model, section.OutgoingShape)
			sectionOutput.IncomingSections = sectionReferenceIDs(section.IncomingSections, sectionIDs)
			sectionOutput.OutgoingSections = sectionReferenceIDs(section.OutgoingSections, sectionIDs)
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
				container := codec.nodeJSON[int(*edge.ContainingNode)].ID
				output.Container = graphjson.IDPtr(container)
			} else {
				output.Container = nil
			}
		}
	}
	return nil
}

func transferShape(shape *graph.Shape, offset geometry.Point, x, y, width, height **float64) {
	*x = graphjson.Float64(shape.X + offset.X)
	*y = graphjson.Float64(shape.Y + offset.Y)
	*width = graphjson.Float64(shape.Width)
	*height = graphjson.Float64(shape.Height)
}

func (codec *Codec) publicShapeID(model *graph.Model, shape *graph.ShapeID) *graphjson.ID {
	if shape == nil {
		return nil
	}
	if shape.Kind == graph.NodeShape {
		id := codec.nodeJSON[int(shape.Node)].ID
		return graphjson.IDPtr(id)
	}
	id := codec.portJSON[int(shape.Port)].ID
	return graphjson.IDPtr(id)
}

func sectionReferenceIDs(references []graph.SectionID, ids map[graph.SectionID]graphjson.ID) []graphjson.ID {
	if len(references) == 0 {
		return nil
	}
	result := make([]graphjson.ID, 0, len(references))
	for _, reference := range references {
		if id, found := ids[reference]; found {
			result = append(result, id)
		}
	}
	return result
}
