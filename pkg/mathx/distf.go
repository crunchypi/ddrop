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

// norm computes the norm (math) of a vec.
func norm(vec []float64) float64 {
	var x float64
	for i := 0; i < len(vec); i++ {
		x += vec[i] * vec[i]
	}
	return math.Sqrt(x)
}

// CosineSimilarity finds the cosine similarity of two vectors.
// Returns false on two conditions if:
//	(A): len(v1) != len(v2)
//	(B): One of the vectors is a zero vector.
func CosineSimilarity(v1, v2 []float64) (float64, bool) {
	if len(v1) != len(v2) {
		return 0, false
	}
	norm1, norm2 := norm(v1), norm(v2)
	if norm1 == 0 && norm2 == 0 {
		return 0, false
	}
	var dot float64
	for i := 0; i < len(v1); i++ {
		dot += v1[i] * v2[i]
	}
	return dot / norm1 / norm2, true
}
