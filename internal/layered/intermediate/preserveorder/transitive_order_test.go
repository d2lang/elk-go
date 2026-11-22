// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package preserveorder

import (
	"math/rand"
	"testing"
)

// mapOrder is the comparators' original map-of-sets bookkeeping.
type mapOrder struct {
	biggerThan  map[int]map[int]struct{}
	smallerThan map[int]map[int]struct{}
}

func newMapOrder() *mapOrder {
	return &mapOrder{biggerThan: map[int]map[int]struct{}{}, smallerThan: map[int]map[int]struct{}{}}
}

func (order *mapOrder) recall(first, second int) int {
	if _, found := order.biggerThan[first]; !found {
		order.biggerThan[first] = map[int]struct{}{}
	} else if _, in := order.biggerThan[first][second]; in {
		return 1
	}
	if _, found := order.biggerThan[second]; !found {
		order.biggerThan[second] = map[int]struct{}{}
	} else if _, in := order.biggerThan[second][first]; in {
		return -1
	}
	if _, found := order.smallerThan[first]; !found {
		order.smallerThan[first] = map[int]struct{}{}
	} else if _, in := order.smallerThan[first][second]; in {
		return -1
	}
	if _, found := order.smallerThan[second]; !found {
		order.smallerThan[second] = map[int]struct{}{}
	} else if _, in := order.biggerThan[second][first]; in {
		return 1
	}
	return 0
}

func (order *mapOrder) ensure(node int) {
	if order.biggerThan[node] == nil {
		order.biggerThan[node] = map[int]struct{}{}
	}
	if order.smallerThan[node] == nil {
		order.smallerThan[node] = map[int]struct{}{}
	}
}

func (order *mapOrder) update(bigger, smaller int) {
	order.ensure(bigger)
	order.ensure(smaller)
	biggerBigger, smallerBigger := order.biggerThan[bigger], order.biggerThan[smaller]
	biggerSmaller, smallerSmaller := order.smallerThan[bigger], order.smallerThan[smaller]
	biggerBigger[smaller] = struct{}{}
	smallerSmaller[bigger] = struct{}{}
	for verySmall := range smallerBigger {
		order.ensure(verySmall)
		biggerBigger[verySmall] = struct{}{}
		order.smallerThan[verySmall][bigger] = struct{}{}
		for veryBig := range biggerSmaller {
			order.smallerThan[verySmall][veryBig] = struct{}{}
		}
	}
	for veryBig := range biggerSmaller {
		order.ensure(veryBig)
		smallerSmaller[veryBig] = struct{}{}
		order.biggerThan[veryBig][smaller] = struct{}{}
		for verySmall := range smallerBigger {
			order.biggerThan[veryBig][verySmall] = struct{}{}
		}
	}
}

// TestTransitiveOrderMatchesMaps drives both implementations with the same
// random calls (including contradictory updates, which create cycles) and
// checks every recall and, at the end, every pair.
func TestTransitiveOrderMatchesMaps(t *testing.T) {
	random := rand.New(rand.NewSource(7))
	for round := 0; round < 400; round++ {
		universe := 2 + random.Intn(150)
		reference := newMapOrder()
		var order TransitiveOrder[int]
		for step := 0; step < 3*universe; step++ {
			first, second := random.Intn(universe), random.Intn(universe)
			if first == second {
				continue
			}
			want := reference.recall(first, second)
			if got := order.Recall(first, second); got != want {
				t.Fatalf("round %d step %d: Recall(%d,%d) = %d, want %d", round, step, first, second, got, want)
			}
			if random.Intn(3) != 0 {
				reference.update(first, second)
				order.Update(first, second)
			}
			if random.Intn(200) == 0 {
				reference = newMapOrder()
				order.Clear()
			}
		}
		for first := 0; first < universe; first++ {
			for second := 0; second < universe; second++ {
				if first == second {
					continue
				}
				want := reference.recall(first, second)
				if got := order.Recall(first, second); got != want {
					t.Fatalf("round %d final: Recall(%d,%d) = %d, want %d", round, first, second, got, want)
				}
			}
		}
	}
}
