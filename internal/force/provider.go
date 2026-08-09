// Copyright (c) 2011, 2020 Kiel University and others.
// Copyright (c) 2015, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package force

import (
	"fmt"

	"github.com/d2lang/elk-go/internal/common/nodespacing"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/graph"
)

type Provider struct{}

func Register(registry *core.AlgorithmRegistry) error {
	return registry.Register(core.AlgorithmData{
		ID:       "org.eclipse.elk.force",
		Features: core.FeatureMultiEdges | core.FeatureEdgeLabels,
		Create:   func() core.LayoutProvider { return &Provider{} },
	})
}

func (*Provider) Layout(model *graph.Model, parent graph.NodeID, monitor core.ProgressMonitor) error {
	if !graph.Get(&model.Node(parent).Properties, OmitNodeMicroLayout) {
		nodespacing.CalculateLabelAndNodeSizes(model, parent, nil)
	}
	importer := NewImporter(model, parent)
	fgraph, err := importer.ImportGraph()
	if err != nil {
		return err
	}
	components := Split(fgraph)
	for _, component := range components {
		var selected forceModel
		switch component.Model {
		case ModelEades:
			selected = &eadesModel{}
		case ModelFruchtermanReingold:
			selected = &fruchtermanReingoldModel{}
		default:
			return fmt.Errorf("unknown force model %q", component.Model)
		}
		selected.layout(component, monitor.Subtask(1/float64(len(components))))
	}
	importer.ApplyLayout(Recombine(components))
	return nil
}

// InitialLayout runs the exact force initialization used by ELK Stress when
// interactive mode is disabled. It is exported only to the sibling stress
// package within this module.
func InitialLayout(model *graph.Model, parent graph.NodeID, monitor core.ProgressMonitor) error {
	return (&Provider{}).Layout(model, parent, monitor)
}
