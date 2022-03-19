package timex

import (
	"math/rand"
	"testing"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// tests that the latency tracker actually gives correct averages for
// a period of time. this is done in a synced way, i.e one goroutine.
func TestLatencyTrackerAverageCorrectness(t *testing.T) {
	maxChainLinkN := 100
	minChainLinkSize := time.Millisecond * 5
	lt := LatencyTracker{
		cfg: NewLatencyTrackerArgs{
			MaxChainLinkN:    maxChainLinkN,
			MinChainLinkSize: minChainLinkSize,
		},
	}
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

		stamp := time.Now()
		time.Sleep(waitTime)
		lt.Register(time.Now().Sub(stamp))
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
