// Copyright (c) 2012, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import (
	"math"
	"sort"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
	"github.com/d2lang/elk-go/internal/reuse"
)

// BrandesKoepfPlacer adapts the default node placer to the layered pipeline.
type BrandesKoepfPlacer struct{}

func (BrandesKoepfPlacer) Process(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	PlaceBrandesKoepf(arena, graphID)
	return nil
}

func (BrandesKoepfPlacer) ProcessorConfiguration(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) layered.ProcessorConfiguration {
	var configuration layered.ProcessorConfiguration
	properties := basegraph.Get(&arena.Graph(graphID).Properties, layered.GraphProperties)
	if properties&layered.GraphPropertyExternalPorts != 0 {
		configuration.AddBefore(layered.PhaseEdgeRouting, layered.HierarchicalPortPositionProcessor)
	}
	return configuration
}

// bkWorkspace is the storage of one PlaceBrandesKoepf call, reused by later
// calls. Every structure taken from it starts in the state a fresh
// allocation has (zeroed layout arrays, empty maps), and nothing taken from
// it is referenced after the call returns.
type bkWorkspace struct {
	nodes   []optNode
	floats  []optFloat
	flags   []bool
	layouts [5]BKAlignedLayout
	used    int
	info    NeighborhoodInformation
	marked  edgeMarks
	compact BKCompactor
	order   []*BKAlignedLayout
	aligner BKAligner
	dense   bkDense
}

// edgeMarks is the set of conflict-marked edges: a flag per arena edge and
// the list of marked edges, which clear walks to reset the flags.
type edgeMarks struct {
	flags []bool
	list  []layered.LEdgeID
}

func (marks *edgeMarks) mark(edgeID layered.LEdgeID) {
	if int(edgeID) >= len(marks.flags) {
		if int(edgeID) < cap(marks.flags) {
			marks.flags = marks.flags[:int(edgeID)+1]
		} else {
			grown := make([]bool, int(edgeID)+1, 2*int(edgeID)+2)
			copy(grown, marks.flags)
			marks.flags = grown
		}
	}
	if !marks.flags[edgeID] {
		marks.flags[edgeID] = true
		marks.list = append(marks.list, edgeID)
	}
}

func (marks *edgeMarks) has(edgeID layered.LEdgeID) bool {
	return int(edgeID) < len(marks.flags) && marks.flags[edgeID]
}

func (marks *edgeMarks) clear() {
	for _, edgeID := range marks.list {
		marks.flags[edgeID] = false
	}
	marks.list = marks.list[:0]
}

var bkWorkspaces reuse.Pool[bkWorkspace]

// maxSlotBKBytes bounds the storage (see retainedBytes) of a workspace that
// goes to bkWorkspaces' slot (it may stay alive indefinitely); a larger one
// goes to the sync.Pool only under the gc toolchain, which garbage collection
// empties, and is dropped under TinyGo (see reuse.Pool and
// reuse.OversizedToPool).
const maxSlotBKBytes = 1 << 20

// retainedBytes bounds the bytes of storage workspace keeps on a 64-bit
// platform: every buffer it keeps for its capacity (the layouts' and
// neighbour lists are cut from these). Element sizes are checked by a test.
func (workspace *bkWorkspace) retainedBytes() int {
	bytes := cap(workspace.nodes)*8 + cap(workspace.floats)*16 + cap(workspace.flags)
	info := &workspace.info
	bytes += (cap(info.LayerIndex)+cap(info.NodeIndex))*8 +
		(cap(info.LeftNeighbors)+cap(info.RightNeighbors))*24 + cap(info.neighborBacking)*8
	bytes += cap(workspace.marked.flags) + cap(workspace.marked.list)*4
	compact := &workspace.compact
	bytes += (cap(compact.sinkClass)+cap(compact.sinkScratch)+cap(compact.sinkOrder))*4 +
		cap(compact.classNodes)*56 + cap(compact.queue)*8
	for _, class := range compact.classNodes[:cap(compact.classNodes)] {
		bytes += cap(class.outgoing) * 16 // also those of classes past the length, kept until reused
	}
	if threshold := compact.threshold; threshold != nil {
		bytes += cap(threshold.blockFinished) + (cap(threshold.postprocessQueue)+cap(threshold.postprocessStack))*16
	}
	bytes += cap(workspace.order)*8 + cap(workspace.aligner.roots)*4 + cap(workspace.aligner.seen)
	dense := &workspace.dense
	bytes += (cap(dense.node)+cap(dense.layer)+cap(dense.start))*4 +
		(cap(dense.sizeY)+cap(dense.marginTop)+cap(dense.marginBottom))*8 +
		cap(dense.longEdge) + cap(dense.nodeType) + cap(dense.individual) +
		(cap(dense.left)+cap(dense.right)+cap(dense.ends))*24 +
		cap(dense.neighborBacking)*8 + cap(dense.endBacking)*12 + cap(dense.order)*8 +
		cap(dense.ints)*4 + cap(dense.floats)*8 + cap(dense.bools) + cap(dense.shifts)*16 +
		cap(dense.roots)*4 + cap(dense.seen) + cap(dense.blockFinished) +
		(cap(dense.postprocessQueue)+cap(dense.postprocessStack))*16
	return bytes
}

func acquireBKWorkspace() *bkWorkspace {
	if workspace := bkWorkspaces.Get(); workspace != nil {
		return workspace
	}
	return &bkWorkspace{compact: BKCompactor{threshold: &ThresholdStrategy{}}}
}

func releaseBKWorkspace(workspace *bkWorkspace) {
	for index := range workspace.layouts {
		workspace.layouts[index] = BKAlignedLayout{}
	}
	for index := range workspace.info.LeftNeighbors {
		workspace.info.LeftNeighbors[index] = nil
	}
	for index := range workspace.info.RightNeighbors {
		workspace.info.RightNeighbors[index] = nil
	}
	workspace.info = NeighborhoodInformation{
		LayerIndex: workspace.info.LayerIndex[:0], NodeIndex: workspace.info.NodeIndex[:0],
		LeftNeighbors: workspace.info.LeftNeighbors[:0], RightNeighbors: workspace.info.RightNeighbors[:0],
		neighborBacking: workspace.info.neighborBacking[:0],
	}
	for index := range workspace.order {
		workspace.order[index] = nil
	}
	workspace.order = workspace.order[:0]
	workspace.marked.clear()
	workspace.aligner.Arena, workspace.aligner.Info = nil, nil
	workspace.compact.clearClasses()
	workspace.compact.spacings = layered.SpacingCache{}
	bkWorkspaces.Put(workspace, workspace.retainedBytes() <= maxSlotBKBytes)
}

// reserve prepares zeroed storage for count layouts of nodeCount nodes.
func (workspace *bkWorkspace) reserve(nodeCount, count int) {
	workspace.used = 0
	nodes, floats, flags := 3*nodeCount*count, 4*nodeCount*count, 2*nodeCount*count
	if cap(workspace.nodes) < nodes {
		workspace.nodes = make([]optNode, nodes)
	} else {
		workspace.nodes = workspace.nodes[:nodes]
		for index := range workspace.nodes {
			workspace.nodes[index] = optNode{}
		}
	}
	if cap(workspace.floats) < floats {
		workspace.floats = make([]optFloat, floats)
	} else {
		workspace.floats = workspace.floats[:floats]
		for index := range workspace.floats {
			workspace.floats[index] = optFloat{}
		}
	}
	if cap(workspace.flags) < flags {
		workspace.flags = make([]bool, flags)
	} else {
		workspace.flags = workspace.flags[:flags]
		for index := range workspace.flags {
			workspace.flags[index] = false
		}
	}
}

// newLayout is NewBKAlignedLayout over the workspace's reserved storage.
func (workspace *bkWorkspace) newLayout(
	graphID layered.LGraphID,
	nodeCount int,
	vertical VerticalDirection,
	horizontal HorizontalDirection,
) *BKAlignedLayout {
	slot := workspace.used
	workspace.used++
	layout := &workspace.layouts[slot]
	layout.init(graphID, nodeCount, vertical, horizontal,
		workspace.nodes[3*nodeCount*slot:3*nodeCount*(slot+1):3*nodeCount*(slot+1)],
		workspace.floats[4*nodeCount*slot:4*nodeCount*(slot+1):4*nodeCount*(slot+1)],
		workspace.flags[2*nodeCount*slot:2*nodeCount*(slot+1):2*nodeCount*(slot+1)])
	return layout
}

// PlaceBrandesKoepf runs ELK 0.8.1's extended Brandes-Koepf node placement.
func PlaceBrandesKoepf(arena *layered.LGraphArena, graphID layered.LGraphID) {
	if !bkDenseDisabled && !bkSingleNodeDisabled && placeSingleNode(arena, graphID) {
		return
	}
	workspace := acquireBKWorkspace()
	placeBrandesKoepfIn(workspace, arena, graphID)
	// The workspace goes back to the pool only from a placement that
	// returned: one that panics may leave its storage half-updated, so it is
	// dropped with the call instead.
	releaseBKWorkspace(workspace)
}

// placeBrandesKoepfIn is PlaceBrandesKoepf with its workspace acquired.
func placeBrandesKoepfIn(workspace *bkWorkspace, arena *layered.LGraphArena, graphID layered.LGraphID) {
	info := &workspace.info
	buildNeighborhoodInformation(arena, graphID, info)
	if !bkDenseDisabled && workspace.dense.build(arena, graphID, info) {
		workspace.dense.place(arena, graphID, workspace)
		return
	}
	alignment := basegraph.Get(&arena.Graph(graphID).Properties, layered.BKFixedAlignment)
	favorStraightEdges := basegraph.Get(&arena.Graph(graphID).Properties, layered.FavorStraightEdges)
	produceBalanced := alignment == layered.FixedAlignmentNone && !favorStraightEdges ||
		alignment == layered.FixedAlignmentBalanced
	markedEdges := &workspace.marked
	markedEdges.clear()
	markConflicts(arena, graphID, info, markedEdges)
	layouts := createDirectionalLayoutsIn(workspace, graphID, info.NodeCount, alignment)
	aligner := &workspace.aligner
	aligner.Arena, aligner.Graph, aligner.Info = arena, graphID, info
	for _, layout := range layouts {
		aligner.verticalAlignment(layout, markedEdges)
		aligner.InsideBlockShift(layout)
	}
	compactor := &workspace.compact
	compactor.reset(arena, graphID)
	for _, layout := range layouts {
		compactor.HorizontalCompaction(arena, graphID, info, layout)
	}
	var chosen *BKAlignedLayout
	if produceBalanced {
		balanced := createBalancedLayoutIn(workspace, arena, graphID, layouts, info.NodeCount)
		if checkOrderConstraint(arena, graphID, balanced) {
			chosen = balanced
		}
	}
	if chosen == nil {
		for _, layout := range layouts {
			if !checkOrderConstraint(arena, graphID, layout) {
				continue
			}
			if chosen == nil || chosen.LayoutSize(arena) > layout.LayoutSize(arena) {
				chosen = layout
			}
		}
	}
	if chosen == nil {
		chosen = layouts[0]
	}
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			index := arena.Node(nodeID).ID
			arena.Node(nodeID).Position.Y = requireFloat(chosen.Y[index], "y") +
				requireFloat(chosen.InnerShift[index], "inner shift")
		}
	}
	// ELK's info.cleanup() only drops references; the workspace's storage is
	// kept for the next call instead.
}

// createDirectionalLayoutsIn returns the layouts to compute: the one
// direction a LeftDown, LeftUp, RightDown or RightUp alignment names, else
// all four (down-right, up-right, down-left, up-left). They live in
// workspace storage, reserved for the directional layouts plus the balanced
// one.
func createDirectionalLayoutsIn(
	workspace *bkWorkspace,
	graphID layered.LGraphID,
	nodeCount int,
	alignment layered.FixedAlignment,
) []*BKAlignedLayout {
	directions := [4][2]uint8{
		{uint8(VerticalDown), uint8(HorizontalRight)}, {uint8(VerticalUp), uint8(HorizontalRight)},
		{uint8(VerticalDown), uint8(HorizontalLeft)}, {uint8(VerticalUp), uint8(HorizontalLeft)},
	}
	count := 4
	switch alignment {
	case layered.FixedAlignmentLeftDown:
		directions[0], count = [2]uint8{uint8(VerticalDown), uint8(HorizontalLeft)}, 1
	case layered.FixedAlignmentLeftUp:
		directions[0], count = [2]uint8{uint8(VerticalUp), uint8(HorizontalLeft)}, 1
	case layered.FixedAlignmentRightDown:
		directions[0], count = [2]uint8{uint8(VerticalDown), uint8(HorizontalRight)}, 1
	case layered.FixedAlignmentRightUp:
		directions[0], count = [2]uint8{uint8(VerticalUp), uint8(HorizontalRight)}, 1
	}
	workspace.reserve(nodeCount, count+1)
	layouts := workspace.order[:0]
	for index := 0; index < count; index++ {
		layouts = append(layouts, workspace.newLayout(graphID, nodeCount,
			VerticalDirection(directions[index][0]), HorizontalDirection(directions[index][1])))
	}
	workspace.order = layouts
	return layouts
}

const minimumLayersForConflicts = 3

func markConflicts(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	info *NeighborhoodInformation,
	markedEdges *edgeMarks,
) {
	layers := arena.Graph(graphID).Layers
	if len(layers) < minimumLayersForConflicts {
		return
	}
	layerSizes := make([]int, len(layers))
	for index, layerID := range layers {
		layerSizes[index] = len(arena.Layer(layerID).Nodes)
	}
	for layerIndex := 1; layerIndex < len(layers)-1; layerIndex++ {
		currentNodes := arena.Layer(layers[layerIndex+1]).Nodes
		k0, lowerStart := 0, 0
		for lowerEnd := 0; lowerEnd < layerSizes[layerIndex+1]; lowerEnd++ {
			nodeID := currentNodes[lowerEnd]
			inner := incidentToInnerSegment(arena, info, nodeID, layerIndex+1, layerIndex)
			if lowerEnd != layerSizes[layerIndex+1]-1 && !inner {
				continue
			}
			k1 := layerSizes[layerIndex] - 1
			if inner {
				neighbors := info.LeftNeighbors[arena.Node(nodeID).ID]
				k1 = info.NodeIndex[arena.Node(neighbors[0].Node).ID]
			}
			for lowerStart <= lowerEnd {
				current := currentNodes[lowerStart]
				if !incidentToInnerSegment(arena, info, current, layerIndex+1, layerIndex) {
					for _, neighbor := range info.LeftNeighbors[arena.Node(current).ID] {
						position := info.NodeIndex[arena.Node(neighbor.Node).ID]
						if position < k0 || position > k1 {
							markedEdges.mark(neighbor.Edge)
						}
					}
				}
				lowerStart++
			}
			k0 = k1
		}
	}
}

func incidentToInnerSegment(
	arena *layered.LGraphArena,
	info *NeighborhoodInformation,
	nodeID layered.LNodeID,
	layer1, layer2 int,
) bool {
	if arena.Node(nodeID).Type != layered.LongEdgeNode {
		return false
	}
	for _, edgeID := range arena.NodeIncomingEdges(nodeID) {
		source := arena.EdgeSourceNode(edgeID)
		if arena.Node(source).Type == layered.LongEdgeNode &&
			info.LayerIndex[arena.Layer(*arena.Node(source).Layer).ID] == layer2 &&
			info.LayerIndex[arena.Layer(*arena.Node(nodeID).Layer).ID] == layer1 {
			return true
		}
	}
	return false
}

// createBalancedLayoutIn returns the balanced layout of layouts
// (fillBalancedLayout), made in the workspace's last reserved slot.
func createBalancedLayoutIn(
	workspace *bkWorkspace,
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	layouts []*BKAlignedLayout,
	nodeCount int,
) *BKAlignedLayout {
	return fillBalancedLayout(arena, graphID, layouts,
		workspace.newLayout(graphID, nodeCount, VerticalBalanced, HorizontalBalanced))
}

func fillBalancedLayout(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	layouts []*BKAlignedLayout,
	balanced *BKAlignedLayout,
) *BKAlignedLayout {
	// Up to four directional layouts: the per-layout values live in arrays
	// (cut to len(layouts), as the slices were).
	var widthArray, minimumArray, maximumArray, shiftArray, calculatedArray [4]float64
	if len(layouts) > len(widthArray) {
		panic("p4nodes: more than four directional layouts")
	}
	width := widthArray[:len(layouts)]
	minimum := minimumArray[:len(layouts)]
	maximum := maximumArray[:len(layouts)]
	for index := range minimum {
		minimum[index] = float64(math.MaxInt32) // Java initializes min to Integer.MAX_VALUE.
		maximum[index] = float64(math.MinInt32)
	}
	minimumWidthLayout := 0
	for layoutIndex, layout := range layouts {
		width[layoutIndex] = layout.LayoutSize(arena)
		if width[minimumWidthLayout] > width[layoutIndex] {
			minimumWidthLayout = layoutIndex
		}
		for _, layerID := range arena.Graph(graphID).Layers {
			for _, nodeID := range arena.Layer(layerID).Nodes {
				index := arena.Node(nodeID).ID
				position := requireFloat(layout.Y[index], "y") + requireFloat(layout.InnerShift[index], "inner shift")
				minimum[layoutIndex] = math.Min(minimum[layoutIndex], position)
				maximum[layoutIndex] = math.Max(maximum[layoutIndex], position+arena.Node(nodeID).Size.Y)
			}
		}
	}
	shift := shiftArray[:len(layouts)]
	for index, layout := range layouts {
		if layout.VDirection == VerticalDown {
			shift[index] = minimum[minimumWidthLayout] - minimum[index]
		} else {
			shift[index] = maximum[minimumWidthLayout] - maximum[index]
		}
	}
	calculated := calculatedArray[:len(layouts)]
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			index := arena.Node(nodeID).ID
			for layoutIndex, layout := range layouts {
				calculated[layoutIndex] = requireFloat(layout.Y[index], "y") +
					requireFloat(layout.InnerShift[index], "inner shift") + shift[layoutIndex]
			}
			sort.Float64s(calculated)
			balanced.Y[index] = someFloat((calculated[1] + calculated[2]) / 2)
			balanced.InnerShift[index] = someFloat(0)
		}
	}
	return balanced
}

func checkOrderConstraint(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	layout *BKAlignedLayout,
) bool {
	for _, layerID := range arena.Graph(graphID).Layers {
		position := math.Inf(-1)
		for _, nodeID := range arena.Layer(layerID).Nodes {
			index := arena.Node(nodeID).ID
			top := requireFloat(layout.Y[index], "y") + requireFloat(layout.InnerShift[index], "inner shift") - arena.Node(nodeID).Margin.Top
			bottom := requireFloat(layout.Y[index], "y") + requireFloat(layout.InnerShift[index], "inner shift") +
				arena.Node(nodeID).Size.Y + arena.Node(nodeID).Margin.Bottom
			if top <= position || bottom <= position {
				return false
			}
			position = bottom
		}
	}
	return true
}

// bkSingleNodeDisabled forces the general path for single nodes (for tests).
var bkSingleNodeDisabled bool

// singleNodeBound bounds the size and margins placeSingleNode accepts: far
// from overflow in every sum the general path forms.
const singleNodeBound = 1 << 60

// placeSingleNode is PlaceBrandesKoepf for a graph whose only layer holds a
// single node without ports, whose Layer field names that layer, with a
// finite vertical size and margins below singleNodeBound in magnitude; it
// does nothing and returns false for any other graph.
//
// The general path, for such a graph: numbers the layer and the node 0 (the
// neighborhood information); every directional layout aligns the node with
// itself, gives it inner shift 0 + top margin and y 0 (one block, no
// neighbor, no class, no shift added, nothing to post-process); the
// balanced layout gets c = (0 + (0 + top)) + s with s = m - m = 0 for each
// layout (the layouts are identical, m finite), so y = (c + c) / 2 = c
// exactly and inner shift 0; so every layout the general path may choose
// (balanced, the first directional, or the smallest-size directional one)
// puts the node at 0 + (0 + top), which is the top margin with -0 turned
// into +0. The properties it reads (fixed alignment, favor straight edges,
// edge straightening, individual spacing) have non-materializing defaults,
// so reading them has no effect.
func placeSingleNode(arena *layered.LGraphArena, graphID layered.LGraphID) bool {
	layers := arena.Graph(graphID).Layers
	if len(layers) != 1 {
		return false
	}
	nodes := arena.Layer(layers[0]).Nodes
	if len(nodes) != 1 {
		return false
	}
	node := arena.Node(nodes[0])
	// The general path finds the node's layer through node.Layer (a nil one
	// panics there; another layer has other neighbors): only the layer listing
	// the node is the case described above.
	if node.Layer == nil || *node.Layer != layers[0] ||
		len(node.Ports) != 0 || !withinSingleNodeBound(node.Size.Y) ||
		!withinSingleNodeBound(node.Margin.Top) || !withinSingleNodeBound(node.Margin.Bottom) {
		return false
	}
	arena.Layer(layers[0]).ID = 0
	node.ID = 0
	y := node.Margin.Top
	if y == 0 {
		y = 0 // 0 + (-0) is +0
	}
	node.Position.Y = y
	return true
}

// withinSingleNodeBound reports whether value is finite and below
// singleNodeBound in magnitude (NaN is not).
func withinSingleNodeBound(value float64) bool {
	return value > -singleNodeBound && value < singleNodeBound
}
