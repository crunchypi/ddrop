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

// latencyTrackerItem is used as node in the LatencyTracker linked list.
type latencyTrackerItem struct {
	created time.Time
	// Layout: further from head = further back in time.
	next *latencyTrackerItem

	cumulativeLatency time.Duration // All the registered durations.
	nWaiters          int           // Amount of registers.
}

// LatencyTracker is intended to track average latency for some processes during
// certain (moving) windows of time, e.g how much latency a server response has
// had the last minute. Note, must be set up with NewLatencyTracker(...).
type LatencyTracker struct {
	sync.RWMutex
	head *latencyTrackerItem

	cfg NewLatencyTrackerArgs
}

// NewLatencyTrackerArgs is to be used for the NewLatencyTracker func.
type NewLatencyTrackerArgs struct {
	// MaxChainLinkN specifies the max amount of links in this chain.
	// When the number of links exceeds this, and new links are added,
	// then old ones (at the end/tail) are dropped.
	MaxChainLinkN int
	// MinChainLinkSize represents the min time delta between any link.
	// A latency tracker tracks latency during a time frame, so link
	// sizes are measured as a time.Duration.
	MinChainLinkSize time.Duration
	// StandardPeriod is meant for consistency. The method LatencyTracker.Average
	// accepts a time.Duration, but there are cases where this arg shouldn't
	// change. As such, LatencyTracker.AverageSTD can be called, which uses
	// this field val instead.
	StandardPeriod time.Duration
}

// Ok returns true if the instance was set up correctly. Specifically:
//	args.MaxChainLinkN > 0
//	args.MinChainLinkSize > 0
func (args *NewLatencyTrackerArgs) Ok() bool {
	ok := true
	ok = ok && args.MaxChainLinkN > 0
	ok = ok && args.MinChainLinkSize > 0
	return ok
}

// NewLatencyTracker sets up- and returns (*LatencyTracker, true) if
// args.Ok() == true. Else, it returns (nil, false).
func NewLatencyTracker(args NewLatencyTrackerArgs) (*LatencyTracker, bool) {
	if !args.Ok() {
		return nil, false
	}
	return &LatencyTracker{cfg: args}, true
}

// Config returns the internal configuration which was used when creating this
// instance with NewLatencyTracker(...).
func (lt *LatencyTracker) Config() NewLatencyTrackerArgs {
	lt.Lock()
	defer lt.Unlock()
	return lt.cfg
}

// Try add new head and trim tail.
// NOTE: no locking, that must be done from the caller.
func (lt *LatencyTracker) maintain() {
	// Handle unset.
	if lt.head == nil {
		lt.head = &latencyTrackerItem{created: time.Now()}
	}

	// New head if enough time has passed.
	// Layout: further from head = further back in time.
	if time.Now().Sub(lt.head.created) >= lt.cfg.MinChainLinkSize {
		lt.head = &latencyTrackerItem{created: time.Now(), next: lt.head}
	}

	// Trim tail.
	current := lt.head
	n := 1
	for current != nil {
		if n >= lt.cfg.MaxChainLinkN {
			current.next = nil
			return
		}
		current = current.next
		n++
	}
}

// Register registers some latency. Specifically, it adds the delta to a node
// that is closest to time.Now(), which might be a node that is created here.
// Additionally, it trims off the old tail(s).
func (lt *LatencyTracker) Register(delta time.Duration) {
	lt.Lock()
	defer lt.Unlock()

	lt.maintain()

	// Add new deltas.
	lt.head.cumulativeLatency += delta
	lt.head.nWaiters++
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

	lt.maintain()

	maxDuration := lt.cfg.MinChainLinkSize * time.Duration(lt.cfg.MaxChainLinkN)
	withinBounds := period <= maxDuration

	var cumulativeWait time.Duration
	var nWaiters int

	// Traverse and add.
	current := lt.head
	for current != nil && stamp.Sub(current.created) <= period {
		cumulativeWait += current.cumulativeLatency
		nWaiters += current.nWaiters

		current = current.next
	}

	// Guard zero div.
	if nWaiters == 0 {
		return 0, withinBounds
	}

	average := cumulativeWait / time.Duration(nWaiters)
	return average, withinBounds
}

// AverageSTD is equivalent to lt.Average(x) where x is the StandardPeriod field
// of NewLatencyTrackerArgs (used when setting up this instance).
func (lt *LatencyTracker) AverageSTD() (time.Duration, bool) {
	return lt.Average(lt.cfg.StandardPeriod)
}
