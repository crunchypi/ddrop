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

func TestKNNRequestToPipeline(t *testing.T) {
	r := newKNNRequest(&KNNArgs{
		//Namespace:,
		Priority:  1,
		QueryVec:  []float64{5},
		KNNMethod: KNNMethodEuclideanDistance,
		Ascending: true,
		K:         1,
		Extent:    1,
		Accept:    0,
		Reject:    5,
		TTL:       time.Second,
	})

	// Simulate faucet.
	chIn := make(chan knnc.ScanItem)
	go func() {
		chIn <- knnc.ScanItem{Distancer: mathx.NewSafeVec(2)} // dist to query = 3
		chIn <- knnc.ScanItem{Distancer: mathx.NewSafeVec(4)} // dist to query = 1
		chIn <- knnc.ScanItem{Distancer: mathx.NewSafeVec(3)} // dist to query = 2
		close(chIn)
	}()

	pipe, ok := r.toPipeline()
	if !ok {
		t.Fatal("failed setup of pipeline")
	}

	// Push faucet -> pipeline.
	if !pipe.AddScanner(chIn) {
		t.Fatal("pipe failed to add scanner")
	}
	go func() { pipe.WaitThenClose() }()

	result := make(knnc.ScoreItems, r.args.K)
	pipe.ConsumeIter(func(scoreItems knnc.ScoreItems) bool {
		for _, scoreItem := range scoreItems {
			result.BubbleInsert(scoreItem, r.args.Ascending)
		}
		return true
	})

	// KNNRequest.K = 1
	if trimmed := result.Trim(); len(trimmed) != 1 {
		t.Fatal("unexpected len:", len(trimmed))
	}
	if score := result[0].Score; score != 1 {
		t.Fatal("unexpected score:", score)
	}
}

func TestKNNRequestConsume(t *testing.T) {
	n := 1000
	dim := 3

	ss, _ := knnc.NewSearchSpaces(knnc.NewSearchSpacesArgs{
		SearchSpacesMaxCap:      n,
		SearchSpacesMaxN:        n,
		MaintenanceTaskInterval: 1,
	})

	for i := 0; i < n; i++ {
		v, _ := mathx.NewSafeVecRand(dim)
		ss.AddSearchable(&DistancerContainer{D: v})
	}

	r := newKNNRequest(&KNNArgs{
		Namespace: "",
		Priority:  1,
		QueryVec:  []float64{1, 1, 1},
		KNNMethod: KNNMethodEuclideanDistance,
		Ascending: true,
		K:         1,
		Extent:    1,
		Accept:    0,
		Reject:    5,
		TTL:       time.Second,
	})

	go r.consume(ss)

	// Doesn't check any correctness, just that something is found.
	// Correctness is checked with other funcs and the knnc pkg.
	set := false
	for scoreItems := range r.enqueueResult.Pipe {
		for _, scoreItem := range scoreItems {
			if !scoreItem.Set {
				continue
			}
			set = true
		}
	}

	if !set {
		t.Fatal("didnt get any result")
	}
}
