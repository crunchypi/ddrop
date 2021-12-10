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

func TestKNNBrute(t *testing.T) {
	type testCases struct {
		args         KNNBruteArgs
		expectResult []int
	}

	tCases := []testCases{
		// 0) k-nearest-neighbours with Euclidean distance.
		{
			args: KNNBruteArgs{
				SearchVec: []float64{0, 1, 2},
				VecPoolGenerator: newVecPoolGenerator([][]float64{
					{1, 5, 4}, // dist to SearchVec: ~4.582.
					{0, 3, 5}, // dist to SearchVec: ~3.605.
				}),
				DistanceFunc: mathx.EuclideanDistance,
				K:            1,
				Ascending:    true,
			},
			expectResult: []int{1},
		},
		// 0) k-furthest-neighbours with Euclidean distance.
		{
			args: KNNBruteArgs{
				SearchVec: []float64{0, 1, 2},
				VecPoolGenerator: newVecPoolGenerator([][]float64{
					{1, 5, 4}, // dist to SearchVec: ~4.582.
					{0, 3, 5}, // dist to SearchVec: ~3.605.
				}),
				DistanceFunc: mathx.EuclideanDistance,
				K:            1,
				Ascending:    false,
			},
			expectResult: []int{0},
		},
		// 1) k-nearest-neighbours with cosine similarity.
		{
			args: KNNBruteArgs{
				SearchVec: []float64{0, 1, 2},
				VecPoolGenerator: newVecPoolGenerator([][]float64{
					{1, 5, 4}, // dist to SearchVec: ~0.897
					{0, 3, 5}, // dist to SearchVec: ~0.997.
				}),
				DistanceFunc: mathx.CosineSimilarity,
				K:            1,
				Ascending:    true,
			},
			expectResult: []int{0},
		},
		// 1) k-furthest-neighbours with cosine similarity.
		{
			args: KNNBruteArgs{
				SearchVec: []float64{0, 1, 2},
				VecPoolGenerator: newVecPoolGenerator([][]float64{
					{1, 5, 4}, // dist to SearchVec: ~0.897
					{0, 3, 5}, // dist to SearchVec: ~0.997.
				}),
				DistanceFunc: mathx.CosineSimilarity,
				K:            1,
				Ascending:    false,
			},
			expectResult: []int{1},
		},
	}

	for i, tCase := range tCases {
		result, ok := KNNBrute(tCase.args)
		if !ok {
			t.Fatalf("failed on test case no. %v: KNNBrute returned false", i)
		}

		for j, expectedElement := range tCase.expectResult {
			resultElement := result[j]

			if expectedElement != resultElement {
				s := "failed on test case no. %v."
				s += " result[%v] is %v, want %v\n"
				t.Fatalf(s, i, j, resultElement, expectedElement)

			}
		}
	}
}

// Prefab, so re-using test case in TestKNNBrute.
func TestKNNEuc(t *testing.T) {
	searchVec := []float64{0, 1, 2}
	vecPool := newVecPoolGenerator([][]float64{
		{1, 5, 4}, // dist to SearchVec: ~4.582.
		{0, 3, 5}, // dist to SearchVec: ~3.605.
	})
	r, ok := KNNEuc(searchVec, vecPool, 1)
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

// Prefab, so re-using test case in TestKNNBrute.
func TestKFNEuc(t *testing.T) {
	searchVec := []float64{0, 1, 2}
	vecPool := newVecPoolGenerator([][]float64{
		{1, 5, 4}, // dist to SearchVec: ~4.582.
		{0, 3, 5}, // dist to SearchVec: ~3.605.
	})
	r, ok := KFNEuc(searchVec, vecPool, 1)
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

// Prefab, so re-using test case in TestKNNBrute.
func TestKNNCos(t *testing.T) {
	searchVec := []float64{0, 1, 2}
	vecPool := newVecPoolGenerator([][]float64{
		{1, 5, 4}, // dist to SearchVec: ~0.897
		{0, 3, 5}, // dist to SearchVec: ~0.997.
	})
	r, ok := KNNCos(searchVec, vecPool, 1)
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

// Prefab, so re-using test case in TestKNNBrute.
func TestKFNCos(t *testing.T) {
	searchVec := []float64{0, 1, 2}
	vecPool := newVecPoolGenerator([][]float64{
		{1, 5, 4}, // dist to SearchVec: ~0.897
		{0, 3, 5}, // dist to SearchVec: ~0.997.
	})
	r, ok := KFNCos(searchVec, vecPool, 1)
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
