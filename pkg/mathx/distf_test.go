package mathx

import (
	"testing"
)

func TestEucDist(t *testing.T) {
	type tcase struct {
		vec1   []float64
		vec2   []float64
		answer float64
	}

	cases := []tcase{
		{vec1: []float64{0, 1, 2}, vec2: []float64{1, 5, 4}, answer: 4.5826},
		{vec1: []float64{0, 1, 2}, vec2: []float64{0, 3, 5}, answer: 3.6056},
	}

	for i, c := range cases {
		res, _ := Euclidean(c.vec1, c.vec2)
		res = RoundF64(res, 4) // 4 decimal places.

		if res != c.answer {
			t.Fatalf("failed case %v. want %v, got %v", i, c.answer, res)

		}
	}
}

func TestCosineSimilarity(t *testing.T) {
	type tcase struct {
		vec1   []float64
		vec2   []float64
		answer float64
	}

	cases := []tcase{
		{vec1: []float64{0, 1, 2}, vec2: []float64{1, 5, 4}, answer: 0.897},
		{vec1: []float64{0, 1, 2}, vec2: []float64{0, 3, 5}, answer: 0.997},
		{vec1: []float64{1, 1, 1}, vec2: []float64{2, 2, 2}, answer: 1.000},
	}

	for i, c := range cases {
		res, _ := CosineSimilarity(c.vec1, c.vec2)
		res = RoundF64(res, 3) // 4 decimal places.

		if res != c.answer {
			t.Fatalf("failed case %v. want %v, got %v", i, c.answer, res)

		}
	}
}
