// Copyright (c) 2017 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package layered

import (
	"testing"

	"github.com/d2lang/elk-go/internal/common/nodespacing"
	"github.com/d2lang/elk-go/internal/core"
	basegraph "github.com/d2lang/elk-go/internal/graph"
)

func TestNetworkSimplexFlexibilityOptionsNormalizeAndRegister(t *testing.T) {
	var source, target basegraph.PropertyMap
	source.SetRaw(NetworkSimplexNodeFlexibility.ID, "NODE_SIZE_WHERE_SPACE_PERMITS")
	source.SetRaw(NetworkSimplexNodeFlexibilityDefault.ID, "PORT_POSITION")
	copyTypedProperties(&target, &source)
	if got := basegraph.Get(&target, NetworkSimplexNodeFlexibility); got != NodeFlexibilitySizeWhereSpacePermits {
		t.Fatalf("node flexibility = %v, want NODE_SIZE_WHERE_SPACE_PERMITS", got)
	}
	if got := basegraph.Get(&target, NetworkSimplexNodeFlexibilityDefault); got != NodeFlexibilityPortPosition {
		t.Fatalf("default flexibility = %v, want PORT_POSITION", got)
	}

	registry := core.NewOptionRegistry()
	RegisterOptions(registry)
	nodeOption := registry.ByID(NetworkSimplexNodeFlexibility.ID)
	if nodeOption == nil || nodeOption.Kind != core.EnumOption || nodeOption.Targets != core.Nodes {
		t.Fatalf("node-flexibility metadata = %#v", nodeOption)
	}
	defaultOption := registry.ByID(NetworkSimplexNodeFlexibilityDefault.ID)
	if defaultOption == nil || defaultOption.Kind != core.EnumOption || defaultOption.Targets != core.Parents {
		t.Fatalf("default-flexibility metadata = %#v", defaultOption)
	}
	labelPlacement := registry.ByID(nodespacing.NodeLabelsPlacement.ID)
	if labelPlacement == nil || labelPlacement.Kind != core.EnumSetOption ||
		labelPlacement.Targets != core.Nodes|core.Labels {
		t.Fatalf("node-label-placement metadata = %#v", labelPlacement)
	}
}
