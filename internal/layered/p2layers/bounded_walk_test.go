// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package p2layers

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/d2lang/elk-go/internal/layered"
)

// chainGraph is a path of n nodes (each with one port), every edge from
// a node to the next, and, when cyclic, an edge from the last back to the
// first; the nodes' x positions make the interactive layerer put them all
// into one initial layer.
func chainGraph(n int, cyclic bool) (*layered.LGraphArena, layered.LGraphID) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	ports := make([]layered.LPortID, n)
	for index := range ports {
		nodeID := arena.AddNode(graphID)
		arena.Graph(graphID).LayerlessNodes = append(arena.Graph(graphID).LayerlessNodes, nodeID)
		arena.Node(nodeID).Size.X = 10
		ports[index] = arena.AddPort()
		arena.SetPortNode(ports[index], &nodeID)
	}
	edge := func(from, to int) {
		edgeID := arena.AddEdge()
		arena.SetEdgeSource(edgeID, &ports[from])
		arena.SetEdgeTarget(edgeID, &ports[to])
	}
	for index := 0; index+1 < n; index++ {
		edge(index, index+1)
	}
	if cyclic {
		edge(n-1, 0)
	}
	return arena, graphID
}

// TestBoundedWalksMatchRecursion lowers the explicit stacks' bound
// (maxIterativeDepth) so that the walks switch to ELK's recursion part-way,
// and compares both layerers with the recursive originals on random DAGs and
// on chains longer than the bound.
func TestBoundedWalksMatchRecursion(t *testing.T) {
	saved := maxIterativeDepth
	defer func() { maxIterativeDepth = saved }()
	for _, bound := range []int{0, 1, 2, 3, 5, 1 << 16} {
		maxIterativeDepth = bound
		check := func(what string, build func() (*layered.LGraphArena, layered.LGraphID)) {
			want, wantGraph := build()
			got, gotGraph := build()
			recursiveLongestPath(want, wantGraph)
			if err := (&LongestPathLayerer{}).Process(got, gotGraph); err != nil {
				t.Fatal(err)
			}
			if describeLayering(want, wantGraph) != describeLayering(got, gotGraph) {
				t.Fatalf("longest path, bound %d, %s:\nrecursive %s\nbounded %s", bound, what,
					describeLayering(want, wantGraph), describeLayering(got, gotGraph))
			}
			want, wantGraph = build()
			got, gotGraph = build()
			if err := (&InteractiveLayerer{}).Process(got, gotGraph); err != nil {
				t.Fatal(err)
			}
			interactiveInitialLayers(want, wantGraph)
			recursiveInteractiveChecks(want, wantGraph)
			if describeLayering(want, wantGraph) != describeLayering(got, gotGraph) {
				t.Fatalf("interactive, bound %d, %s:\nrecursive %s\nbounded %s", bound, what,
					describeLayering(want, wantGraph), describeLayering(got, gotGraph))
			}
		}
		for seed := int64(1); seed <= 600; seed++ {
			check(fmt.Sprint("seed ", seed), func() (*layered.LGraphArena, layered.LGraphID) { return randomDAG(seed) })
		}
		for _, n := range []int{1, 2, 7, 40} {
			check(fmt.Sprint("chain of ", n), func() (*layered.LGraphArena, layered.LGraphID) { return chainGraph(n, false) })
		}
	}
}

// TestCyclicLayeringOverflowsLikeOriginal: on a graph left cyclic,
// both layerers' walks never end. The original recurses until the goroutine
// stack overflows, a fatal error, with little heap; unbounded explicit stacks
// would grow until memory ran out instead (on generated INTERACTIVE and
// LONGEST_PATH graphs they passed 4 GB within two seconds).
// Each layerer runs on a cycle in a child process with a 16 MiB stack limit
// and a watchdog that stops it once its heap passes 512 MiB: it must end in
// the stack overflow.
func TestCyclicLayeringOverflowsLikeOriginal(t *testing.T) {
	if which := os.Getenv("ELK_TEST_CYCLE_CHILD"); which != "" {
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
		arena, graphID := chainGraph(3, true)
		var layerer layered.LayoutProcessor = &LongestPathLayerer{}
		if which == "interactive" {
			layerer = &InteractiveLayerer{}
		}
		done := make(chan error)
		go func() { done <- layerer.Process(arena, graphID) }()
		fmt.Println("CHILD RETURNED", <-done)
		return
	}
	if runtime.GOOS == "js" || runtime.GOOS == "wasip1" || runtime.Compiler != "gc" {
		t.Skip("needs child processes and gc's stack limit (debug.SetMaxStack)")
	}
	for _, which := range []string{"longest", "interactive"} {
		command := exec.Command(os.Args[0], "-test.run=^TestCyclicLayeringOverflowsLikeOriginal$", "-test.count=1")
		command.Env = append(os.Environ(), "ELK_TEST_CYCLE_CHILD="+which)
		output, err := command.CombinedOutput()
		text := string(output)
		if err == nil || !strings.Contains(text, "goroutine stack exceeds") || strings.Contains(text, "CHILD HEAP LIMIT") {
			if len(text) > 1500 {
				text = text[:1500]
			}
			t.Fatalf("%s layerer on a cycle: %v (want a fatal stack overflow)\n%s", which, err, text)
		}
	}
}
