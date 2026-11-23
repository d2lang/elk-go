// Copyright (c) 2016, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package stress

import (
	"github.com/d2lang/elk-go/internal/common/nodespacing"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/force"
	"github.com/d2lang/elk-go/internal/graph"
)

type Provider struct{}

func Register(registry *core.AlgorithmRegistry) error {
	return registry.Register(core.AlgorithmData{
		ID:     "org.eclipse.elk.stress",
		Create: func() core.LayoutProvider { return &Provider{} },
	})
}

func (*Provider) Layout(model *graph.Model, parent graph.NodeID, monitor core.ProgressMonitor) error {
	if !graph.Get(&model.Node(parent).Properties, force.Interactive) {
		if err := force.InitialLayout(model, parent, monitor.Subtask(1)); err != nil {
			return err
		}
	} else if !graph.Get(&model.Node(parent).Properties, force.OmitNodeMicroLayout) {
		nodespacing.CalculateLabelAndNodeSizes(model, parent, nil)
	}

	importer := force.NewImporter(model, parent)
	fgraph, err := importer.ImportGraph()
	if err != nil {
		return err
	}
	components := force.Split(fgraph)
	for _, component := range components {
		if component.ArrangeSelfLoops() {
			if err := fitPinnedLoops(model, component); err != nil {
				return err
			}
		}
		if len(component.Nodes) <= 1 {
			continue
		}
		majorization := &Majorization{}
		majorization.Initialize(model, component)
		majorization.Execute()
		component.SeparateSelfLoopFootprints(func(node *force.FNode) bool {
			return !graph.Get(&model.Node(node.Origin).Properties, Fixed)
		}, majorization.dimension != DimensionY, majorization.dimension != DimensionX)
		for _, label := range component.Labels {
			label.RefreshPosition()
		}
	}
	return importer.ApplyLayout(force.Recombine(components))
}
