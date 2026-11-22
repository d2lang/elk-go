// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package elk

import (
	"errors"
	"fmt"
	"sync"

	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

// Request is a graph for layout given element by element instead of as a
// Graph, with its layout read back element by element: Layout lays it out
// exactly as Layout(graph) lays out the Graph with the same elements, IDs,
// shapes and options, and the read-back methods return what Layout(graph)
// writes into that Graph's nodes and edges.
//
// Elements are added in the order in which Layout imports a Graph: Root
// first; for every node, its ports (each followed by its labels), then its
// labels, then its children (each with everything below it) before the next
// node that is not below it; after all nodes, the edges, each followed by its
// labels, ordered by the position of their parent node in that node order
// (edges of one parent in their order). Every node and port is given a string
// ID and all four of x, y, width and height (as a Graph whose elements set
// them all); labels are given their text and shape and have no ID; edges are
// given in sources/targets form without sections. An element added out of
// order, or an invalid element (a duplicate ID, a source that does not
// exist, ...), makes Err and Layout report an error and the request
// unusable; the error is the one Layout(graph) reports for the first invalid
// element, except that a node option elkjs cannot instantiate is only
// reported first if every node is added.
//
// A Request is not safe for concurrent use. Release it when done: its
// storage is reused by later requests.
type Request struct {
	codec *core.Codec
	model *graph.Model
	err   error

	// open is the path from the root to the node elements are being added
	// to, phases its stage per node (see the request order above).
	open   []openNode
	rooted bool
	edges  bool
	// lastEdgeParent, lastPort and lastEdge check the order of edges and
	// of port and edge labels.
	lastEdgeParent graph.NodeID
	lastPort       PortRef
	lastEdge       EdgeRef

	laidOut bool
	// laying is set while Layout runs the engine: a request released with it
	// set (Layout panicked) is dropped, not reused.
	laying bool
	// offsetEdge caches EdgeOutputOffset for the edge read last.
	offsetEdge EdgeRef
	offset     geometry.Point
}

type openNode struct {
	id    graph.NodeID
	phase uint8 // 0: ports may follow, 1: labels may follow, 2: children
}

// NodeRef identifies a node of a Request.
type NodeRef int32

// PortRef identifies a port of a Request.
type PortRef int32

// EdgeRef identifies an edge of a Request.
type EdgeRef int32

// Options is a set of layout options prepared for elements of requests:
// resolved and parsed once, as Layout imports them, and stored by every
// element given them without resolving or parsing again.
type Options core.PreparedOptions

func (options *Options) core() *core.PreparedOptions {
	return (*core.PreparedOptions)(options)
}

// PrepareOptions prepares the layout options of an element whose
// LayoutOptions hold values[i] under keys[i], with member order keys (as
// SetLayoutOptionKeys records it). Values are what decoding the options'
// JSON gives (strings, booleans, float64 numbers, ...). keys and values are
// not retained; the result may be used by any number of requests and
// elements, also concurrently.
func PrepareOptions(keys []string, values []any) *Options {
	options, _ := sharedRegistries()
	prepared := core.NewCodec(options).PrepareOptions(keys, values)
	return (*Options)(&prepared)
}

// PrepareOptions is the package function PrepareOptions for elements of this
// request only, using its caches and storage: the result is valid until
// Release.
func (r *Request) PrepareOptions(keys []string, values []any) *Options {
	return (*Options)(r.codec.PrepareRequestOptions(keys, values))
}

// NewRequest returns an empty request with room for the given numbers of
// elements (more only cost growth); the root counts as a node.
func NewRequest(nodes, ports, labels, edges int) *Request {
	options, _ := sharedRegistries()
	codec := core.AcquireCodec(options)
	r := takeRequest()
	*r = Request{codec: codec, model: codec.BeginTyped(nodes, ports, labels, edges), open: r.open[:0],
		lastPort: -1, lastEdge: -1, offsetEdge: -1}
	return r
}

// maxRetainedOpen bounds the node stack a released request keeps.
const maxRetainedOpen = 1 << 10

// Release gives the request's storage back for reuse. Neither the request
// nor options it prepared may be used afterwards. A request whose Layout did
// not return (it panicked) gives nothing back: its state is dropped.
func (r *Request) Release() {
	if r.codec == nil {
		return
	}
	if r.laying {
		*r = Request{}
		return
	}
	core.ReleaseCodec(r.codec, r.model)
	open := r.open[:0]
	if cap(open) > maxRetainedOpen {
		open = nil
	}
	*r = Request{open: open}
	releasedRequests.Lock()
	if releasedRequests.slot == nil {
		releasedRequests.slot = r
		r = nil
	}
	releasedRequests.Unlock()
	if r != nil {
		releasedRequests.pool.Put(r)
	}
}

// releasedRequests keeps released requests (their structs and node stacks)
// for NewRequest: one in a slot that garbage collection does not empty, more
// in a sync.Pool, as core keeps codecs.
var releasedRequests struct {
	sync.Mutex
	slot *Request
	pool sync.Pool
}

func takeRequest() *Request {
	releasedRequests.Lock()
	r := releasedRequests.slot
	releasedRequests.slot = nil
	releasedRequests.Unlock()
	if r == nil {
		r, _ = releasedRequests.pool.Get().(*Request)
	}
	if r == nil {
		r = new(Request)
	}
	return r
}

// Err returns the error that adding an element caused, if any.
func (r *Request) Err() error { return r.err }

var errRequestOrder = errors.New("elk: request element added out of import order")

func (r *Request) fail(err error) {
	if r.err == nil {
		r.err = err
	}
}

// Root imports the root node: its ID and options (it has no shape).
func (r *Request) Root(id string, options *Options) NodeRef {
	if r.err != nil {
		return 0
	}
	if r.rooted {
		r.fail(errRequestOrder)
		return 0
	}
	r.rooted = true
	root := r.model.Root
	r.open = append(r.open[:0], openNode{id: root})
	if err := r.codec.TypedNode(r.model, root, id, nil, options.core()); err != nil {
		r.fail(err)
	}
	return NodeRef(root)
}

// enter makes node the node elements are added to, at least at phase.
func (r *Request) enter(node graph.NodeID, phase uint8) bool {
	if !r.rooted || r.edges {
		return false
	}
	for len(r.open) > 0 && r.open[len(r.open)-1].id != node {
		r.open = r.open[:len(r.open)-1]
	}
	if len(r.open) == 0 {
		return false
	}
	top := &r.open[len(r.open)-1]
	if top.phase > phase {
		return false
	}
	top.phase = phase
	return true
}

// Node imports a child of parent with an ID, a shape and options.
func (r *Request) Node(parent NodeRef, id string, x, y, width, height float64, options *Options) NodeRef {
	if r.err != nil {
		return -1
	}
	if !r.enter(graph.NodeID(parent), 2) {
		r.fail(errRequestOrder)
		return -1
	}
	child := r.codec.TypedChild(r.model, graph.NodeID(parent))
	r.open = append(r.open, openNode{id: child})
	shape := core.TypedShape{X: x, Y: y, Width: width, Height: height}
	if err := r.codec.TypedNode(r.model, child, id, &shape, options.core()); err != nil {
		r.fail(err)
	}
	return NodeRef(child)
}

// Port imports a port of node with an ID, a shape and options.
func (r *Request) Port(node NodeRef, id string, x, y, width, height float64, options *Options) PortRef {
	if r.err != nil {
		return -1
	}
	if !r.enter(graph.NodeID(node), 0) {
		r.fail(errRequestOrder)
		return -1
	}
	shape := core.TypedShape{X: x, Y: y, Width: width, Height: height}
	port, err := r.codec.TypedPort(r.model, graph.NodeID(node), id, &shape, options.core())
	if err != nil {
		r.fail(err)
		return -1
	}
	r.lastPort = PortRef(port)
	return PortRef(port)
}

// PortLabel imports a label of port, the port added last.
func (r *Request) PortLabel(port PortRef, text string, x, y, width, height float64, options *Options) {
	if r.err != nil {
		return
	}
	if port != r.lastPort || !r.enter(r.model.Port(graph.PortID(port)).Parent, 0) {
		r.fail(errRequestOrder)
		return
	}
	shape := core.TypedShape{X: x, Y: y, Width: width, Height: height}
	r.codec.TypedLabel(r.model, graph.PortElementRef(graph.PortID(port)), text, &shape, options.core())
}

// NodeLabel imports a label of node with its text, shape and options.
func (r *Request) NodeLabel(node NodeRef, text string, x, y, width, height float64, options *Options) {
	if r.err != nil {
		return
	}
	if !r.enter(graph.NodeID(node), 1) {
		r.fail(errRequestOrder)
		return
	}
	shape := core.TypedShape{X: x, Y: y, Width: width, Height: height}
	r.codec.TypedLabel(r.model, graph.NodeElementRef(graph.NodeID(node)), text, &shape, options.core())
}

// Edge imports an edge listed by parent with an ID, the IDs of its sources
// and targets (nodes or ports), and options.
func (r *Request) Edge(parent NodeRef, id string, sources, targets []string, options *Options) EdgeRef {
	if r.err != nil {
		return -1
	}
	if !r.rooted || parent < 0 || int(parent) >= len(r.model.Nodes) || (r.edges && graph.NodeID(parent) < r.lastEdgeParent) {
		r.fail(errRequestOrder)
		return -1
	}
	r.edges, r.lastEdgeParent = true, graph.NodeID(parent)
	edge, err := r.codec.TypedEdge(r.model, graph.NodeID(parent), id, sources, targets, options.core())
	if err != nil {
		r.fail(err)
		return -1
	}
	r.lastEdge = EdgeRef(edge)
	return EdgeRef(edge)
}

// EdgeLabel imports a label of edge, the edge added last.
func (r *Request) EdgeLabel(edge EdgeRef, text string, x, y, width, height float64, options *Options) {
	if r.err != nil {
		return
	}
	if edge != r.lastEdge {
		r.fail(errRequestOrder)
		return
	}
	shape := core.TypedShape{X: x, Y: y, Width: width, Height: height}
	r.codec.TypedLabel(r.model, graph.EdgeElementRef(graph.EdgeID(edge)), text, &shape, options.core())
}

// Layout lays the request out, reporting the errors Layout(graph) reports.
func (r *Request) Layout() error {
	if r.err == nil && !r.rooted {
		r.err = fmt.Errorf("every node must have an id")
	}
	if r.err != nil {
		return fmt.Errorf("import ELK graph: %w", r.err)
	}
	r.laying = true
	if err := layoutModel(r.model); err != nil {
		r.laying = false
		return fmt.Errorf("layout ELK graph: %w", err)
	}
	err := r.codec.TypedCoordinates(r.model)
	r.laying = false
	if err != nil {
		return fmt.Errorf("export ELK layout: %w", err)
	}
	r.laidOut = true
	return nil
}

// NodeLayout returns the x, y, width and height Layout(graph) writes into
// the node.
func (r *Request) NodeLayout(node NodeRef) (x, y, width, height float64) {
	return r.codec.NodeOutput(r.model, int(node))
}

// Sections returns the number of sections Layout(graph) writes into the
// edge (it leaves Sections nil when there are none).
func (r *Request) Sections(edge EdgeRef) int {
	return len(r.model.Edge(graph.EdgeID(edge)).Sections)
}

func (r *Request) edgeOffset(edge EdgeRef) geometry.Point {
	if edge != r.offsetEdge {
		r.offsetEdge, r.offset = edge, r.codec.EdgeOutputOffset(r.model, graph.EdgeID(edge))
	}
	return r.offset
}

// Section returns the start and end points Layout(graph) writes into section
// index of the edge, and its number of bend points (it leaves BendPoints nil
// when there are none).
func (r *Request) Section(edge EdgeRef, index int) (start, end Point, bends int) {
	offset := r.edgeOffset(edge)
	section := r.model.Section(r.model.Edge(graph.EdgeID(edge)).Sections[index])
	start, end = toPoint(section.Start.Add(offset)), toPoint(section.End.Add(offset))
	return start, end, len(section.BendPoints)
}

// Bend returns bend point index of section of the edge, as Layout(graph)
// writes it.
func (r *Request) Bend(edge EdgeRef, section, index int) Point {
	offset := r.edgeOffset(edge)
	value := r.model.Section(r.model.Edge(graph.EdgeID(edge)).Sections[section])
	return toPoint(value.BendPoints[index].Add(offset))
}

// Container returns the node whose ID Layout(graph) writes as the edge's
// container, and false when it writes none (null) or leaves the edge's
// Container as it was.
func (r *Request) Container(edge EdgeRef) (NodeRef, bool) {
	node, ok := r.codec.EdgeOutputContainer(r.model, graph.EdgeID(edge))
	return NodeRef(node), ok
}

func toPoint(point geometry.Point) Point {
	return Point{X: point.X, Y: point.Y}
}
