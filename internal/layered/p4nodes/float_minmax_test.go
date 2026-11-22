package p4nodes

import (
	"math"
	"testing"
)

func TestFloatMinMaxMatchMath(t *testing.T) {
	values := []float64{math.Inf(1), math.Inf(-1), math.NaN(), 0, math.Copysign(0, -1), 1, -1, 2.5, -7.25, math.MaxFloat64, -math.MaxFloat64, math.SmallestNonzeroFloat64}
	same := func(a, b float64) bool {
		if math.IsNaN(a) || math.IsNaN(b) {
			return math.IsNaN(a) && math.IsNaN(b)
		}
		return math.Float64bits(a) == math.Float64bits(b)
	}
	for _, x := range values {
		for _, y := range values {
			if got, want := maxFloat(x, y), math.Max(x, y); !same(got, want) {
				t.Fatalf("maxFloat(%v, %v) = %v, math.Max %v", x, y, got, want)
			}
			if got, want := minFloat(x, y), math.Min(x, y); !same(got, want) {
				t.Fatalf("minFloat(%v, %v) = %v, math.Min %v", x, y, got, want)
			}
		}
	}
}
