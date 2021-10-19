package mathx

import "math"

// EuclideanDistance finds the Euclidean distance between two vectors.
// Returns false if:
//	len(v1) != len(v2)
func Euclidean(v1, v2 []float64) (float64, bool) {
	if len(v1) != len(v2) {
		return 0, false
	}
	var r float64
	for i := 0; i < len(v1); i++ {
		r += (v1[i] - v2[i]) * (v1[i] - v2[i])
	}
	return math.Sqrt(r), true
}
