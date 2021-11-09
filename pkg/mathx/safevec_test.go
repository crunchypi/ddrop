package mathx

import (
	"testing"
)

func TestSafeVecIter(t *testing.T) {
	v := []float64{1, 2}
	w := NewSafeVec(v...)

	w.Iter(func(index int, element float64) bool {
		if v[index] != element {
			t.Fatalf("unexpected safevec elm on index %v: %v", index, element)
		}
		return true
	})
}

func TestSafeVecEq(t *testing.T) {
	v := []float64{1, 2, 3, 0, 4}

	w1 := NewSafeVec(v...)
	w2 := NewSafeVec(v...)

	if !w1.Eq(w2) {
		t.Fatal("false negative")
	}

	w3 := NewSafeVec(append(v, 1.)...)
	if w1.Eq(w3) {
		t.Fatal("false positive")
	}

}

func TestSafeVecIn(t *testing.T) {
	vecs := []*SafeVec{
		NewSafeVec(1, 2, 3),
		NewSafeVec(2, 3, 4),
		NewSafeVec(3, 4, 5),
	}

	if !vecs[0].In(vecs) {
		t.Fatal("false negative")
	}

	if NewSafeVec(0, 0, 0).In(vecs) {
		t.Fatal("false positive")
	}
}

func TestSafeVecPeek(t *testing.T) {
	v := []float64{1, 2, 3, 0, 4}
	w := NewSafeVec(v...)

	for i, elm1 := range v {
		elm2, ok := w.Peek(i)
		if !ok {
			t.Fatal("unexpected out of bounds on index", i)
		}
		if elm1 != elm2 {
			t.Fatal("unexpected neq element on index", 1)
		}
	}

	elm, ok := w.Peek(len(v))
	if ok {
		t.Fatalf("did not get out-of bounds on index %v. elm=%v", len(v), elm)
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
