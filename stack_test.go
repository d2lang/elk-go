// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package elk

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strconv"
	"testing"
)

// chainRequest is a path n0 -> n1 -> ... with d2's root options.
func chainRequest(n int) []byte {
	type object = map[string]any
	children := make([]any, n)
	for index := range children {
		children[index] = object{"id": fmt.Sprintf("n%d", index), "x": 0, "y": 0, "width": 50, "height": 30}
	}
	edges := make([]any, 0, n)
	for index := 1; index < n; index++ {
		edges = append(edges, object{"id": fmt.Sprintf("e%d", index),
			"sources": []any{fmt.Sprintf("n%d", index-1)}, "targets": []any{fmt.Sprintf("n%d", index)}})
	}
	graph := object{"id": "", "layoutOptions": object{
		"elk.algorithm": "layered", "elk.hierarchyHandling": "INCLUDE_CHILDREN", "elk.direction": "DOWN",
		"elk.layered.considerModelOrder.strategy": "NODES_AND_EDGES", "elk.layered.cycleBreaking.strategy": "GREEDY_MODEL_ORDER",
		"elk.layered.nodePlacement.bk.fixedAlignment": "BALANCED", "elk.layered.thoroughness": 8,
	}, "children": children, "edges": edges}
	input, err := json.Marshal(graph)
	if err != nil {
		panic(err)
	}
	return input
}

// TestLongChainFitsOneMiBStack lays out long chains on a
// goroutine whose stack may not grow beyond 1 MiB, the stack of d2's
// WebAssembly build (TinyGo -stack-size=1MB). The original fits chains of up
// to 4656 nodes this way; the component search and network simplex walks of
// an earlier version used more stack per node and overflowed from 3640
// nodes; they are iterative now. A stack
// overflow kills the process, so each chain runs in a child process.
func TestLongChainFitsOneMiBStack(t *testing.T) {
	if value := os.Getenv("ELK_TEST_CHAIN_CHILD"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil {
			t.Fatal(err)
		}
		input := chainRequest(n)
		debug.SetMaxStack(1 << 20)
		done := make(chan error)
		go func() {
			_, err := LayoutJSON(input)
			done <- err
		}()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		return
	}
	if runtime.GOOS == "js" || runtime.GOOS == "wasip1" {
		t.Skip("no child processes")
	}
	for _, n := range []int{4656, 20000} {
		command := exec.Command(os.Args[0], "-test.run=^TestLongChainFitsOneMiBStack$", "-test.count=1")
		command.Env = append(os.Environ(), "ELK_TEST_CHAIN_CHILD="+strconv.Itoa(n))
		output, err := command.CombinedOutput()
		if err != nil {
			if len(output) > 2000 {
				output = output[:2000]
			}
			t.Fatalf("chain of %d nodes under a 1 MiB stack: %v\n%s", n, err, output)
		}
	}
}
