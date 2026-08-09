// Copyright (c) 2010, 2015 Kiel University and others.
// Copyright (c) 2010, 2017 Kiel University and others.
// Copyright (c) 2014, 2017 Kiel University and others.
// Copyright (c) 2016 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package geometry implements ELK's core two-dimensional data types.
package geometry

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// Point is the Go counterpart of ELK's KVector.
type Point struct {
	X float64
	Y float64
}

func NewPoint(x, y float64) Point { return Point{X: x, Y: y} }

// Between returns the vector pointing from start to end.
func Between(start, end Point) Point { return Point{X: end.X - start.X, Y: end.Y - start.Y} }

// FromAngle returns a unit vector at the given angle in radians.
func FromAngle(angle float64) Point { return Point{X: math.Cos(angle), Y: math.Sin(angle)} }

func (p Point) LengthSquared() float64     { return p.X*p.X + p.Y*p.Y }
func (p Point) Length() float64            { return math.Sqrt(p.LengthSquared()) }
func (p Point) Distance(q Point) float64   { return Between(p, q).Length() }
func (p Point) Dot(q Point) float64        { return p.X*q.X + p.Y*q.Y }
func (p Point) Cross(q Point) float64      { return p.X*q.Y - p.Y*q.X }
func (p Point) Add(q Point) Point          { return Point{X: p.X + q.X, Y: p.Y + q.Y} }
func (p Point) Sub(q Point) Point          { return Point{X: p.X - q.X, Y: p.Y - q.Y} }
func (p Point) Scale(factor float64) Point { return Point{X: p.X * factor, Y: p.Y * factor} }
func (p Point) ScaleXY(x, y float64) Point { return Point{X: p.X * x, Y: p.Y * y} }
func (p Point) Negate() Point              { return Point{X: -p.X, Y: -p.Y} }

func (p Point) Normalize() Point {
	length := p.Length()
	if length == 0 {
		return p
	}
	return p.Scale(1 / length)
}

func (p Point) ScaleToLength(length float64) Point { return p.Normalize().Scale(length) }

// Radians returns an angle in [0, 2*pi), matching KVector.toRadians.
func (p Point) Radians() float64 {
	length := p.Length()
	if length == 0 {
		return math.NaN()
	}
	asin := math.Asin(p.Y / length)
	if p.X >= 0 && p.Y >= 0 {
		return asin
	}
	if p.X < 0 {
		return math.Pi - asin
	}
	return 2*math.Pi + asin
}

func (p Point) Degrees() float64 { return p.Radians() * 180 / math.Pi }

func (p Point) Rotate(angle float64) Point {
	sin, cos := math.Sincos(angle)
	return Point{X: p.X*cos - p.Y*sin, Y: p.X*sin + p.Y*cos}
}

func (p Point) Angle(q Point) float64 { return math.Acos(p.Dot(q) / (p.Length() * q.Length())) }

func (p Point) Bound(lowX, lowY, highX, highY float64) Point {
	if highX < lowX || highY < lowY {
		panic("high bounds must not be smaller than low bounds")
	}
	p.X = math.Max(lowX, math.Min(highX, p.X))
	p.Y = math.Max(lowY, math.Min(highY, p.Y))
	return p
}

func (p Point) IsNaN() bool      { return math.IsNaN(p.X) || math.IsNaN(p.Y) }
func (p Point) IsInfinite() bool { return math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) }
func (p Point) FuzzyEqual(q Point, tolerance float64) bool {
	return math.Abs(p.X-q.X) <= tolerance && math.Abs(p.Y-q.Y) <= tolerance
}

func ParsePoint(input string) (Point, error) {
	trimmed := strings.Trim(input, "()[]{}\"' \t\r\n")
	parts := strings.FieldsFunc(trimmed, func(r rune) bool { return r == ',' || r == ';' || r == '\r' || r == '\n' })
	if len(parts) != 2 {
		return Point{}, fmt.Errorf("exactly two numbers are expected, %d were found", len(parts))
	}
	x, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return Point{}, err
	}
	y, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return Point{}, err
	}
	return Point{X: x, Y: y}, nil
}

func (p Point) String() string { return "(" + formatDouble(p.X) + "," + formatDouble(p.Y) + ")" }

// Chain is ELK's ordered KVectorChain.
type Chain []Point

func (c Chain) Clone() Chain { return append(Chain(nil), c...) }

func (c Chain) Offset(offset Point) Chain {
	for index := range c {
		c[index] = c[index].Add(offset)
	}
	return c
}

func (c Chain) Scale(factor float64) Chain {
	for index := range c {
		c[index] = c[index].Scale(factor)
	}
	return c
}

func (c Chain) ScaleXY(x, y float64) Chain {
	for index := range c {
		c[index] = c[index].ScaleXY(x, y)
	}
	return c
}

func (c Chain) Length() float64 {
	var result float64
	for index := 1; index < len(c); index++ {
		result += c[index-1].Distance(c[index])
	}
	return result
}

func (c Chain) HasNaN() bool {
	for _, point := range c {
		if point.IsNaN() {
			return true
		}
	}
	return false
}

func (c Chain) HasInfinite() bool {
	for _, point := range c {
		if point.IsInfinite() {
			return true
		}
	}
	return false
}

// PointAt returns a point at distance from the start, or from the end when
// distance is negative.
func (c Chain) PointAt(distance float64) Point {
	if len(c) == 0 {
		panic("cannot determine a point on an empty chain")
	}
	if len(c) == 1 {
		return c[0]
	}
	forward := distance >= 0
	distance = math.Abs(distance)
	index := func(i int) int {
		if forward {
			return i
		}
		return len(c) - 1 - i
	}
	var covered float64
	for i := 0; i < len(c)-1; i++ {
		current, next := c[index(i)], c[index(i+1)]
		segment := current.Distance(next)
		if segment == 0 {
			continue
		}
		if covered+segment >= distance {
			return current.Add(next.Sub(current).Scale((distance - covered) / segment))
		}
		covered += segment
	}
	return c[index(len(c)-1)]
}

func (c Chain) Reverse() Chain {
	result := make(Chain, len(c))
	for i := range c {
		result[len(c)-1-i] = c[i]
	}
	return result
}

func ParseChain(input string) (Chain, error) {
	parts := strings.FieldsFunc(input, func(r rune) bool {
		return strings.ContainsRune(",;()[]{}", r) || unicode.IsSpace(r)
	})
	if len(parts)%2 != 0 {
		return nil, fmt.Errorf("vector chain requires coordinate pairs")
	}
	result := make(Chain, 0, len(parts)/2)
	for i := 0; i < len(parts); i += 2 {
		x, err := strconv.ParseFloat(parts[i], 64)
		if err != nil {
			return nil, err
		}
		y, err := strconv.ParseFloat(parts[i+1], 64)
		if err != nil {
			return nil, err
		}
		result = append(result, Point{X: x, Y: y})
	}
	return result, nil
}

func (c Chain) String() string {
	parts := make([]string, len(c))
	for i, point := range c {
		parts[i] = formatDouble(point.X) + "," + formatDouble(point.Y)
	}
	return "(" + strings.Join(parts, "; ") + ")"
}

// Spacing is used for both ELK padding and margin values.
type Spacing struct{ Top, Right, Bottom, Left float64 }

func UniformSpacing(value float64) Spacing {
	return Spacing{Top: value, Right: value, Bottom: value, Left: value}
}
func LRBTSpacing(horizontal, vertical float64) Spacing {
	return Spacing{Top: vertical, Right: horizontal, Bottom: vertical, Left: horizontal}
}
func (s Spacing) Horizontal() float64 { return s.Left + s.Right }
func (s Spacing) Vertical() float64   { return s.Top + s.Bottom }

func ParseSpacing(input string) (Spacing, error) {
	trimmed := strings.Trim(input, "()[]{}\"' \t\r\n")
	var result Spacing
	if trimmed == "" {
		return result, nil
	}
	for _, component := range strings.FieldsFunc(trimmed, func(r rune) bool { return r == ',' || r == ';' }) {
		pair := strings.Split(component, "=")
		if len(pair) != 2 {
			return Spacing{}, fmt.Errorf("expecting a list of key-value pairs")
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(pair[1]), 64)
		if err != nil {
			return Spacing{}, fmt.Errorf("spacing contains a value that is not a number: %w", err)
		}
		switch strings.TrimSpace(pair[0]) {
		case "top":
			result.Top = value
		case "right":
			result.Right = value
		case "bottom":
			result.Bottom = value
		case "left":
			result.Left = value
		}
	}
	return result, nil
}

func (s Spacing) String() string {
	return "[top=" + formatDouble(s.Top) + ",left=" + formatDouble(s.Left) +
		",bottom=" + formatDouble(s.Bottom) + ",right=" + formatDouble(s.Right) + "]"
}

// Rectangle is an axis-aligned rectangle.
type Rectangle struct{ X, Y, Width, Height float64 }

func (r Rectangle) Position() Point             { return Point{X: r.X, Y: r.Y} }
func (r Rectangle) TopRight() Point             { return Point{X: r.X + r.Width, Y: r.Y} }
func (r Rectangle) BottomLeft() Point           { return Point{X: r.X, Y: r.Y + r.Height} }
func (r Rectangle) BottomRight() Point          { return Point{X: r.X + r.Width, Y: r.Y + r.Height} }
func (r Rectangle) Center() Point               { return Point{X: r.X + r.Width/2, Y: r.Y + r.Height/2} }
func (r Rectangle) MaxX() float64               { return r.X + r.Width }
func (r Rectangle) MaxY() float64               { return r.Y + r.Height }
func (r Rectangle) Move(offset Point) Rectangle { r.X += offset.X; r.Y += offset.Y; return r }
func (r Rectangle) Union(other Rectangle) Rectangle {
	left := math.Min(r.X, other.X)
	top := math.Min(r.Y, other.Y)
	right := math.Max(r.MaxX(), other.MaxX())
	bottom := math.Max(r.MaxY(), other.MaxY())
	return Rectangle{X: left, Y: top, Width: right - left, Height: bottom - top}
}
func (r Rectangle) Intersects(other Rectangle) bool {
	return r.X < other.MaxX() && r.MaxX() > other.X && r.MaxY() > other.Y && r.Y < other.MaxY()
}

func formatDouble(value float64) string {
	if math.IsInf(value, 1) {
		return "Infinity"
	}
	if math.IsInf(value, -1) {
		return "-Infinity"
	}
	if math.IsNaN(value) {
		return "NaN"
	}
	return strconv.FormatFloat(value, 'g', -1, 64)
}
