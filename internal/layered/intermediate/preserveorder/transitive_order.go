// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package preserveorder

import "math/bits"

// TransitiveOrder is the biggerThan/smallerThan bookkeeping of ELK's
// model-order comparators: for every element the set of elements it was
// recorded as bigger than and the set it was recorded as smaller than. The
// comparators only test membership and form unions, so each set is a row of
// bits over the elements, numbered in order of first appearance.
//
// ELK keeps the sets in hash maps and creates an element's (empty) sets on
// first sight; an absent set and an empty one answer every membership test
// the same way, so only membership is represented here.
type TransitiveOrder[K comparable] struct {
	keys    []K         // element number -> key
	index   map[K]int32 // key -> element number, once there are many keys
	count   int
	words   int // row width in 64-bit words
	bigger  []uint64
	smaller []uint64
}

// linearKeys is how many keys are searched linearly before a map is built.
const linearKeys = 16

// minRows is the smallest number of rows (and keys) storage is made for; most
// comparators see only a few elements.
const minRows = 8

// id returns key's element number, assigning the next one on first sight.
func (order *TransitiveOrder[K]) id(key K) int {
	if order.index != nil {
		if id, found := order.index[key]; found {
			return int(id)
		}
	} else {
		for id, candidate := range order.keys {
			if candidate == key {
				return id
			}
		}
	}
	id := order.count
	order.count++
	if order.keys == nil {
		order.keys = make([]K, 0, minRows)
	}
	order.keys = append(order.keys, key)
	if order.index != nil {
		order.index[key] = int32(id)
	} else if len(order.keys) > linearKeys {
		order.index = make(map[K]int32, 2*len(order.keys))
		for number, candidate := range order.keys {
			order.index[candidate] = int32(number)
		}
	}
	order.reserve(order.count)
	return id
}

// reserve makes room for n rows of at least n bits each (new bits zero).
func (order *TransitiveOrder[K]) reserve(n int) {
	rows := 0
	if order.words > 0 {
		rows = len(order.bigger) / order.words
	}
	words := (n + 63) / 64
	if words <= order.words && n <= rows {
		return
	}
	newWords := max(words, order.words)
	if words > order.words {
		newWords = max(words, 2*order.words)
	}
	newRows := max(n, rows)
	if n > rows {
		newRows = max(n, 2*rows, minRows)
	}
	size := newRows * newWords
	both := make([]uint64, 2*size)
	bigger, smaller := both[:size:size], both[size:]
	for row := 0; row < rows; row++ {
		copy(bigger[row*newWords:], order.bigger[row*order.words:(row+1)*order.words])
		copy(smaller[row*newWords:], order.smaller[row*order.words:(row+1)*order.words])
	}
	order.bigger, order.smaller, order.words = bigger, smaller, newWords
}

func (order *TransitiveOrder[K]) row(rows []uint64, element int) []uint64 {
	return rows[element*order.words : (element+1)*order.words]
}

func hasBit(row []uint64, element int) bool { return row[element>>6]&(1<<(element&63)) != 0 }
func setBit(row []uint64, element int)      { row[element>>6] |= 1 << (element & 63) }

// Recall is initializeAndRecall: 1 when first is recorded bigger than
// second, -1 when smaller, 0 when the pair is unrelated so far. It keeps the
// source's checks in their order, including its repeated biggerThan test.
func (order *TransitiveOrder[K]) Recall(first, second K) int {
	f, s := order.id(first), order.id(second)
	switch {
	case hasBit(order.row(order.bigger, f), s):
		return 1
	case hasBit(order.row(order.bigger, s), f):
		return -1
	case hasBit(order.row(order.smaller, f), s):
		return -1
	case hasBit(order.row(order.bigger, s), f):
		return 1
	}
	return 0
}

// Update records bigger > smaller and closes the relation transitively, as
// the comparators' update methods do: bigger is also bigger than everything
// smaller was bigger than, and everything recorded smaller than bigger
// becomes bigger than smaller and all it is bigger than; symmetrically for
// the smaller-than sets. All steps are unions, so the result does not depend
// on the order in which set members are visited.
func (order *TransitiveOrder[K]) Update(bigger, smaller K) {
	b, s := order.id(bigger), order.id(smaller)
	biggerOfB, smallerOfS := order.row(order.bigger, b), order.row(order.smaller, s)
	setBit(biggerOfB, s)
	setBit(smallerOfS, b)
	biggerOfS, smallerOfB := order.row(order.bigger, s), order.row(order.smaller, b)
	for word := range biggerOfS {
		for members := biggerOfS[word]; members != 0; members &= members - 1 {
			verySmall := word<<6 | bits.TrailingZeros64(members)
			setBit(biggerOfB, verySmall)
			row := order.row(order.smaller, verySmall)
			setBit(row, b)
			for index := range row {
				row[index] |= smallerOfB[index]
			}
		}
	}
	for word := range smallerOfB {
		for members := smallerOfB[word]; members != 0; members &= members - 1 {
			veryBig := word<<6 | bits.TrailingZeros64(members)
			setBit(smallerOfS, veryBig)
			row := order.row(order.bigger, veryBig)
			setBit(row, s)
			for index := range row {
				row[index] |= biggerOfS[index]
			}
		}
	}
}

// Clear forgets every recorded relation (ClearTransitiveOrdering).
func (order *TransitiveOrder[K]) Clear() {
	order.index = nil
	order.keys = order.keys[:0]
	order.count = 0
	for index := range order.bigger {
		order.bigger[index] = 0
		order.smaller[index] = 0
	}
}
