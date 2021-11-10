package knn

import (
	"testing"
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
