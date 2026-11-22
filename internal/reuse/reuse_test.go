// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package reuse

import (
	"runtime"
	"sync"
	"testing"
)

func TestPoolKeepsSlotAcrossCollections(t *testing.T) {
	var pool Pool[int]
	if pool.Get() != nil {
		t.Fatal("empty pool returned a value")
	}
	value := new(int)
	pool.Put(value, true)
	runtime.GC()
	runtime.GC()
	if got := pool.Get(); got != value {
		t.Fatalf("Get = %p, want the slot's %p", got, value)
	}
	pool.Put(nil, true)
	if pool.Get() != nil {
		t.Fatal("nil was stored")
	}
}

func TestPoolConcurrent(t *testing.T) {
	var pool Pool[[]int]
	var wait sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for step := 0; step < 1000; step++ {
				value := pool.Get()
				if value == nil {
					value = new([]int)
				}
				*value = append((*value)[:0], step)
				pool.Put(value, step%2 == 0)
			}
		}()
	}
	wait.Wait()
}
