// Copyright (c) 2008, 2018 Kiel University and others.
// Copyright (c) 2018, 2020 TypeFox GmbH and others.
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

	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

const DefaultAlgorithmID = "org.eclipse.elk.layered"

// Engine applies registered providers recursively across graph hierarchy.
type Engine struct {
	Options    *OptionRegistry
	Algorithms *AlgorithmRegistry
	Monitor    ProgressMonitor
}

func NewEngine(options *OptionRegistry, algorithms *AlgorithmRegistry) *Engine {
	if options == nil {
		options = NewOptionRegistry()
	}
	if algorithms == nil {
		algorithms = NewAlgorithmRegistry()
	}
	return &Engine{Options: options, Algorithms: algorithms, Monitor: NullProgressMonitor}
}

func (engine *Engine) Layout(model *graph.Model) error {
	if model == nil || len(model.Nodes) == 0 {
		return fmt.Errorf("layout graph must contain a root node")
	}
	if engine.Monitor == nil {
		engine.Monitor = NullProgressMonitor
	}
	if err := engine.resolveAlgorithms(model); err != nil {
		return err
	}
	_, err := engine.layoutRecursively(model, model.Root, engine.Monitor)
	return err
}

func (engine *Engine) resolveAlgorithms(model *graph.Model) error {
	nodes := append([]graph.NodeID{model.Root}, model.Descendants(model.Root)...)
	for _, nodeID := range nodes {
		node := model.Node(nodeID)
		if graph.Get(&node.Properties, NoLayout) {
			continue
		}
		hasLayoutContent := len(node.Children) > 0 || len(gatherInsideSelfLoops(model, nodeID)) > 0
		algorithmID := graph.Get(&node.Properties, Algorithm)
		algorithm := engine.Algorithms.BySuffix(algorithmID)
		if algorithm == nil && algorithmID == "" && hasLayoutContent {
			algorithm = engine.Algorithms.BySuffix(DefaultAlgorithmID)
		}
		if algorithm == nil {
			if !hasLayoutContent {
				continue
			}
			if algorithmID == "" {
				return fmt.Errorf("default layout algorithm %q is not registered", DefaultAlgorithmID)
			}
			return fmt.Errorf("layout algorithm %q not found for %s", algorithmID, nodePath(model, nodeID))
		}
		graph.Set(&node.Properties, ResolvedAlgorithm, algorithm)
	}
	return nil
}

func (engine *Engine) layoutRecursively(model *graph.Model, parent graph.NodeID, monitor ProgressMonitor) ([]graph.EdgeID, error) {
	node := model.Node(parent)
	insideSelfLoops := gatherInsideSelfLoops(model, parent)
	if graph.Get(&node.Properties, NoLayout) || len(node.Children) == 0 && len(insideSelfLoops) == 0 {
		return nil, nil
	}
	algorithm, found := graph.TryGet(&node.Properties, ResolvedAlgorithm)
	if !found || algorithm == nil {
		return nil, fmt.Errorf("resolved algorithm is not set for %s", nodePath(model, parent))
	}
	engine.evaluateHierarchyHandling(model, parent)
	supportsInsideSelfLoops := algorithm.Features.Has(FeatureInsideSelfLoops)
	if len(node.Children) == 0 && len(insideSelfLoops) > 0 && !supportsInsideSelfLoops {
		return nil, nil
	}
	childrenInsideSelfLoops := make([]graph.EdgeID, 0)
	includeChildren := graph.Get(&node.Properties, HierarchyHandling) == "INCLUDE_CHILDREN" &&
		(algorithm.Features.Has(FeatureCompound) || algorithm.Features.Has(FeatureClusters))
	if includeChildren {
		queue := append([]graph.NodeID(nil), node.Children...)
		for len(queue) > 0 {
			childID := queue[0]
			queue = queue[1:]
			engine.evaluateHierarchyHandling(model, childID)
			child := model.Node(childID)
			childAlgorithm, hasChildAlgorithm := graph.TryGet(&child.Properties, ResolvedAlgorithm)
			stop := graph.Get(&child.Properties, HierarchyHandling) == "SEPARATE_CHILDREN"
			switches := graph.Has(&child.Properties, Algorithm) && (!hasChildAlgorithm || childAlgorithm.ID != algorithm.ID)
			if stop || switches {
				childLoops, err := engine.layoutRecursively(
					model, childID, monitor.Subtask(float64(len(child.Children))))
				if err != nil {
					return nil, err
				}
				childrenInsideSelfLoops = append(childrenInsideSelfLoops, childLoops...)
				graph.Set(&child.Properties, HierarchyHandling, "SEPARATE_CHILDREN")
				applyScale(model, childID)
			} else {
				queue = append(queue, child.Children...)
			}
		}
	} else {
		children := append([]graph.NodeID(nil), node.Children...)
		for _, childID := range children {
			childLoops, err := engine.layoutRecursively(
				model, childID, monitor.Subtask(float64(len(model.Node(childID).Children))))
			if err != nil {
				return nil, err
			}
			childrenInsideSelfLoops = append(childrenInsideSelfLoops, childLoops...)
			applyScale(model, childID)
		}
	}
	if monitor.Canceled() {
		return nil, nil
	}
	provider := algorithm.Create()
	if provider == nil {
		return nil, fmt.Errorf("layout algorithm %q returned a nil provider", algorithm.ID)
	}
	monitor.Begin(algorithm.ID, float64(len(node.Children)))
	for _, edgeID := range childrenInsideSelfLoops {
		graph.Set(&model.Edge(edgeID).Properties, NoLayout, true)
	}
	err := provider.Layout(model, parent, monitor)
	monitor.Done()
	if err != nil {
		return nil, fmt.Errorf("layout algorithm %s: %w", algorithm.ID, err)
	}
	for _, edgeID := range childrenInsideSelfLoops {
		postProcessInsideSelfLoop(model, edgeID)
	}
	if supportsInsideSelfLoops {
		return insideSelfLoops, nil
	}
	return nil, nil
}

func gatherInsideSelfLoops(model *graph.Model, nodeID graph.NodeID) []graph.EdgeID {
	node := model.Node(nodeID)
	if !graph.Get(&node.Properties, InsideSelfLoopsActivate) {
		return nil
	}
	result := make([]graph.EdgeID, 0)
	seen := make(map[graph.EdgeID]bool)
	consider := func(edgeID graph.EdgeID) {
		if seen[edgeID] {
			return
		}
		seen[edgeID] = true
		if graph.Get(&model.Edge(edgeID).Properties, InsideSelfLoopsEdge) &&
			IsSelfLoop(model, edgeID) && insideSelfLoopNode(model, edgeID) == nodeID {
			result = append(result, edgeID)
		}
	}
	for _, edgeID := range node.OutgoingEdges {
		consider(edgeID)
	}
	for _, portID := range node.Ports {
		for _, edgeID := range model.Port(portID).OutgoingEdges {
			consider(edgeID)
		}
	}
	return result
}

func insideSelfLoopNode(model *graph.Model, edgeID graph.EdgeID) graph.NodeID {
	edge := model.Edge(edgeID)
	if len(edge.Sources) > 0 {
		return model.ShapeNode(edge.Sources[0])
	}
	return 0
}

func postProcessInsideSelfLoop(model *graph.Model, edgeID graph.EdgeID) {
	edge := model.Edge(edgeID)
	if len(edge.Sources) == 0 || len(edge.Sections) == 0 {
		return
	}
	offset := model.Node(model.ShapeNode(edge.Sources[0])).Shape.Position()
	section := model.Section(edge.Sections[0])
	section.Start = section.Start.Add(offset)
	section.End = section.End.Add(offset)
	section.BendPoints.Offset(offset)
	if junctions, found := graph.TryGet(&edge.Properties, JunctionPoints); found {
		junctions.Offset(offset)
		graph.Set(&edge.Properties, JunctionPoints, junctions)
	}
}

func (engine *Engine) evaluateHierarchyHandling(model *graph.Model, nodeID graph.NodeID) {
	node := model.Node(nodeID)
	if graph.Get(&node.Properties, HierarchyHandling) != "INHERIT" {
		return
	}
	value := "SEPARATE_CHILDREN"
	if node.Parent != nil {
		value = graph.Get(&model.Node(*node.Parent).Properties, HierarchyHandling)
	}
	graph.Set(&node.Properties, HierarchyHandling, value)
}

func applyScale(model *graph.Model, nodeID graph.NodeID) {
	scale := graph.Get(&model.Node(nodeID).Properties, ScaleFactor)
	if scale == 1 {
		return
	}
	node := model.Node(nodeID)
	for _, childID := range node.Children {
		child := model.Node(childID)
		child.Shape.X *= scale
		child.Shape.Y *= scale
		child.Shape.Width *= scale
		child.Shape.Height *= scale
		for _, portID := range child.Ports {
			port := model.Port(portID)
			port.Shape.X *= scale
			port.Shape.Y *= scale
			port.Shape.Width *= scale
			port.Shape.Height *= scale
		}
		for _, labelID := range child.Labels {
			label := model.Label(labelID)
			label.Shape.X *= scale
			label.Shape.Y *= scale
			label.Shape.Width *= scale
			label.Shape.Height *= scale
		}
	}
	for _, edgeID := range node.ContainedEdges {
		for _, sectionID := range model.Edge(edgeID).Sections {
			section := model.Section(sectionID)
			section.Start = section.Start.Scale(scale)
			section.End = section.End.Scale(scale)
			section.BendPoints.Scale(scale)
		}
		if points, found := graph.TryGet(&model.Edge(edgeID).Properties, JunctionPoints); found {
			points.Scale(scale)
			graph.Set(&model.Edge(edgeID).Properties, JunctionPoints, points)
		}
	}
}

// TranslateSection moves every point in a section by offset.
func TranslateSection(model *graph.Model, sectionID graph.SectionID, offset geometry.Point) {
	section := model.Section(sectionID)
	section.Start = section.Start.Add(offset)
	section.End = section.End.Add(offset)
	section.BendPoints.Offset(offset)
}

func IsSelfLoop(model *graph.Model, edgeID graph.EdgeID) bool {
	edge := model.Edge(edgeID)
	if len(edge.Sources)+len(edge.Targets) == 0 {
		return false
	}
	first := graph.NodeID(0)
	initialized := false
	for _, shape := range append(append([]graph.ShapeID(nil), edge.Sources...), edge.Targets...) {
		node := model.ShapeNode(shape)
		if !initialized {
			first, initialized = node, true
		} else if node != first {
			return false
		}
	}
	return true
}

func nodePath(model *graph.Model, nodeID graph.NodeID) string {
	parts := make([]string, 0)
	for current := &nodeID; current != nil; current = model.Node(*current).Parent {
		node := model.Node(*current)
		value := node.Identifier
		if !node.IdentifierPresent {
			value = fmt.Sprintf("#%d", *current)
		}
		parts = append(parts, value)
	}
	for left, right := 0, len(parts)-1; left < right; left, right = left+1, right-1 {
		parts[left], parts[right] = parts[right], parts[left]
	}
	result := ""
	for index, part := range parts {
		if index > 0 {
			result += " > "
		}
		result += part
	}
	return result
}
