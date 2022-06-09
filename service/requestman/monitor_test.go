package requestman

import (
	"context"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/crunchypi/ddrop/pkg/knnc"
	"github.com/crunchypi/ddrop/pkg/mathx"
	"github.com/crunchypi/ddrop/pkg/timex"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestMonItemAvgMergeKNNMonItem(t *testing.T) {
	kmi1 := knnMonItem{Latency: 1, AvgScore: 0.5, Satisfaction: 0.4}
	kmi2 := knnMonItem{Latency: 1, AvgScore: 0.0, Satisfaction: 0.0}

	kmia := KNNMonItemAvg{}
	kmia.mergeKNNMonItem(kmi1)
	kmia.mergeKNNMonItem(kmi2)
	kmia.AvgScoreNoFails = mathx.RoundF64(kmia.AvgScoreNoFails, 2)

	if kmia.N != 2 {
		t.Fatal("unexpected N:", kmia.N)
	}
	if kmia.NFailed != 1 {
		t.Fatal("unexpected NFailed:", kmia.NFailed)
	}
	if kmia.AvgLatency != (kmi1.Latency+kmi2.Latency)/2 {
		t.Fatal("unexpected AvgLatency:", kmia.AvgLatency)
	}
	if kmia.AvgScore != (kmi1.AvgScore+kmi2.AvgScore)/2 {
		t.Fatal("unexpected AvgScore:", kmia.AvgScore)
	}
	if kmia.AvgScoreNoFails != kmi1.AvgScore {
		t.Fatal("unexpected AvgScoreNoFails:", kmia.AvgScoreNoFails)
	}
	if kmia.AvgSatisfaction != (kmi1.Satisfaction+kmi2.Satisfaction)/2 {
		t.Fatal("unexpected AvgSatisfaction:", kmia.AvgSatisfaction)
	}
}

func TestMonItemAvgMergeKNNMonItemAvg(t *testing.T) {
	n := 10 // Must be even.

	knnMonItems := make([]knnMonItem, n)
	for i := 0; i < n; i++ {
		knnMonItems[i] = knnMonItem{
			Latency:      time.Millisecond * time.Duration(rand.Int63n(10)),
			AvgScore:     rand.Float64(),
			Satisfaction: rand.Float64(),
		}
	}

	kmiaExpect := KNNMonItemAvg{}
	for _, item := range knnMonItems {
		kmiaExpect.mergeKNNMonItem(item)
	}

	half1 := knnMonItems[:n]
	kmia1 := KNNMonItemAvg{}
	for _, item := range half1 {
		kmia1.mergeKNNMonItem(item)
	}

	half2 := knnMonItems[n:]
	kmia2 := KNNMonItemAvg{}
	for _, item := range half2 {
		kmia2.mergeKNNMonItem(item)
	}

	kmiaCombo := KNNMonItemAvg{}
	kmiaCombo.mergeKNNMonItemAvg(&kmia1)
	kmiaCombo.mergeKNNMonItemAvg(&kmia2)

	if kmiaCombo.Created != kmia1.Created {
		s := "unexpected time; want %v, have %v \n"
		t.Fatalf(s, kmia1.Created, kmiaCombo.Created)
	}

	if kmiaCombo.N != kmiaExpect.N {
		s := "unexpected N field val; want %v, got %v\n"
		t.Fatalf(s, kmiaExpect.N, kmiaCombo.N)
	}

	if kmiaCombo.NFailed != kmiaExpect.NFailed {
		s := "unexpected NFailed field val; want %v, got %v\n"
		t.Fatalf(s, kmiaExpect.NFailed, kmiaCombo.NFailed)
	}

	if kmiaCombo.AvgLatency != kmiaExpect.AvgLatency {
		s := "unexpected AvgLatency field val; want %v, got %v\n"
		t.Fatalf(s, kmiaExpect.AvgLatency, kmiaCombo.AvgLatency)
	}

	if kmiaCombo.AvgScore != kmiaExpect.AvgScore {
		s := "unexpected AvgScore field val; want %v, got %v\n"
		t.Fatalf(s, kmiaExpect.AvgScore, kmiaCombo.AvgScore)
	}

	if kmiaCombo.AvgScoreNoFails != kmiaExpect.AvgScoreNoFails {
		s := "unexpected AvgScoreNoFails field val; want %v, got %v\n"
		t.Fatalf(s, kmiaExpect.AvgScoreNoFails, kmiaCombo.AvgScoreNoFails)
	}

	if kmiaCombo.AvgSatisfaction != kmiaExpect.AvgSatisfaction {
		s := "unexpected AvgSatisfaction field val; want %v, got %v\n"
		t.Fatalf(s, kmiaExpect.AvgSatisfaction, kmiaCombo.AvgSatisfaction)
	}
}

func TestMonitorAveragePrecice(t *testing.T) {
	d := time.Millisecond * 100

	kmi1 := knnMonItem{Latency: 1, AvgScore: 0.0, Satisfaction: 0.0}
	kmi2 := knnMonItem{Latency: 1, AvgScore: 0.5, Satisfaction: 0.4}

	et, _ := timex.NewEventTracker[KNNMonItemAvg](
		timex.EventTrackerConfig{MaxN: 10, MinStep: d},
	)

	monitor := knnMonitor{et: *et}

	// ll layout, starting with head:  [kmi2]-[kmi1]-[kmi1x2]
	monitor.registerMonItem(kmi1)
	monitor.registerMonItem(kmi1)
	time.Sleep(d)
	monitor.registerMonItem(kmi1)
	time.Sleep(d)
	monitor.registerMonItem(kmi2)

	// All nodes.
	r := monitor.average(time.Hour)
	// Note, not checking all stats, that is done in one of the tests above,
	// related to KNNMonItemAvg.
	if r.N != 4 {
		t.Fatal("unexpected N field val:", r.N)
	}
	if r.AvgScore != 0.125 {
		t.Fatal("unexpected AvgScore field val:", r.AvgScore)
	}

	// 2/3 nodes in the linked list (so kmi1+kmi2) and take their average.
	r = monitor.average(d * 2)
	if r.N != 2 {
		t.Fatal("unexpected N field val:", r.N)
	}
	if r.AvgScore != 0.25 {
		t.Fatal("unexpected AvgScore field val:", r.AvgScore)
	}
}

func TestMonitorRegister(t *testing.T) {
	type enqResultDuo struct {
		raw KNNEnqueueResult // Normal
		mon KNNEnqueueResult // Returned from monitor.register
	}

	// Amount of (concurrent-ish) requests. Should be fairly high since
	// some of the test checks are probability based.
	n := 10_000
	// Measurement of time, expecially in knnMonitor.minChainlLinkSize
	d := time.Second

	testRuntime := d * 4
	testStarted := time.Now()
	testEnds := testStarted.Add(testRuntime)

	// KNNMonItemAvg.AverageScore should be somewhere in between.
	minScore := 0.4
	maxScore := 0.6

	// What is expected in a 'query'. Actual n 'results' will always be 1.
	// KNNMonItem.AvgSatisfaction should be approx min/(max+1).
	minK := 1
	maxK := 5

	startedNGoroutines := runtime.NumGoroutine()

	et, _ := timex.NewEventTracker[KNNMonItemAvg](
		timex.EventTrackerConfig{MaxN: 10, MinStep: d},
	)

	monitor := knnMonitor{et: *et}
	// Note; using channels here becase of their lazy nature.

	// Simulate hotspot for request creation, such as Handle.KNN.
	enqueueResultsDuo := make(chan enqResultDuo)
	go func() {
		defer close(enqueueResultsDuo)
		for i := 0; i < n; i++ {

			_, ctxCancel := context.WithCancel(context.Background())
			enqRNew := KNNEnqueueResult{
				Pipe:   make(chan knnc.ScoreItems, 1),
				Cancel: ctxCancel,
			}
			enqueueResultsDuo <- enqResultDuo{
				raw: enqRNew,
				mon: monitor.register(knnMonitorRegisterArgs{
					knnEnqueueResult: enqRNew,
					k:                rand.Intn(maxK-minK) + maxK,
					ttl:              testEnds.Sub(testStarted),
				}),
			}
		}
	}()

	// Simulate request processing.
	wg := sync.WaitGroup{}
	wg.Add(n)
	for enqRDuo := range enqueueResultsDuo {
		go func(enqRDuo enqResultDuo) {
			defer wg.Done()
			// Make all goroutines end at somewhere between now and testEnds.
			// Also guard non-positive integers in rand.Int63n, it'll be angry.
			testRemainder := testEnds.Sub(time.Now())
			if testRemainder > 0 {
				time.Sleep(time.Duration(rand.Int63n(int64(testRemainder))))
			}

			// Request done and sent.
			enqRDuo.raw.Pipe <- knnc.ScoreItems{
				{Set: true, Score: rand.Float64()*(maxScore-minScore) + minScore},
			}
			// Request received.
			<-enqRDuo.mon.Pipe
			close(enqRDuo.raw.Pipe)
		}(enqRDuo)
	}

	wg.Wait()
	// Make sure the whole linked list is filled.
	if len(monitor.et.Collect(time.Hour)) != int(testRuntime/d) {
		t.Log("unexpected ll len:", len(monitor.et.Collect(time.Hour)))
	}

	// Simple check; make sure that all entries are accounted for.
	r := monitor.average(testRuntime)
	if r.N != n {
		s := "some entries were unaccounted for. want %v, have %v"
		t.Fatalf(s, n, r.N)
	}

	// Complicated checks; get a portion of the timedLinkedList entries.
	// It is a bit non-deterministic, so using ranges for these checks.
	sampleSize := 0.5
	durationSpan := time.Duration(float64(testRuntime) * sampleSize)
	r = monitor.average(durationSpan)

	// Check number of entries for period.
	margin := 0.2
	targetN := int(float64(n) * sampleSize)
	targetNLower := int(float64(targetN) * (1 - margin))
	targetNUpper := int(float64(targetN) * (1 + margin))
	if r.N < targetNLower || r.N > targetNUpper {
		s := "KNNMonItemAvg.N is out of expected bounds; "
		s += "want min %v max %v, have %v. "
		s += "Might want to re-run test, as this is probability based."
		t.Fatalf(s, targetNLower, targetNUpper, r.N)
	}

	// Check score for period.
	if r.AvgScore < minScore || r.AvgScore > maxScore {
		s := "KNNMonItemAvg.AvgScore is out of expected bounds; "
		s += "want min %v max %v, have %v"
		t.Fatalf(s, minScore, maxScore, r.AvgScore)
	}

	// Check satisfaction for period.
	targetSat := float64(minK) / float64(maxK+1)
	targetSatLower := targetSat * (1 - margin)
	targetSatUpper := targetSat * (1 + margin)
	if r.AvgSatisfaction < targetSatLower || r.AvgSatisfaction > targetSatUpper {
		s := "KNNMonItemAvg.AvgSatisfaction is out of expected bounds; "
		s += "want min %v max %v, have %v"
		t.Fatalf(s, targetSatLower, targetSatUpper, r.AvgSatisfaction)
	}

	// Check leaks.
	runtime.GC()
	if runtime.NumGoroutine() > startedNGoroutines {
		s := "n goroutines is higher now than at the start of test;"
		s += "had %v, have %v. There may be a leak."
		t.Fatalf(s, startedNGoroutines, runtime.NumGoroutine())
	}
}
