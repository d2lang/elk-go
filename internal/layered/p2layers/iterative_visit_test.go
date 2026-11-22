// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package p2layers

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/d2lang/elk-go/internal/layered"
)

// recursiveVisitComponent is visitComponent as it was before it was made
// iterative (its frames made long chains overflow a 1 MiB
// stack); the iterative one must do exactly what it does.
func recursiveVisitComponent(
	arena *layered.LGraphArena,
	visited []bool,
	component []layered.LNodeID,
	nodeID layered.LNodeID,
) []layered.LNodeID {
	index := arena.Node(nodeID).ID
	if visited[index] {
		return component
	}
	visited[index] = true
	component = append(component, nodeID)
	for _, portID := range arena.Node(nodeID).Ports {
		for _, edges := range arena.PortEdgeLists(portID) {
			for _, edgeID := range edges {
				opposite := arena.EdgeOtherNode(edgeID, nodeID)
				if !visited[arena.Node(opposite).ID] {
					component = recursiveVisitComponent(arena, visited, component, opposite)
				}
			}
		}
	}
	return component
}

// randomComponentArena is a graph of random nodes with random ports and
// edges between them (self-loops and parallel edges included). With outside
// set, some edges lead to nodes that are not in the returned node list
// (their scratch IDs may point anywhere, as in a malformed call).
func randomComponentArena(random *rand.Rand, outside bool) (*layered.LGraphArena, []layered.LNodeID) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	count := 1 + random.Intn(10)
	var nodes, all []layered.LNodeID
	var ports []layered.LPortID
	for index := 0; index < count+2; index++ {
		nodeID := arena.AddNode(graphID)
		all = append(all, nodeID)
		if index < count || !outside {
			nodes = append(nodes, nodeID)
		}
		for portCount := random.Intn(4); portCount > 0; portCount-- {
			portID := arena.AddPort()
			arena.SetPortNode(portID, &nodeID)
			ports = append(ports, portID)
		}
	}
	if len(ports) > 0 {
		for edges := random.Intn(3 * count); edges > 0; edges-- {
			source, target := ports[random.Intn(len(ports))], ports[random.Intn(len(ports))]
			edgeID := arena.AddEdge()
			arena.SetEdgeSource(edgeID, &source)
			arena.SetEdgeTarget(edgeID, &target)
		}
	}
	for _, nodeID := range all {
		arena.Node(nodeID).ID = random.Intn(count + 3)
	}
	return arena, nodes
}

func visitRecovering(visit func() []layered.LNodeID) (component []layered.LNodeID, panicked any) {
	defer func() { panicked = recover() }()
	return visit(), nil
}

// TestIterativeVisitMatchesRecursion compares the iterative
// visitComponent with the recursive original on random graphs from every
// node and with random visited marks: the component (order included), the
// marks, and panics (out-of-range scratch IDs of outside nodes).
func TestIterativeVisitMatchesRecursion(t *testing.T) {
	random := rand.New(rand.NewSource(1))
	var frames []visitFrame
	for round := 0; round < 4000; round++ {
		arena, nodes := randomComponentArena(random, round%4 == 3)
		if round%4 != 3 {
			for index, nodeID := range nodes {
				arena.Node(nodeID).ID = index
			}
		}
		for _, start := range nodes {
			marks := make([]bool, len(nodes))
			for index := range marks {
				marks[index] = random.Intn(5) == 0
			}
			wantMarks := append([]bool(nil), marks...)
			gotMarks := append([]bool(nil), marks...)
			prefix := []layered.LNodeID{7}
			want, wantPanic := visitRecovering(func() []layered.LNodeID {
				return recursiveVisitComponent(arena, wantMarks, append([]layered.LNodeID(nil), prefix...), start)
			})
			got, gotPanic := visitRecovering(func() []layered.LNodeID {
				var component []layered.LNodeID
				component, frames = visitComponent(arena, gotMarks, append([]layered.LNodeID(nil), prefix...), start, frames)
				return component
			})
			if fmt.Sprint(want, wantMarks, wantPanic) != fmt.Sprint(got, gotMarks, gotPanic) {
				t.Fatalf("round %d from %d: recursive %v %v %v, iterative %v %v %v",
					round, start, want, wantMarks, wantPanic, got, gotMarks, gotPanic)
			}
		}
	}
}
