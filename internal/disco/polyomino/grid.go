// Copyright (c) 2017 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package polyomino is a private port of ELK 0.8.1's common polyomino
// structures. It lives below DisCo to avoid coupling unrelated providers.
package polyomino

import "fmt"

type Cell uint8

const (
	Empty Cell = iota
	Blocked
	WeaklyBlocked
)

type Grid struct {
	Width, Height int
	CenterX       int
	CenterY       int
	cells         []Cell
}

func NewGrid(width, height int) *Grid {
	return &Grid{Width: width, Height: height, CenterX: (width - 1) >> 1, CenterY: (height - 1) >> 1, cells: make([]Cell, width*height)}
}

func (grid *Grid) Reinitialize(width, height int) {
	grid.Width, grid.Height = width, height
	grid.CenterX, grid.CenterY = (width-1)>>1, (height-1)>>1
	grid.cells = make([]Cell, width*height)
}

func (grid *Grid) InBounds(x, y int) bool {
	return x >= 0 && y >= 0 && x < grid.Width && y < grid.Height
}

func (grid *Grid) index(x, y int) int {
	if !grid.InBounds(x, y) {
		panic(fmt.Sprintf("grid is only of size %d*%d; requested (%d,%d)", grid.Width, grid.Height, x, y))
	}
	return y*grid.Width + x
}

func (grid *Grid) At(x, y int) Cell        { return grid.cells[grid.index(x, y)] }
func (grid *Grid) Set(x, y int, cell Cell) { grid.cells[grid.index(x, y)] = cell }
func (grid *Grid) IsEmpty(x, y int) bool   { return grid.At(x, y) == Empty }
func (grid *Grid) IsBlocked(x, y int) bool { return grid.At(x, y) == Blocked }

func (grid *Grid) InBoundsCenter(x, y int) bool  { return grid.InBounds(x+grid.CenterX, y+grid.CenterY) }
func (grid *Grid) AtCenter(x, y int) Cell        { return grid.At(x+grid.CenterX, y+grid.CenterY) }
func (grid *Grid) SetCenter(x, y int, cell Cell) { grid.Set(x+grid.CenterX, y+grid.CenterY, cell) }

type Direction uint8

const (
	North Direction = iota
	East
	South
	West
)

func (direction Direction) Horizontal() bool { return direction == East || direction == West }

type Extension struct {
	Direction     Direction
	Offset, Width int
}

type Poly struct {
	*Grid
	X, Y       int
	Extensions []Extension
	Data       int
}

func NewPoly(width, height int) *Poly { return &Poly{Grid: NewGrid(width, height)} }

func (poly *Poly) WeaklyBlockArea(x0, y0, x1, y1 int) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if !poly.IsBlocked(x, y) {
				poly.Set(x, y, WeaklyBlocked)
			}
		}
	}
}

func (grid *Grid) weaklyIntersectsArea(x0, y0, x1, y1 int) bool {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if grid.IsBlocked(x, y) {
				return true
			}
		}
	}
	return false
}

func (grid *Grid) Intersects(poly *Poly, offsetX, offsetY int) bool {
	for x := 0; x < poly.Width; x++ {
		tx := x - poly.CenterX + offsetX
		for y := 0; y < poly.Height; y++ {
			ty := y - poly.CenterY + offsetY
			if grid.InBoundsCenter(tx, ty) {
				other, own := poly.At(x, y), grid.AtCenter(tx, ty)
				if (other != Empty && own == Blocked) || (other == Blocked && own != Empty) {
					return true
				}
			}
		}
	}
	left := grid.CenterX - poly.CenterX + offsetX
	right := left + poly.Width
	top := grid.CenterY - poly.CenterY + offsetY
	bottom := top + poly.Height
	for _, extension := range poly.Extensions {
		switch extension.Direction {
		case North:
			if grid.weaklyIntersectsArea(left+extension.Offset, 0, left+extension.Width, top-1) {
				return true
			}
		case East:
			if grid.weaklyIntersectsArea(right, top+extension.Offset, grid.Width-1, top+extension.Width) {
				return true
			}
		case South:
			if grid.weaklyIntersectsArea(left+extension.Offset, bottom, left+extension.Width, grid.Height-1) {
				return true
			}
		case West:
			if grid.weaklyIntersectsArea(0, top+extension.Offset, left-1, top+extension.Width) {
				return true
			}
		}
	}
	return false
}

func (grid *Grid) Add(poly *Poly, offsetX, offsetY int) {
	for x := 0; x < poly.Width; x++ {
		tx := x - poly.CenterX + offsetX
		for y := 0; y < poly.Height; y++ {
			ty := y - poly.CenterY + offsetY
			switch poly.At(x, y) {
			case Blocked:
				if grid.AtCenter(tx, ty) != WeaklyBlocked {
					grid.SetCenter(tx, ty, Blocked)
				}
			case WeaklyBlocked:
				if grid.AtCenter(tx, ty) != Blocked {
					grid.SetCenter(tx, ty, WeaklyBlocked)
				}
			}
		}
	}
	poly.X = grid.CenterX - poly.CenterX + offsetX
	poly.Y = grid.CenterY - poly.CenterY + offsetY
	for _, extension := range poly.Extensions {
		switch extension.Direction {
		case North:
			grid.weaklyBlockArea(poly.X+extension.Offset, 0, poly.X+extension.Width, poly.Y-1)
		case East:
			grid.weaklyBlockArea(poly.X+poly.Width, poly.Y+extension.Offset, grid.Width-1, poly.Y+extension.Width)
		case South:
			grid.weaklyBlockArea(poly.X+extension.Offset, poly.Y+poly.Height, poly.X+extension.Width, grid.Height-1)
		case West:
			grid.weaklyBlockArea(0, poly.Y+extension.Offset, poly.X-1, poly.Y+extension.Width)
		}
	}
}

func (grid *Grid) weaklyBlockArea(x0, y0, x1, y1 int) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if !grid.IsBlocked(x, y) {
				grid.Set(x, y, WeaklyBlocked)
			}
		}
	}
}

func (grid *Grid) FilledBounds() (x, y, width, height int) {
	minX, minY, maxX, maxY := int(^uint(0)>>1), int(^uint(0)>>1), -int(^uint(0)>>1)-1, -int(^uint(0)>>1)-1
	for xi := 0; xi < grid.Width; xi++ {
		for yi := 0; yi < grid.Height; yi++ {
			if grid.IsBlocked(xi, yi) {
				if xi < minX {
					minX = xi
				}
				if xi > maxX {
					maxX = xi
				}
				if yi < minY {
					minY = yi
				}
				if yi > maxY {
					maxY = yi
				}
			}
		}
	}
	return minX, minY, maxX - minX + 1, maxY - minY + 1
}
