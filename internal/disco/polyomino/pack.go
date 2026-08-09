// Copyright (c) 2017 Kiel University and others.
// Copyright (c) 2017 University of Kiel and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package polyomino

import (
	"math"
	"sort"
)

type Options struct {
	AspectRatio   float64
	Fill          bool
	LowLevelSort  string
	HighLevelSort string
	Traversal     string
}

func Fill(poly *Poly) {
	north, south := make([]int, poly.Width), make([]int, poly.Width)
	east, west := make([]int, poly.Height), make([]int, poly.Height)
	for x := 0; x < poly.Width; x++ {
		y := 0
		for y < poly.Height && !poly.IsBlocked(x, y) {
			y++
		}
		north[x] = y
		y = poly.Height - 1
		for y >= 0 && !poly.IsBlocked(x, y) {
			y--
		}
		south[x] = y
	}
	for y := 0; y < poly.Height; y++ {
		x := 0
		for x < poly.Width && !poly.IsBlocked(x, y) {
			x++
		}
		east[y] = x
		x = poly.Width - 1
		for x >= 0 && !poly.IsBlocked(x, y) {
			x--
		}
		west[y] = x
	}
	for x := 0; x < poly.Width; x++ {
		for y := 0; y < poly.Height; y++ {
			if x < west[y] && x > east[y] && y < south[x] && y > north[x] {
				poly.Set(x, y, Blocked)
			}
		}
	}
}

func Pack(polys []*Poly, options Options) (*Grid, []*Poly) {
	width, height := 0, 0
	for _, poly := range polys {
		if options.Fill {
			Fill(poly)
		}
		width += poly.Width
		height += poly.Height
	}
	if len(polys) > 0 {
		width += polys[0].Width
		height += polys[0].Height
	}
	width *= 2
	height *= 2
	if options.AspectRatio > 1 {
		width = int(math.Ceil(float64(width) * options.AspectRatio))
	} else {
		height = int(math.Ceil(float64(height) / options.AspectRatio))
	}
	grid := NewGrid(width, height)

	if options.LowLevelSort == "BY_SIZE" {
		sort.SliceStable(polys, func(i, j int) bool { return polys[i].Width+polys[i].Height > polys[j].Width+polys[j].Height })
	} else {
		sort.SliceStable(polys, func(i, j int) bool { return shapeValue(polys[i]) > shapeValue(polys[j]) })
	}
	sort.SliceStable(polys, func(i, j int) bool { return len(polys[i].Extensions) < len(polys[j].Extensions) })
	if options.HighLevelSort == "CORNER_CASES_THAN_SINGLE_SIDE_LAST" {
		sort.SliceStable(polys, func(i, j int) bool { return singleDirection(polys[i]) < singleDirection(polys[j]) })
		sort.SliceStable(polys, func(i, j int) bool { return cornerCase(polys[i]) < cornerCase(polys[j]) })
	} else {
		sort.SliceStable(polys, func(i, j int) bool { return directionCount(polys[i]) < directionCount(polys[j]) })
	}

	for _, poly := range polys {
		x, y := 0, 0
		state := successorState{poly: poly, mode: options.Traversal}
		for grid.Intersects(poly, x, y) {
			x, y = state.next(x, y)
		}
		grid.Add(poly, x, y)
	}
	return grid, polys
}

func shapeValue(poly *Poly) int {
	w, h := poly.Width, poly.Height
	if w < h {
		w *= w
	} else {
		h *= h
	}
	return w + h
}
func directions(poly *Poly) map[Direction]bool {
	result := map[Direction]bool{}
	for _, ext := range poly.Extensions {
		result[ext.Direction] = true
	}
	return result
}
func directionCount(poly *Poly) int { return len(directions(poly)) }
func singleDirection(poly *Poly) int {
	if directionCount(poly) == 1 {
		return 1
	}
	return 0
}
func cornerCase(poly *Poly) int {
	dirs := directions(poly)
	if len(dirs) != 2 {
		return 0
	}
	horizontal := 0
	for direction := range dirs {
		if direction.Horizontal() {
			horizontal++
		}
	}
	if horizontal%2 == 0 {
		return 0
	}
	return 1
}

type successorState struct {
	poly        *Poly
	mode        string
	allowed     [4]bool
	initialized bool
}

func lineByLine(x, y int) (int, int) {
	if x >= 0 {
		if x == y {
			return -x - 1, -x - 1
		}
		if x == -y {
			return -x, y + 1
		}
	}
	if abs(x) > abs(y) {
		if x < 0 {
			return -x, y
		}
		return -x, y + 1
	}
	return x + 1, y
}
func jitter(x, y int) (int, int) {
	cost := max(abs(x), abs(y))
	if x <= 0 && x == y {
		return 0, y - 1
	}
	if x == -cost && y != cost {
		nx := y
		if y >= 0 {
			nx++
		}
		return nx, x
	}
	return -y, x
}
func spiral(x, y int) (int, int) {
	cost := max(abs(x), abs(y))
	if x < cost && y == -cost {
		return x + 1, y
	}
	if x == cost && y < cost {
		return x, y + 1
	}
	if x >= -cost && y == cost {
		return x - 1, y
	}
	return x, y - 1
}
func manhattan(x, y int) (int, int) {
	if x == 0 && y == 0 {
		return x, y - 1
	}
	if x == -1 && y <= 0 {
		return 0, y - 2
	}
	if x <= 0 && y > 0 {
		return x - 1, y - 1
	}
	if x >= 0 && y < 0 {
		return x + 1, y + 1
	}
	if x > 0 && y >= 0 {
		return x - 1, y + 1
	}
	return x + 1, y - 1
}

func (state *successorState) base(x, y int) (int, int) {
	switch state.mode {
	case "SPIRAL":
		return spiral(x, y)
	case "LINE_BY_LINE", "QUADRANTS_LINE_BY_LINE", "COMBINE_LINE_BY_LINE_MANHATTAN":
		return lineByLine(x, y)
	case "MANHATTAN", "QUADRANTS_MANHATTAN":
		return manhattan(x, y)
	case "JITTER", "QUADRANTS_JITTER", "COMBINE_JITTER_MANHATTAN":
		return jitter(x, y)
	default:
		return lineByLine(x, y)
	}
}
func (state *successorState) usesQuadrants() bool {
	switch state.mode {
	case "QUADRANTS_LINE_BY_LINE", "QUADRANTS_MANHATTAN", "QUADRANTS_JITTER", "COMBINE_LINE_BY_LINE_MANHATTAN", "COMBINE_JITTER_MANHATTAN":
		return true
	}
	return false
}
func (state *successorState) initialize() {
	state.allowed = [4]bool{true, true, true, true}
	dirs := directions(state.poly)
	if dirs[North] && !dirs[South] {
		state.allowed[1] = false
	}
	if !dirs[North] && dirs[South] {
		state.allowed[3] = false
	}
	if dirs[East] && !dirs[West] {
		state.allowed[2] = false
	}
	if !dirs[East] && dirs[West] {
		state.allowed[0] = false
	}
	state.initialized = true
}
func (state *successorState) next(x, y int) (int, int) {
	if !state.usesQuadrants() {
		return state.base(x, y)
	}
	if !state.initialized {
		state.initialize()
	}
	for {
		x, y = state.base(x, y)
		validX := state.allowed[0]
		if x >= 0 {
			validX = state.allowed[2]
		}
		validY := state.allowed[1]
		if y >= 0 {
			validY = state.allowed[3]
		}
		if validX && validY {
			return x, y
		}
	}
}
func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
