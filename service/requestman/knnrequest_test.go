package requestman

import (
	"fmt"
	"testing"
	"time"

	"github.com/crunchypi/ddrop/pkg/knnc"
	"github.com/crunchypi/ddrop/pkg/mathx"
)

/*
--------------------------------------------------------------------------------
Testing basics, i.e all the KNNRequest.toXYZ funcs that interact with the knnc
pkg. As such, some of these tests will depend on the correct knnc functionality.
--------------------------------------------------------------------------------
*/

func TestKNNRequestToMapFunc(t *testing.T) {
	r := newKNNRequest(&KNNArgs{
		QueryVec:  []float64{1, 1},
		KNNMethod: KNNMethodEuclideanDistance,
	})

	score, _ := r.toMapFunc()(mathx.NewSafeVec(1, 2))
	if score.Score != 1 {
		t.Fatal("unexpected score (Euclidean):", score)
	}

	r.args.KNNMethod = KNNMethodCosineSimilarity
	score, _ = r.toMapFunc()(mathx.NewSafeVec(1, 3))
	if mathx.RoundF64(score.Score, 2) != .89 {
		t.Fatal("unexpected score (cosine):", score)
	}
}

func TestKNNRequestToMapStage(t *testing.T) {

	r := newKNNRequest(&KNNArgs{
		QueryVec:  []float64{1, 1},
		KNNMethod: KNNMethodEuclideanDistance,
		TTL:       time.Second,
		Priority:  1,
	})

	chI := make(chan knnc.ScanItem)
	chO, ok := r.toMapStage()(chI)
	if !ok {
		t.Fatal("failed starting map stage")
	}

	go func() {
		chI <- knnc.ScanItem{Distancer: mathx.NewSafeVec(1, 2)}
		close(chI)
	}()

	score := 0.
	for scoreItem := range chO {
		score += scoreItem.Score
	}

	if score == 0 {
		t.Fatal("suspecting unset score")
	}

	if score != 1 {
		t.Fatal("unexpected score:", score)
	}
}

func TestKNNRequestToFilterFunc(t *testing.T) {
	r := newKNNRequest(&KNNArgs{
		Reject:    1.,
		Ascending: true,
	})

	// Convenience.
	fmtcfg := func() string {
		return fmt.Sprintf("reject=%v, acending=%v",
			r.args.Reject, r.args.Ascending)
	}

	// true = keep
	if r.toFilterFunc()(knnc.ScoreItem{Score: 2}) {
		t.Fatalf("cfg: '%v', dropped score=0", fmtcfg())
	}

	if !r.toFilterFunc()(knnc.ScoreItem{Score: 0}) {
		t.Fatalf("cfg: '%v', kept score=0", fmtcfg())
	}

	// flip the significanse of scores.
	r.args.Ascending = false
	if !r.toFilterFunc()(knnc.ScoreItem{Score: 2}) {
		t.Fatalf("cfg: '%v', dropped score=2", fmtcfg())
	}

	if r.toFilterFunc()(knnc.ScoreItem{Score: 0}) {
		t.Fatalf("cfg: '%v', kept score=0", fmtcfg())
	}
}

func TestKNNRequestToFilterStage(t *testing.T) {
	r := newKNNRequest(&KNNArgs{
		TTL:       time.Second,
		Priority:  1,
		Reject:    2,
		Ascending: true,
	})

	chI := make(chan knnc.ScoreItem)
	chO, ok := r.toFilterStage()(chI)
	if !ok {
		t.Fatal("failed starting filter stage")
	}

	go func() {
		// On either side of qi.request.Reject.
		chI <- knnc.ScoreItem{Score: 1}
		chI <- knnc.ScoreItem{Score: 3}
		close(chI)
	}()

	score := 0.
	for scoreItem := range chO {
		score += scoreItem.Score
	}

	if score == 0 {
		t.Fatal("suspecting unset score")
	}

	if score != 1 {
		t.Fatal("unexpected score:", score)
	}
}

func TestKNNRequestToMergeStage(t *testing.T) {
	r := newKNNRequest(&KNNArgs{
		TTL:       time.Second,
		Priority:  1,
		Ascending: true,
		K:         1,
	})

	chI := make(chan knnc.ScoreItem)
	chO, ok := r.toMergeStage()(chI)
	if !ok {
		t.Fatal("failed starting merge stage")
	}

	go func() {
		chI <- knnc.ScoreItem{Score: 3, Set: true} // Reject since k=1.
		chI <- knnc.ScoreItem{Score: 1, Set: true} // Best.
		close(chI)
	}()

	score := 0.
	for scoreItems := range chO {
		for _, scoreItem := range scoreItems {
			if !scoreItem.Set {
				continue
			}
			score += scoreItem.Score
		}
	}

	if score == 0 {
		t.Fatal("suspecting unset score")
	}

	if score != 1 {
		t.Fatal("unexpected score:", score)
	}
}
