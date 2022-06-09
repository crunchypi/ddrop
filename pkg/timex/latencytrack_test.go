package timex

import (
	"math/rand"
	"sync"
	"testing"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Validate that len of chained link does not exceed the specified maximum,
// even though a duration entry is entered at a frequency that is smaller
// than the minimum duration between each link (which is measured in time).
func TestLatencyTrackerDenseWindow(t *testing.T) {
	n := 100
	maxChainLinkN := 10
	minChainLinkSize := time.Millisecond * 5

	lt, _ := NewLatencyTracker(
		EventTrackerConfig{
			MaxN:    maxChainLinkN,
			MinStep: minChainLinkSize,
		},
	)

	for i := 0; i < n; i++ {
		done := lt.RegisterCallback()
		// NOTE: half.
		time.Sleep(minChainLinkSize / 2)
		done()

		nLinks := len(lt.et.Collect(time.Duration(maxChainLinkN) * minChainLinkSize))
		if nLinks > maxChainLinkN {
			t.Fatalf("len of linked list exceeded. max: %v, have: %v",
				maxChainLinkN, nLinks)
		}
	}
}

// Validate that len of chained link does not exceed the specified maximum,
// even though a duration entry is entered at a frequency that is larger
// than the minimum duration between each link (which is measured in time).
func TestLatencyTrackerSparseWindow(t *testing.T) {
	n := 100
	maxChainLinkN := 10
	minChainLinkSize := time.Millisecond * 5

	lt, _ := NewLatencyTracker(
		EventTrackerConfig{
			MaxN:    maxChainLinkN,
			MinStep: minChainLinkSize,
		},
	)

	for i := 0; i < n; i++ {
		done := lt.RegisterCallback()
		// NOTE: double.
		time.Sleep(minChainLinkSize * 2)
		done()

		nLinks := len(lt.et.Collect(time.Duration(maxChainLinkN) * minChainLinkSize))
		if nLinks > maxChainLinkN {
			t.Fatalf("len of linked list exceeded. max: %v, have: %v",
				maxChainLinkN, nLinks)
		}
	}
}

// Validate that len of chained link does not exceed the specified maximum,
// even though a duration entry is entered at a frequency that is random
// than the minimum duration between each link (which is measured in time),
// and happens in a concurrent environment.
func TestLatencyTrackerFuzzedWindow(t *testing.T) {
	n := 100
	maxChainLinkN := 10
	minChainLinkSize := time.Millisecond * 5

	lt, _ := NewLatencyTracker(
		EventTrackerConfig{
			MaxN:    maxChainLinkN,
			MinStep: minChainLinkSize,
		},
	)

	// Used for preventing goroutines from doing anything before all of
	// them have started, for the purpose of factoring out the the startup
	// overhead time.
	wgStartline := sync.WaitGroup{}
	wgStartline.Add(n)

	// Used to know when individual goroutines are finished.
	goroutineFinished := make(chan bool)

	// Used to know when all goroutines are finished (for closing the chan above).
	wgFinishline := sync.WaitGroup{}
	wgFinishline.Add(n)

	for i := 0; i < n; i++ {
		go func(i int) {
			wgStartline.Done()
			wgStartline.Wait()

			done := lt.RegisterCallback()
			// Spread over limited time, with some randomness.
			maxWait := time.Duration(n-i) * minChainLinkSize
			time.Sleep(time.Duration(rand.Int63n(int64(maxWait))))

			done()

			wgFinishline.Done()
			goroutineFinished <- true
		}(i)
	}

	go func() { wgFinishline.Wait(); close(goroutineFinished) }()

	for <-goroutineFinished {
		// Control that the length of the chan is not exceeded.
		nLinks := len(lt.et.Collect(time.Duration(maxChainLinkN) * minChainLinkSize))
		if nLinks > maxChainLinkN {
			t.Fatalf("len of linked list exceeded. max: %v, have: %v",
				maxChainLinkN, nLinks)
		}
	}
}

// tests that the latency tracker actually gives correct averages for
// a period of time. this is done in a synced way, i.e one goroutine.
func TestLatencyTrackerAverageCorrectness(t *testing.T) {
	maxChainLinkN := 100
	minChainLinkSize := time.Millisecond * 5

	lt, _ := NewLatencyTracker(
		EventTrackerConfig{
			MaxN:    maxChainLinkN,
			MinStep: minChainLinkSize,
		},
	)

	// Should be fairly high, unless the two variables above are higher (and
	// test is longer), since there is a lot of measurement overhead.
	errMargin := minChainLinkSize / 10

	var actualWait time.Duration

	// Potential for filling the whole chain length capacity.
	for i := 0; i < maxChainLinkN; i++ {

		// Random amount of time such that there are on average two
		// waiting 'processes' entries per chain link. This should
		// fill only approximately half of the linked list capacity.
		waitTime := time.Duration(rand.Int63n(int64(minChainLinkSize)))
		actualWait += waitTime

		done := lt.RegisterCallback()
		time.Sleep(waitTime)
		done()
	}

	actualAverage := actualWait / time.Duration(maxChainLinkN)
	estimatedAverage, _ := lt.Average(time.Duration(maxChainLinkN) * minChainLinkSize)
	diff := actualAverage - estimatedAverage
	// Absolute.
	if diff < 0 {
		diff = diff * -1
	}

	withinMargin := diff < errMargin
	if !withinMargin {
		t.Fatalf("fail. actual: %v, estimate: %v", actualAverage, estimatedAverage)
	}
}

// Basically same as the TestLatencyTrackerAverageCorrectness test, just in a
// concurrent environment.
func TestLatencyTrackerAverageCorrectnessFuzzed(t *testing.T) {
	maxChainLinkN := 10
	minChainLinkSize := time.Millisecond * 5

	lt, _ := NewLatencyTracker(
		EventTrackerConfig{
			MaxN:    maxChainLinkN,
			MinStep: minChainLinkSize,
		},
	)

	errMargin := minChainLinkSize / 20

	// Used for preventing goroutines from doing anything before all of
	// them have started, for the purpose of factoring out the the startup
	// overhead time.
	nGoroutines := 1000
	wgStartline := sync.WaitGroup{}
	wgStartline.Add(nGoroutines)

	// Used to know when all goroutines are finished.
	wgFinishline := sync.WaitGroup{}
	wgFinishline.Add(nGoroutines)

	actualWaitChan := make(chan time.Duration, nGoroutines)

	for i := 0; i < nGoroutines; i++ {
		go func(i int) {
			// Sync up all goroutines.
			wgStartline.Done()
			wgStartline.Wait()

			defer wgFinishline.Done()

			// Spread goroutines over half the linked list capacity.
			if i > 0 { // Guard div by zero.
				max := time.Duration(maxChainLinkN) * minChainLinkSize / 2
				step := max / time.Duration(nGoroutines)
				time.Sleep(step * time.Duration(i))
			}

			// Wait on average half a link size.
			waitTime := time.Duration(rand.Int63n(int64(minChainLinkSize)))
			//waitTime := minChainLinkSize
			actualWaitChan <- waitTime

			done := lt.RegisterCallback()
			time.Sleep(waitTime)
			done()
		}(i)
	}

	go func() { wgFinishline.Wait(); close(actualWaitChan) }()

	// Collect.
	var actualWait time.Duration
	for actualWaitItem := range actualWaitChan {
		actualWait += actualWaitItem
	}

	// Check diff.
	actualAverage := actualWait / time.Duration(nGoroutines)
	estimatedAverage, _ := lt.Average(time.Duration(maxChainLinkN) * minChainLinkSize)
	diff := actualAverage - estimatedAverage
	// Absolute.
	if diff < 0 {
		diff = diff * -1
	}

	withinMargin := diff < errMargin
	if !withinMargin {
		t.Fatalf("fail. actual: %v, estimate: %v", actualAverage, estimatedAverage)
	}
}
