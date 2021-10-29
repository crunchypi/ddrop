package mathx

import (
	"testing"
)

func TestPeek(t *testing.T) {
	v := NewSafeVec(1, 2)
	elm, ok := v.Peek(0)
	if !ok || elm != 1 {
		t.Fatalf("peek no. 1: want 1, got %v (ok=%v)", elm, ok)
	}
	elm, ok = v.Peek(1)
	if !ok || elm != 2 {
		t.Fatalf("peek no. 2: want 1, got %v (ok=%v)", elm, ok)
	}
	_, ok = v.Peek(2)
	if ok {
		t.Fatalf("peek no. 4: did not get out-of bounds")
	}
}

func TestSafeVecEucDist(t *testing.T) {
	type tcase struct {
		vec1   Distancer
		vec2   Distancer
		answer float64
	}

	cases := []tcase{
		{vec1: NewSafeVec(0, 1, 2), vec2: NewSafeVec(1, 5, 4), answer: 4.5826},
		{vec1: NewSafeVec(0, 1, 2), vec2: NewSafeVec(0, 3, 5), answer: 3.6056},
	}

	for i, c := range cases {
		res, _ := c.vec1.EuclideanDistance(c.vec2)
		res = RoundF64(res, 4) // 4 decimal places.

		if res != c.answer {
			t.Fatalf("failed case %v. want %v, got %v", i, c.answer, res)
		}
	}
}

func TestSafeVecCosDist(t *testing.T) {
	type tcase struct {
		vec1   Distancer
		vec2   Distancer
		answer float64
	}

	cases := []tcase{
		{vec1: NewSafeVec(0, 1, 2), vec2: NewSafeVec(1, 5, 4), answer: 0.897},
		{vec1: NewSafeVec(0, 1, 2), vec2: NewSafeVec(0, 3, 5), answer: 0.997},
		{vec1: NewSafeVec(1, 1, 1), vec2: NewSafeVec(2, 2, 2), answer: 1.000},
	}

	for i, c := range cases {
		res, _ := c.vec1.CosineSimilarity(c.vec2)
		res = RoundF64(res, 3) // 3 decimal places.

		if res != c.answer {
			t.Fatalf("failed case %v. want %v, got %v", i, c.answer, res)
		}
	}
}
