package knnc

import (
	"math/rand"
	"testing"
	"time"

	"github.com/crunchypi/ddrop/pkg/mathx"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestMapStage(t *testing.T) {
	// input data.
	queryVec := newTVec(0, 1, 2)
	otherVecs := []*tVec{
		newTVec(1, 5, 4), // Euclidean dist to qv: 4.5826
		newTVec(0, 3, 5), // Euclidean dist to qv: 3.6056
	}

	// Simulate scanner.
	chFaucet := make(chan ScanItem)
	go func() {
		defer close(chFaucet)
		for _, v := range otherVecs {
			chFaucet <- ScanItem{Distancer: v}
		}
	}()

	// Run stage.
	chOut, ok := MapStage(MapStageArgs{
		In: chFaucet,
		// Note Euclidean distance.
		MapFunc: func(d Distancer) (ScoreItem, bool) {
			score, ok := d.EuclideanDistance(queryVec)
			// Field 'set' is handled inside stage, so omitted here.
			return ScoreItem{ID: "", Score: score}, ok
		},
		BaseStageArgs: BaseStageArgs{
			NWorkers: 100,
			BaseWorkerArgs: BaseWorkerArgs{
				Buf:           100,
				Cancel:        NewCancelSignal(),
				BlockDeadline: time.Second * 10,
			},
		},
	})

	if !ok {
		t.Fatal("args validation check failed; test impl error")
	}

	// Validate.
	for scoreItem := range chOut {
		// Not ideal check but the order is not deterministic.
		scoreItem.Score = mathx.RoundF64(scoreItem.Score, 4)
		if scoreItem.Score != 4.5826 && scoreItem.Score != 3.6056 {
			t.Fatalf("unexpected score: %v", scoreItem.Score)
		}
	}
}

func TestFilterStage(t *testing.T) {
	// Input data.
	scores := []ScoreItem{
		{Score: 5, set: true},
		{Score: 3, set: true},
		{Score: 1, set: true},
		{Score: 9, set: true},
	}

	dontFilter := scores[len(scores)-1] // What not to filter out.

	// Simulate previous (intended as mapping) stage
	chFaucet := make(chan ScoreItem)
	go func() {
		defer close(chFaucet)
		for _, v := range scores {
			chFaucet <- v
		}
	}()

	// Run stage.
	chOut, ok := FilterStage(FilterStageArgs{
		In: chFaucet,
		// Note that everything besides 'dontFilter' is filtered.
		FilterFunc: func(scoreItem ScoreItem) bool {
			return scoreItem.Score == dontFilter.Score
		},
		BaseStageArgs: BaseStageArgs{
			NWorkers: 100,
			BaseWorkerArgs: BaseWorkerArgs{
				Buf:           100,
				Cancel:        NewCancelSignal(),
				BlockDeadline: time.Second * 5,
			},
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
