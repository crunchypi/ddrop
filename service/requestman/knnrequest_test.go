package requestman

import (
	"fmt"
    "math/rand"
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

/*
--------------------------------------------------------------------------------
Testing parameter tweaking. Some parameters/configs of KNNArgs are related to
optimization and/or trading accuracy for speed. The tests below validate that
(only the time component) by doing several runs while tweaking singular params
(one param per test case/func), which is is conceptualised as a time series (ish)
with a slope. The logic is that the (average) slope is negative if tweaks work.
--------------------------------------------------------------------------------
*/
type testTimeSlopeArgs struct {
	// Test data group.
	poolSize int // Number of vecs in pool.
	poolDim  int // Dimension of each vec in pool.

	// Func behaviour group.
	n int                  // Number of test iterations.
	m int                  // Number of test iteration re-runs (evending out scores).
	f func(i int) *KNNArgs // Func for getting KNNRequest. i is the curent iter of n.

	// Note: f is called with changing i such that the KNNRequst can change in
	// a way such that a query is faster/slower. m is just for evening out timestamps.
}

var scoreDump float64 // Just so the compiler won't do unwanted optimization.
// Helper func intended for timeing queries while tweaking paramaters.
// The purpose if to validate that some (query) param changes do indeed speed
// up / slow down query times (accuracy is not checked). This is timed here and
// the slope is returned.
func testTimeSlope(args testTimeSlopeArgs) (time.Duration, bool) {
	durations := make([]time.Duration, 0, args.n)

	// Create data.
	ss, _ := knnc.NewSearchSpaces(knnc.NewSearchSpacesArgs{
		SearchSpacesMaxCap:      args.poolSize,
		SearchSpacesMaxN:        args.poolSize,
		MaintenanceTaskInterval: 1,
	})

	for i := 0; i < args.poolSize; i++ {
		v, _ := mathx.NewSafeVecRand(args.poolDim)
		ss.AddSearchable(&DistancerContainer{D: v})
	}

	// Loop for adjusting param.
	for i := 0; i < args.n; i++ {
		// Loop for average.
		totalDuration := time.Duration(0)
		for j := 0; j < args.m; j++ {
			// Has to be in inner loop because the 'KNNEnqueueResult'
			// instance cannot be re-used be design.
			//request := args.f(i)
			request := newKNNRequest(args.f(i))
			if !request.Ok() {
				return 0, false
			}

			stamp := time.Now()

			// Query result might not have the requested k. Most correct would be
			// to check "k == qi.request.K", but randomness is involved in pool
			// creation (and the request from 'f(i)' has some freedom..). So leaving
			// this as it is, at least it validates that the request went through.
			go request.consume(ss)
			if len((<-request.enqueueResult.Pipe).Trim()) == 0 {
				return 0, false
			}

			delta := time.Now().Sub(stamp)
			totalDuration += delta
		}

		averageDuration := totalDuration / time.Duration(args.m)
		durations = append(durations, averageDuration)
	}

	// This method of calculation takes the average descent/ascent between
	// each step as i approaches n. This is opposed to taking the average
	// slope. There's no particular reason (the result is different but the
	// negative/positive property is equivalent for this purpose), but it
	// has the benefit of giving information about how discrete changes in i
	// affects the performance (on average).
	relativeDeltaSum := time.Duration(0)
	for x := 0; x < len(durations)-1; x++ {
		relativeDeltaSum += durations[x+1] - durations[x]
	}
	// -1 because it is relative deltas. 0 div potential but keeping it for
	// simplicity, it's for testing anyway.
	relativeDeltaAverage := relativeDeltaSum / (time.Duration(len(durations)) - 1)
	return relativeDeltaAverage, true
}

func randFloat64Slice(dim int) ([]float64, bool) {
	if dim <= 0 {
		return nil, false
	}

	s := make([]float64, dim)
	for i := 0; i < dim; i++ {
		s[i] = rand.Float64()
	}

	return s, true
}

// Increases KNNRequest (search) 'priority' for each step, which should make
// query faster because 'priority'=num of goroutines per stage.
func TestTimeSlopePriority(t *testing.T) {
	poolSize := 100_000
	poolDim := 3

	n := 5
	m := 10

	slope, ok := testTimeSlope(testTimeSlopeArgs{
		poolSize: poolSize,
		poolDim:  poolDim,
		n:        n,
		m:        m,
		f: func(i int) *KNNArgs {
			v, _ := randFloat64Slice(poolDim)

			return &KNNArgs{
				Namespace: "",
				Priority:  i + 1, // +1 because 0 priority is invalid.
				QueryVec:  v,
				KNNMethod: KNNMethodCosineSimilarity,
				Ascending: false,
				K:         3,
				Extent:    1,
				Accept:    1,
				Reject:    0,
				TTL:       time.Minute,
			}
		},
	})

	if !ok {
		t.Fatal("timeSlope func returned false, test is broken")
	}
	if slope > 0 {
		t.Fatal("positive slope, implying increase in time per step.")
	}
}

// Decreases KNNRequest (search) 'extent' for each step, which should make
// query faster because a lower and lower amount of vec pool is checked.
func TestTimeSlopeExtent(t *testing.T) {
	poolSize := 2_000
	poolDim := 3

	n := 100
	m := 10

	slope, ok := testTimeSlope(testTimeSlopeArgs{
		poolSize: poolSize,
		poolDim:  poolDim,
		n:        n,
		m:        m,
		f: func(i int) *KNNArgs {
			v, _ := randFloat64Slice(poolDim)

			// Increasing by n. Add constant to avoid 0 (KNNRequest.Ok() must pass).
			step := (1. / float64(n) * float64(i)) + 0.000000001
			// Decreasing by n.
			extent := 1 - step

			return &KNNArgs{
				Namespace: "",
				Priority:  1,
				QueryVec:  v,
				KNNMethod: KNNMethodCosineSimilarity,
				Ascending: false,
				K:         3,
				Extent:    extent,
				Accept:    1,
				Reject:    0,
				TTL:       time.Minute,
			}
		},
	})

	if !ok {
		t.Fatal("timeSlope func returned false, test is broken")
	}
	if slope > 0 {
		t.Fatal("positive slope, implying increase in time per step.")
	}
}

// Decreases KNNRequest 'accepted' scores for each step. With Cosine similarity,
// this means that worse and worse scores are accepted as a tradeoff for speed.
// This func validates this behaviour.
func TestTimeSlopeAccept(t *testing.T) {
	poolSize := 2_000
	poolDim := 3

	n := 100
	m := 10

	slope, ok := testTimeSlope(testTimeSlopeArgs{
		poolSize: poolSize,
		poolDim:  poolDim,
		n:        n,
		m:        m,
		f: func(i int) *KNNArgs {
			v, _ := randFloat64Slice(poolDim)

			// Increasing by n. Add constant to avoid 0 (KNNRequest.Ok() must pass).
			step := (1. / float64(n) * float64(i)) + 0.000000001
			// Decreasing by n.
			accept := 1 - step

			return &KNNArgs{
				Namespace: "",
				Priority:  1,
				QueryVec:  v,
				KNNMethod: KNNMethodCosineSimilarity,
				Ascending: false,
				K:         3,
				Extent:    1,
				Accept:    accept,
				Reject:    0,
				TTL:       time.Minute,
			}
		},
	})

	if !ok {
		t.Fatal("timeSlope func returned false, test is broken")
	}
	if slope > 0 {
		t.Fatal("positive slope, implying increase in time per step.")
	}
}

// Increases KNNRequest 'rejected' scores for each step. With Cosine similarity,
// this means that more and more scores are excluded from the 'merge' stage of
// the knn query, where scores are compared for the knn part. As such, the query
// should get faster as a benefit, this func validates this behavior.
func TestTimeSlopeReject(t *testing.T) {
	poolSize := 2_000
	poolDim := 3

	n := 100
	m := 10

	slope, ok := testTimeSlope(testTimeSlopeArgs{
		poolSize: poolSize,
		poolDim:  poolDim,
		n:        n,
		m:        m,
		f: func(i int) *KNNArgs {
			v, _ := randFloat64Slice(poolDim)

			// Increasing by n. Add constant to avoid 0 (KNNRequest.Ok() must pass).
			step := (1. / float64(n) * float64(i)) + 0.000000001

			return &KNNArgs{
				Namespace: "",
				Priority:  1,
				QueryVec:  v,
				KNNMethod: KNNMethodCosineSimilarity,
				Ascending: false,
				K:         3,
				Extent:    1,
				Accept:    1,
				Reject:    step,
				TTL:       time.Minute,
			}
		},
	})

	if !ok {
		t.Fatal("timeSlope func returned false, test is broken")
	}
	if slope > 0 {
		t.Fatal("positive slope, implying increase in time per step.")
	}
}
