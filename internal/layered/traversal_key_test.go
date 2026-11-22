// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package layered

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	basegraph "github.com/d2lang/elk-go/internal/graph"
)

// traversalChain is a random port-dummy chain into nested graphs
// (as TestTraversalKeyLoopMatchesRecursion builds), and the
// port it starts from.
func traversalChain(random *rand.Rand) (*LGraphArena, LGraphID, LPortID) {
	arena := NewLGraphArena()
	graphID := arena.AddGraph()
	addNodes := func(graphID LGraphID) []LNodeID {
		var nodes []LNodeID
		for count := 1 + random.Intn(3); count > 0; count-- {
			nodeID := arena.AddNode(graphID)
			arena.Graph(graphID).LayerlessNodes = append(arena.Graph(graphID).LayerlessNodes, nodeID)
			for ports := random.Intn(3); ports > 0; ports-- {
				portID := arena.AddPort()
				arena.SetPortNode(portID, &nodeID)
			}
			nodes = append(nodes, nodeID)
		}
		return nodes
	}
	nodes := addNodes(graphID)
	start := arena.AddPort()
	first := nodes[random.Intn(len(nodes))]
	arena.SetPortNode(start, &first)
	port, node := start, first
	for depth := random.Intn(9); depth > 0; depth-- {
		nested := arena.AddGraph()
		arena.Graph(nested).ParentNode = &node
		arena.Node(node).NestedGraph = &nested
		inner := addNodes(nested)
		if random.Intn(8) == 0 {
			break
		}
		dummy := arena.AddNode(nested)
		basegraph.Set(&arena.Port(port).Properties, PortDummy, dummy)
		dummyPort := arena.AddPort()
		arena.SetPortNode(dummyPort, &dummy)
		if random.Intn(8) == 0 {
			break
		}
		next := arena.AddPort()
		target := inner[random.Intn(len(inner))]
		if random.Intn(10) != 0 {
			arena.SetPortNode(next, &target)
		}
		edgeID := arena.AddEdge()
		if random.Intn(2) == 0 {
			arena.SetEdgeSource(edgeID, &dummyPort)
			arena.SetEdgeTarget(edgeID, &next)
		} else {
			arena.SetEdgeSource(edgeID, &next)
			arena.SetEdgeTarget(edgeID, &dummyPort)
		}
		if arena.Port(next).Node == nil {
			break
		}
		port, node = next, target
	}
	return arena, graphID, start
}

// TestBoundedTraversalKeyMatchesRecursion lowers the loop's bound
// so that compoundInsideTraversalKey switches to ELK's recursion part-way,
// and compares it with the recursive original (path, nil-ness, port index).
func TestBoundedTraversalKeyMatchesRecursion(t *testing.T) {
	saved := maxTraversalKeyLoop
	defer func() { maxTraversalKeyLoop = saved }()
	random := rand.New(rand.NewSource(3))
	for _, bound := range []int{0, 1, 2, 3, 5} {
		maxTraversalKeyLoop = bound
		for round := 0; round < 1500; round++ {
			arena, graphID, start := traversalChain(random)
			wantPath, wantIndex := recursiveCompoundInsideTraversalKey(arena, graphID, start)
			gotPath, gotIndex := compoundInsideTraversalKey(arena, graphID, start)
			if fmt.Sprint(wantPath, wantIndex, wantPath == nil) != fmt.Sprint(gotPath, gotIndex, gotPath == nil) {
				t.Fatalf("bound %d, round %d: recursion %v %d, bounded loop %v %d", bound, round, wantPath, wantIndex, gotPath, gotIndex)
			}
		}
	}
}

// TestCyclicTraversalKeyOverflowsLikeOriginal: a port whose dummy
// leads back to the port itself makes the original recurse until the
// goroutine stack overflows (a fatal error); the loop must end the same way
// instead of growing its path until memory runs out. Run in a child process
// with a 16 MiB stack limit and a watchdog that stops it once its heap passes
// 512 MiB.
func TestCyclicTraversalKeyOverflowsLikeOriginal(t *testing.T) {
	if os.Getenv("ELK_TEST_KEYCYCLE_CHILD") != "" {
		debug.SetMaxStack(16 << 20)
		go func() {
			var stats runtime.MemStats
			for {
				time.Sleep(20 * time.Millisecond)
				runtime.ReadMemStats(&stats)
				if stats.HeapAlloc > 512<<20 {
					fmt.Println("CHILD HEAP LIMIT", stats.HeapAlloc>>20, "MiB")
					os.Exit(3)
				}
			}
		}()
		arena := NewLGraphArena()
		outer := arena.AddGraph()
		compound := arena.AddNode(outer)
		arena.Graph(outer).LayerlessNodes = []LNodeID{compound}
		nested := arena.AddGraph()
		arena.Graph(nested).ParentNode = &compound
		arena.Node(compound).NestedGraph = &nested
		port := arena.AddPort()
		arena.SetPortNode(port, &compound)
		dummy := arena.AddNode(nested)
		arena.Graph(nested).LayerlessNodes = []LNodeID{dummy}
		basegraph.Set(&arena.Port(port).Properties, PortDummy, dummy)
		dummyPort := arena.AddPort()
		arena.SetPortNode(dummyPort, &dummy)
		edgeID := arena.AddEdge()
		arena.SetEdgeSource(edgeID, &dummyPort)
		arena.SetEdgeTarget(edgeID, &port) // the chain leads back to port
		done := make(chan int)
		go func() {
			path, _ := compoundInsideTraversalKey(arena, outer, port)
			done <- len(path)
		}()
		fmt.Println("CHILD RETURNED", <-done)
		return
	}
	if runtime.GOOS == "js" || runtime.GOOS == "wasip1" || runtime.Compiler != "gc" {
		t.Skip("needs child processes and gc's stack limit (debug.SetMaxStack)")
	}
	command := exec.Command(os.Args[0], "-test.run=^TestCyclicTraversalKeyOverflowsLikeOriginal$", "-test.count=1")
	command.Env = append(os.Environ(), "ELK_TEST_KEYCYCLE_CHILD=1")
	output, err := command.CombinedOutput()
	text := string(output)
	if err == nil || !strings.Contains(text, "goroutine stack exceeds") || strings.Contains(text, "CHILD HEAP LIMIT") {
		if len(text) > 1500 {
			text = text[:1500]
		}
		t.Fatalf("cyclic dummy chain: %v (want a fatal stack overflow)\n%s", err, text)
	}
}
