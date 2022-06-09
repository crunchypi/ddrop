package timex

/*
File contains a latency tracker, of which the primary purpose is to track how
much (average) latency some processes have during some time window.
It is a linked list where each link/node represents some discrete time, and
store data about cumulative latency and amount of additions (for average calc).
*/

import (
	"sync"
	"time"
)

// LatencyTracker is intended to track average latency for some processes during
// certain (moving) windows of time, e.g how much latency a server response has
// had the last minute. Note, must be set up with NewLatencyTracker(...).
type LatencyTracker struct {
	sync.RWMutex
	et EventTracker[time.Duration]
}

// NewLatencyTracker sets up- and returns (*LatencyTracker, true) if
// args.Ok() == true. Else, it returns (nil, false).
func NewLatencyTracker(cfg EventTrackerConfig) (*LatencyTracker, bool) {
	return &LatencyTracker{et: EventTracker[time.Duration]{cfg: cfg}}, cfg.Ok()
}

// Cfg returns the internal configuration which was used when creating this
// instance with NewLatencyTracker(...).
func (lt *LatencyTracker) Cfg() EventTrackerConfig {
	lt.Lock()
	defer lt.Unlock()
	return lt.et.cfg
}

// Register registers some latency. Specifically, it adds the delta to a node
// that is closest to time.Now(), which might be a node that is created here.
// Additionally, it trims off the old tail(s).
func (lt *LatencyTracker) Register(delta time.Duration) {
	lt.Lock()
	defer lt.Unlock()

	lt.et.Register(func(created time.Time, n int, d time.Duration) time.Duration {
		return d + delta
	})
}

// RegisterCallback is a convenience method around LatencyTracker.Register(...).
// It returns a callback that, when invoked, will automatically call the Register
// method with the delta between now and then.
//
// For instance, calling "defer lt.RegisterCallback()" at the start of a func f,
// will register the whole execution time of f.
func (lt *LatencyTracker) RegisterCallback() func() {
	then := time.Now()
	return func() {
		lt.Register(time.Now().Sub(then))
	}
}

// Average gives the average latency for the last period. e.g if 'period' is
// 1 min, then it will return the average latency for the last minute.
//
// Will return false if that period exceeds the internal tracker, which is
// (min link size) * (max amount of links), as specified with the argument
// given when creating this instance with NewLatencyTrackerArgs.
func (lt *LatencyTracker) Average(period time.Duration) (time.Duration, bool) {
	stamp := time.Now()
	lt.RLock()
	defer lt.RUnlock()

	maxDuration := lt.et.cfg.MinStep * time.Duration(lt.et.cfg.MaxN)
	withinBounds := period <= maxDuration

	var cumulativeWait time.Duration
	var nWaiters int

	lt.et.Iter(func(i int, item *EventTrackerItem[time.Duration]) bool {
		include := stamp.Sub(item.Created) <= period
		// Don't include out of duration bounds.
		if include {
			cumulativeWait += item.Payload
			nWaiters += item.N
		}
		return include

	})

	// Guard zero div.
	if nWaiters == 0 {
		return 0, withinBounds
	}

	average := cumulativeWait / time.Duration(nWaiters)
	return average, withinBounds

}
