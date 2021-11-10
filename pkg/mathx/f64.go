package mathx

import "math"

// RoundF64 rounds a float64 to the specified amount of decimals.
// Rounds to the closest num, so no ceil or floor.
func RoundF64(f float64, decimals int) float64 {
	// Special case.
	if decimals < 1 {
		return float64(int(f))
	}

	round := 10.
	for i := 0; i < decimals-1; i++ {
		round *= 10
	}
	return math.Round(f*round) / round
}
