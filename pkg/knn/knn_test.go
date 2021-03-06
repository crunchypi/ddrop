package knn

import (
	"testing"

	"github.com/crunchypi/ddrop/pkg/mathx"
)

func TestResultItemsBubbleInsert(t *testing.T) {

	type testCases struct {
		insertee       resultItem
		testItems      resultItems
		expectedScores []float64
		ascending      bool
	}

	tCases := []testCases{
		// 0) Linear.
		{
			insertee: resultItem{score: 0, set: true},
			testItems: resultItems{
				{score: 1, set: true},
				{score: 2, set: true},
				{score: 3, set: true},
			},
			expectedScores: []float64{0, 1, 2},
			ascending:      true,
		},
		// 1) Guard false positive.
		{
			insertee: resultItem{score: 4, set: true},
			testItems: resultItems{
				{score: 1, set: true},
				{score: 2, set: true},
				{score: 3, set: true},
			},
			expectedScores: []float64{1, 2, 3},
			ascending:      true,
		},
		// 2) Linear with one unset.
		{
			insertee: resultItem{score: 0, set: true},
			testItems: resultItems{
				{score: 1, set: true},
				{score: 2, set: false},
				{score: 3, set: true},
			},
			expectedScores: []float64{0, 1, 3},
			ascending:      true,
		},
		// 3) Linear (descend).
		{
			insertee: resultItem{score: 4, set: true},
			testItems: resultItems{
				{score: 3, set: true},
				{score: 2, set: true},
				{score: 1, set: true},
			},
			expectedScores: []float64{4, 3, 2},
			ascending:      false,
		},
		// 4) Guard false positive (descend)
		{
			insertee: resultItem{score: 0, set: true},
			testItems: resultItems{
				{score: 3, set: true},
				{score: 2, set: true},
				{score: 1, set: true},
			},
			expectedScores: []float64{3, 2, 1},
			ascending:      false,
		},
		// 5) Linear with one unset (descend).
		{
			insertee: resultItem{score: 4, set: true},
			testItems: resultItems{
				{score: 3, set: true},
				{score: 2, set: false},
				{score: 1, set: true},
			},
			expectedScores: []float64{4, 3, 1},
			ascending:      false,
		},
	}

	for i, tCase := range tCases {
		tCase.testItems.bubbleInsert(tCase.insertee, tCase.ascending)
		for j, expectedScore := range tCase.expectedScores {
			gotScore := tCase.testItems[j].score

			if expectedScore != gotScore {
				s := "failed on test case no. %v."
				s += " resultItems[%v].score is %v, want %v\n"
				t.Fatalf(s, i, j, gotScore, expectedScore)
			}
		}
	}
}

func newVecPoolGenerator(vecs [][]float64) VecPoolGenerator {
	i := 0
	return func() ([]float64, bool) {
		if i >= len(vecs) {
			return nil, false
		}
		i++
		return vecs[i-1], true
	}
}

func TestKNNEucFloats(t *testing.T) {
	searchVec := []float64{0, 1, 2}
	vecPool := newVecPoolGenerator([][]float64{
		{1, 5, 4}, // dist to SearchVec: ~4.582.
		{0, 3, 5}, // dist to SearchVec: ~3.605.
	})
	r, ok := KNNEucFloats(searchVec, vecPool, 1)
	if !ok {
		t.Fatal("arg check fail")
	}

	if len(r) != 1 {
		t.Fatal("unexpected result len:", len(r))
	}

	if r[0] != 1 {
		t.Fatal("unexpected result index:", r[0])
	}
}

func TestKFNEucFloats(t *testing.T) {
	searchVec := []float64{0, 1, 2}
	vecPool := newVecPoolGenerator([][]float64{
		{1, 5, 4}, // dist to SearchVec: ~4.582.
		{0, 3, 5}, // dist to SearchVec: ~3.605.
	})
	r, ok := KFNEucFloats(searchVec, vecPool, 1)
	if !ok {
		t.Fatal("arg check fail")
	}

	if len(r) != 1 {
		t.Fatal("unexpected result len:", len(r))
	}

	if r[0] != 0 {
		t.Fatal("unexpected result index:", r[0])
	}
}

func TestKNNCosFloats(t *testing.T) {
	searchVec := []float64{0, 1, 2}
	vecPool := newVecPoolGenerator([][]float64{
		{1, 5, 4}, // dist to SearchVec: ~0.897
		{0, 3, 5}, // dist to SearchVec: ~0.997.
	})
	r, ok := KNNCosFloats(searchVec, vecPool, 1)
	if !ok {
		t.Fatal("arg check fail")
	}

	if len(r) != 1 {
		t.Fatal("unexpected result len:", len(r))
	}

	if r[0] != 1 {
		t.Fatal("unexpected result index:", r[0])
	}
}

func TestKFNCosFloats(t *testing.T) {
	searchVec := []float64{0, 1, 2}
	vecPool := newVecPoolGenerator([][]float64{
		{1, 5, 4}, // dist to SearchVec: ~0.897
		{0, 3, 5}, // dist to SearchVec: ~0.997.
	})
	r, ok := KFNCosFloats(searchVec, vecPool, 1)
	if !ok {
		t.Fatal("arg check fail")
	}

	if len(r) != 1 {
		t.Fatal("unexpected result len:", len(r))
	}

	if r[0] != 0 {
		t.Fatal("unexpected result index:", r[0])
	}
}

func newDistancerPoolGenerator(distancers []mathx.Distancer) DistancerPoolGenerator {
	i := 0
	return func() (mathx.Distancer, bool) {
		if i >= len(distancers) {
			return nil, false
		}
		i++
		return distancers[i-1], true
	}
}

func TestKNNEucDist(t *testing.T) {
	query := mathx.NewSafeVec(0, 1, 2)
	pool := newDistancerPoolGenerator([]mathx.Distancer{
		mathx.NewSafeVec(1, 5, 4), // dist to SearchVec: ~4.582.
		mathx.NewSafeVec(0, 3, 5), // dist to SearchVec: ~3.605.
	})
	r, ok := KNNEucDist(query, pool, 1)
	if !ok {
		t.Fatal("arg check fail")
	}

	if len(r) != 1 {
		t.Fatal("unexpected result len:", len(r))
	}

	if r[0] != 1 {
		t.Fatal("unexpected result index:", r[0])
	}
}

func TestKFNEucDist(t *testing.T) {
	query := mathx.NewSafeVec(0, 1, 2)
	pool := newDistancerPoolGenerator([]mathx.Distancer{
		mathx.NewSafeVec(1, 5, 4), // dist to SearchVec: ~4.582.
		mathx.NewSafeVec(0, 3, 5), // dist to SearchVec: ~3.605.
	})
	r, ok := KFNEucDist(query, pool, 1)
	if !ok {
		t.Fatal("arg check fail")
	}

	if len(r) != 1 {
		t.Fatal("unexpected result len:", len(r))
	}

	if r[0] != 0 {
		t.Fatal("unexpected result index:", r[0])
	}
}

func TestKNNCosDist(t *testing.T) {
	query := mathx.NewSafeVec(0, 1, 2)
	pool := newDistancerPoolGenerator([]mathx.Distancer{
		mathx.NewSafeVec(1, 5, 4), // dist to SearchVec: ~0.897
		mathx.NewSafeVec(0, 3, 5), // dist to SearchVec: ~0.997.
	})
	r, ok := KNNCosDist(query, pool, 1)
	if !ok {
		t.Fatal("arg check fail")
	}

	if len(r) != 1 {
		t.Fatal("unexpected result len:", len(r))
	}

	if r[0] != 1 {
		t.Fatal("unexpected result index:", r[0])
	}
}

func TestKFNCosDist(t *testing.T) {
	query := mathx.NewSafeVec(0, 1, 2)
	pool := newDistancerPoolGenerator([]mathx.Distancer{
		mathx.NewSafeVec(1, 5, 4), // dist to SearchVec: ~0.897
		mathx.NewSafeVec(0, 3, 5), // dist to SearchVec: ~0.997.
	})
	r, ok := KFNCosDist(query, pool, 1)
	if !ok {
		t.Fatal("arg check fail")
	}

	if len(r) != 1 {
		t.Fatal("unexpected result len:", len(r))
	}

	if r[0] != 0 {
		t.Fatal("unexpected result index:", r[0])
	}
}
