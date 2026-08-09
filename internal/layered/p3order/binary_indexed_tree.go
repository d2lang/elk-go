// Copyright (c) 2016 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

// binaryIndexedTree is ELK 0.8.1's sorted integer multiset. It stores values
// in [0,maxNum) and supports insertion, exclusive rank, and bulk removal in
// O(log maxNum). Keeping the two backing arrays is intentional: removeAll
// needs the multiplicity at one leaf while binarySums is a Fenwick tree.
type binaryIndexedTree struct {
	binarySums   []int
	numsPerIndex []int
	size         int
	maxNum       int
}

func newBinaryIndexedTree(maxNum int) *binaryIndexedTree {
	if maxNum < 0 {
		panic("p3order: negative binary indexed tree size")
	}
	return &binaryIndexedTree{
		binarySums:   make([]int, maxNum+1),
		numsPerIndex: make([]int, maxNum),
		maxNum:       maxNum,
	}
}

func (tree *binaryIndexedTree) add(index int) {
	if index < 0 || index >= tree.maxNum {
		panic("p3order: binary indexed tree index out of range")
	}
	tree.size++
	tree.numsPerIndex[index]++
	for next := index + 1; next < len(tree.binarySums); next += next & -next {
		tree.binarySums[next]++
	}
}

// rank returns the number of stored values strictly before index.
func (tree *binaryIndexedTree) rank(index int) int {
	if index < 0 || index > tree.maxNum {
		panic("p3order: binary indexed tree rank index out of range")
	}
	sum := 0
	for next := index; next > 0; next -= next & -next {
		sum += tree.binarySums[next]
	}
	return sum
}

func (tree *binaryIndexedTree) removeAll(index int) {
	if index < 0 || index >= tree.maxNum {
		panic("p3order: binary indexed tree index out of range")
	}
	entries := tree.numsPerIndex[index]
	if entries == 0 {
		return
	}
	tree.numsPerIndex[index] = 0
	tree.size -= entries
	for next := index + 1; next < len(tree.binarySums); next += next & -next {
		tree.binarySums[next] -= entries
	}
}

func (tree *binaryIndexedTree) clear() {
	tree.binarySums = make([]int, tree.maxNum+1)
	tree.numsPerIndex = make([]int, tree.maxNum)
	tree.size = 0
}

func (tree *binaryIndexedTree) empty() bool { return tree.size == 0 }
