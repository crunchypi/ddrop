package knnc

import (
	"math/rand"
	"testing"
	"time"
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

func commonTestingCodeBaseStageArgs() BaseStageArgs {
	return BaseStageArgs{
		NWorkers: 100,
		BaseWorkerArgs: BaseWorkerArgs{
			Buf:    50,
			Cancel: NewCancelSignal(),
			TTL:    time.Second * 3,
		},
	}
}

func commonTestingCodeRawScanItemFaucet(vecs []*tVec) <-chan ScanItem {
	out := make(chan ScanItem)
	go func() {
		defer close(out)
		for _, v := range vecs {
			out <- ScanItem{Distancer: v}
		}
	}()
	return out
}

func commonTestingCodeRawScoreItemFaucet(scoreItems []ScoreItem) <-chan ScoreItem {
	out := make(chan ScoreItem)
	go func() {
		defer close(out)
		for _, scoreItem := range scoreItems {
			out <- scoreItem
		}
	}()

	return out
}

/*
--------------------------------------------------------------------------------
Tests below.
--------------------------------------------------------------------------------
*/

func TestMapStage(t *testing.T) {
	// input data.
	queryVec := newTVec(0)
	chFaucet := commonTestingCodeRawScanItemFaucet([]*tVec{
		newTVec(1), // Euclidean dist to qv: 1
		newTVec(2), // Euclidean dist to qv: 2
	})

	// Run stage.
	chOut, ok := MapStage(MapStageArgs{
		In: chFaucet,
		// Note Euclidean distance.
		MapStagePartialArgs: MapStagePartialArgs{
			MapFunc: func(d Distancer) (ScoreItem, bool) {
				score, ok := d.EuclideanDistance(queryVec)
				// Field 'set' is handled inside stage, so omitted here.
				return ScoreItem{Score: score}, ok
			},
			BaseStageArgs: commonTestingCodeBaseStageArgs(),
		},
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
		// Simulate previous (intended as mapping) stage
		In: commonTestingCodeRawScoreItemFaucet(scores),
		FilterStagePartialArgs: FilterStagePartialArgs{
			// Note that everything besides 'dontFilter' is filtered.
			FilterFunc: func(scoreItem ScoreItem) bool {
				return scoreItem.Score == dontFilter.Score
			},
			BaseStageArgs: commonTestingCodeBaseStageArgs(),
		},
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
	n := 100_000
	k := 2
	ascending := true

	// Input data.
	scores := make([]ScoreItem, n)
	for i := 0; i < n; i++ {
		scores[i] = ScoreItem{Score: float64(i), Set: true}
	}

	// Shuffle.
	for i := 0; i < n; i++ {
		j := rand.Intn(n)
		scores[i], scores[j] = scores[j], scores[i]
	}

	ch, ok := MergeStage(MergeStageArgs{
		// Simulate previous (intended as mapping) stage, using the input data.
		In: commonTestingCodeRawScoreItemFaucet(scores),
		MergeStagePartialArgs: MergeStagePartialArgs{
			K:             k,
			Ascending:     ascending,
			SendInterval:  2,
			BaseStageArgs: commonTestingCodeBaseStageArgs(),
		},
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
	n := 100_000
	k := 2
	ascending := false

	// Input data.
	scores := make([]ScoreItem, n)
	for i := 0; i < n; i++ {
		scores[i] = ScoreItem{Score: float64(i), Set: true}
	}

	// Shuffle.
	for i := 0; i < n; i++ {
		j := rand.Intn(n)
		scores[i], scores[j] = scores[j], scores[i]
	}

	ch, ok := MergeStage(MergeStageArgs{
		// Simulate previous (intended as mapping) stage, using the input data.
		In: commonTestingCodeRawScoreItemFaucet(scores),
		MergeStagePartialArgs: MergeStagePartialArgs{
			K:             k,
			Ascending:     ascending,
			SendInterval:  2,
			BaseStageArgs: commonTestingCodeBaseStageArgs(),
		},
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
		t.Fatal("unexpected len of resulting scoreitems slice:", len(scoreItems))
	}

	a := scoreItems[0].Score
	b := scoreItems[1].Score
	if (a != float64(n-1) && b != float64(n-2)) || a == b {
		t.Fatal("unexpected result in scoreitems slice:", scoreItems)
	}
}
