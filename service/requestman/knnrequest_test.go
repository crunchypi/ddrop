package requestman

import (
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
