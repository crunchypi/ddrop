package knnc

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/crunchypi/ddrop/pkg/syncx"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

/*
--------------------------------------------------------------------------------
This test file covers stages.go and havs some common setup code in each test
func. As such, some common code is factored out and put here at the top.
--------------------------------------------------------------------------------
*/

func testUtilCommonStageArgsPartial() syncx.StageArgsPartial {
	return syncx.StageArgsPartial{
		Ctx: context.Background(),
		TTL: time.Second * 3,
		Buf: 50,
	}
}

// Note; not safe.
func testUtilRange(start, end, stride int) []int {
	s := make([]int, 0, (end-start)/stride)
	for i := start; i < end; i += stride {
		s = append(s, i)
	}

	return s
}

func testUtilMap[T, U any](s []T, f func(T) U) []U {
	r := make([]U, len(s))
	for i, v := range s {
		r[i] = f(v)
	}
	return r
}

func testUtilShuffled[T any](s []T) []T {
	r := make([]T, len(s))
	copy(r, s)
	for i := 0; i < len(s); i++ {
		j := rand.Intn(len(s))
		r[i], r[j] = r[j], r[i]
	}

	return r
}

/*
--------------------------------------------------------------------------------
Tests below.
--------------------------------------------------------------------------------
*/

func TestMapStage(t *testing.T) {
	// input data.
	queryVec := newTVec(0)
	chIn := syncx.ChanFromSlice([]ScanItem{
		{newTVec(1)}, // Euclidean dist to qv: 1
		{newTVec(2)}, // Euclidean dist to qv: 2
	})

	// Run stage.
	chOut, ok := MapStage(MapStageArgs{
		NWorkers: 3,
		In:       chIn,
		// Note Euclidean distance.
		MapFunc: func(d Distancer) (ScoreItem, bool) {
			score, ok := d.EuclideanDistance(queryVec)
			// Field 'set' is handled inside stage, so omitted here.
			return ScoreItem{Score: score}, ok
		},
		StageArgsPartial: testUtilCommonStageArgsPartial(),
	})

	if !ok {
		t.Fatal("args validation check failed; test impl error")
	}

	// Validate.
	for scoreItem := range chOut {
		// Not ideal check but the order is not deterministic.
		if scoreItem.Score != 1. && scoreItem.Score != 2. {
			t.Fatalf("unexpected score: %v", scoreItem.Score)
		}
	}
}

func TestFilterStage(t *testing.T) {
	// Input data.
	scores := []ScoreItem{
		{Score: 5, Set: true},
		{Score: 3, Set: true},
		{Score: 1, Set: true},
		{Score: 9, Set: true},
	}

	dontFilter := scores[len(scores)-1] // What not to filter out.

	// Run stage.
	chOut, ok := FilterStage(FilterStageArgs{
		NWorkers: 3,
		// Simulate previous (intended as mapping) stage
		In: syncx.ChanFromSlice(scores),
		// Note that everything besides 'dontFilter' is filtered.
		FilterFunc: func(scoreItem ScoreItem) bool {
			return scoreItem.Score == dontFilter.Score
		},
		StageArgsPartial: testUtilCommonStageArgsPartial(),
	})

	if !ok {
		t.Fatal("args validation check failed; test impl error")
	}

	// Validate.
	for scoreItem := range chOut {
		if scoreItem.Score != dontFilter.Score {
			t.Fatalf("unexpected item with score %v", scoreItem.Score)
		}
	}
}

func TestMergeStageAscending(t *testing.T) {
	n := 1000
	k := 2
	ascending := true

	// Input data.
	scores := testUtilMap(testUtilRange(0, n, 1), func(i int) ScoreItem {
		return ScoreItem{Score: float64(i), Set: true}
	})
	scores = testUtilShuffled(scores)

	ch, ok := MergeStage(MergeStageArgs{
		// Simulate previous (intended as mapping) stage, using the input data.
		In:               syncx.ChanFromSlice(scores),
		K:                k,
		Ascending:        ascending,
		SendInterval:     2,
		StageArgsPartial: testUtilCommonStageArgsPartial(),
	})

	if !ok {
		t.Fatal("args validation check failed; test impl error")
	}

	scoreItems := make(ScoreItems, k)
	for scoreItemsTemp := range ch {
		for _, scoreItem := range scoreItemsTemp {
			scoreItems.BubbleInsert(scoreItem, ascending)
		}
	}

	if len(scoreItems) != k {
		t.Fatal("unexpected len of resulting scoreitems slice", len(scoreItems))
	}

	a := scoreItems[0].Score
	b := scoreItems[1].Score
	if (a != 0 && b != 1) || a == b {
		t.Fatal("unexpected result in scoreitems slice:", scoreItems)
	}
}

func TestMergeStageDescending(t *testing.T) {
	n := 1000
	k := 2
	ascending := false

	// Input data.
	scores := testUtilMap(testUtilRange(0, n, 1), func(i int) ScoreItem {
		return ScoreItem{Score: float64(i), Set: true}
	})
	scores = testUtilShuffled(scores)

	ch, ok := MergeStage(MergeStageArgs{
		// Simulate previous (intended as mapping) stage, using the input data.
		In:               syncx.ChanFromSlice(scores),
		K:                k,
		Ascending:        ascending,
		SendInterval:     2,
		StageArgsPartial: testUtilCommonStageArgsPartial(),
	})

	if !ok {
		t.Fatal("args validation check failed; test impl error")
	}

	scoreItems := make(ScoreItems, k)
	for scoreItemsTemp := range ch {
		for _, scoreItem := range scoreItemsTemp {
			scoreItems.BubbleInsert(scoreItem, ascending)
		}
	}

	if len(scoreItems) != k {
		t.Fatal("unexpected len of resulting scoreitems slice", len(scoreItems))
	}

	a := scoreItems[k-1].Score
	b := scoreItems[k-2].Score
	if (a != float64(n-2) && b != float64(n-1)) || a == b {
		t.Fatal("unexpected result in scoreitems slice:", scoreItems)
	}
}
