package requestman

import (
	"math"
	"sync"
	"time"
)

/*
--------------------------------------------------------------------------------
Linked list impl for tracking monitoring stats over windows of time.
--------------------------------------------------------------------------------
*/

// linkedListItem is intended as a link in T linkedList.
type linkedListItem[T any] struct {
	payload T
	next    *linkedListItem[T]
}

// linkedList is a simple linked list implementation with a few useful methods.
type linkedList[T any] struct {
	head *linkedListItem[T]
}

// iter iterates over the linked list instance and passes each link (and index)
// to the recieving func. Stops iteration if the recieving func returns false.
func (ll *linkedList[T]) iter(f func(index int, item *linkedListItem[T]) bool) {
	current := ll.head
	i := 0
	for current != nil && f(i, current) {
		current = current.next
		i++
	}
}

// tail returns the tail (and index) of the linked list instance. Tail might be nil.
func (ll *linkedList[T]) tail() (int, *linkedListItem[T]) {
	var i int
	var tail *linkedListItem[T]

	// tail var above might not be set.
	ll.iter(func(j int, current *linkedListItem[T]) bool {
		i = j
		tail = current
		return true
	})

	return i, tail
}

// add adds a new link with the given data at the end of the linked list instance.
func (ll *linkedList[T]) add(payload T) {
	newTail := linkedListItem[T]{payload: payload}
	_, tail := ll.tail()

	// Handle nil/unset head.
	if tail == nil {
		ll.head = &newTail
		return
	}

	tail.next = &newTail
}

// trim iterates over the linked list and passes each item (and index) to the
// receiving func. Will trim/delete the current item from the chain if the
// receiving func returns false.
func (ll *linkedList[T]) trim(f func(index int, item *linkedListItem[T]) bool) {
	// Check head seperatly for cleanliness.
	if ll.head == nil || (ll.head != nil && !f(0, ll.head)) {
		ll.head = nil
		return
	}

	// Trim tail.
	var prev *linkedListItem[T] = ll.head
	var curr *linkedListItem[T] = ll.head.next
	var iter int = 1

	for curr != nil {
		if !f(iter, curr) {
			prev.next = nil
			return
		}

		prev = curr
		curr = curr.next
		iter++
	}
}

// len returns the number of nodes.
func (ll *linkedList[T]) len() int {
	r := 0
	ll.iter(func(_ int, _ *linkedListItem[T]) bool {
		r++
		return true
	})

	return r
}

// timed is a time wrapper for any T, i.e timed T.
type timed[T any] struct {
	created time.Time
	inner   T
}

// timedLinkedList is an extension of T linkedList, where each link represents a
// discrete frame of time. It's intended use case is to keep track of events during
// windows of time, e.g latency the last second, minute, etc.
//
// Layout is [head:now]-...-[tail:then].
type timedLinkedList[T any] struct {
	inner linkedList[timed[T]]

	// maxChainLinkN specifies the max amount of links in this chain.
	// When the number of links exceeds this, and new links are added,
	// then old ones (at the end/tail) are dropped.
	maxChainLinkN int
	// MinChainLinkSize represents the min time delta between any link.
	// This linked list impl operates on the principle that each link
	// represents a discrete timeframe, this field specifies that window.
	minChainLinkSize time.Duration
}

// Inner exposes the inner linked list.
func (tll *timedLinkedList[T]) Inner() *linkedList[timed[T]] {
	return &tll.inner
}

// maintain does maintenance in order to make the instance state true to the
// configurations. Specifically, it
// - Adds a new head if current head is nil.
// - Adds a new head (moving the old one) if delta time between now and
//   creation time of old head is greater than tll.minChainLinkSize.
// - Trims the tail such that n links does not exceed tll.maxChainLinkN
func (tll *timedLinkedList[T]) maintain() {
	now := time.Now()
	// Handle nil head.
	if tll.inner.head == nil {
		tll.inner.add(timed[T]{created: now})
	}

	// Handle expired head.
	headDelta := now.Sub(tll.inner.head.payload.created)
	if headDelta > tll.minChainLinkSize {
		newHead := linkedListItem[timed[T]]{}
		newHead.payload.created = now
		newHead.next = tll.inner.head
		tll.inner.head = &newHead
	}

	// Handle tail limit.
	tll.inner.trim(func(i int, item *linkedListItem[timed[T]]) bool {
		// -1 because i is zero indexed.
		return i <= tll.maxChainLinkN-1
	})
}

// timeRange returns all links that were created in the given time range..
// Note that 'start' should be _after_ 'end', which might be counter-intuitive.
// The reason being that the linked list goes from head to tail in a reverse
// chronological order, so [now-x*0]-[now-x*1]-[now-x*2] etc.
//
// Example: to get items created the last minute, one should do this:
//  now := time.Now()
//  x.timeRange(now, now.Add(-time.Minute))
//
func (tll *timedLinkedList[T]) timeRange(start, end time.Time) []timed[T] {
	// Max number of links/nodes to include, a slight optimization.
	d := start.Sub(end)
	resultMaxN := int(d / tll.minChainLinkSize)
	if resultMaxN <= 0 {
		return []timed[T]{}
	}
	result := make([]timed[T], 0, resultMaxN)

	tll.inner.trim(func(i int, item *linkedListItem[timed[T]]) bool {
		include := true
		include = include && (i+1) <= resultMaxN
		include = include && start.Sub(item.payload.created) < d

		if include {
			result = append(result, item.payload)
		}

		return include
	})

	tll.maintain()
	return result
}

/*
--------------------------------------------------------------------------------
Monitor impl starts here.
--------------------------------------------------------------------------------
*/

// knnMonItem captures stats per individual KNN request.
type knnMonItem struct {
	Latency      time.Duration
	AvgScore     float64
	Satisfaction float64
}

// KNNMonItemAvg captures stats for a group of KNN requests over a period.
type KNNMonItemAvg struct {
	isSet   bool
	Created time.Time     // Start of recording.
	Span    time.Duration // Recording duration.

	N               int           // Number of recorded requests (including fails).
	NFailed         int           // Number of (completely) failed requests.
	AvgLatency      time.Duration // Average latency of all requests.
	AvgScore        float64       // Average score for all requests.
	AvgScoreNoFails float64       // Same as AvgScore but without fails.
	AvgSatisfaction float64       // Success ratio (got n / want n).
}

// mergeKNNMonItem merges a knnMonItem in such a way that averages are maintained.
// Note that KNNMonItemAvg.AvgScoreNoFails will have some imprecision and should
// only be used for estimation.
//
// Internals: ia.isSet will be set to true.
func (ia *KNNMonItemAvg) mergeKNNMonItem(i knnMonItem) {
	if !ia.isSet {
		ia.isSet = true
		ia.Created = time.Now()
	}

	// Expand to old total.
	n := float64(ia.N)
	totalLatency := ia.AvgLatency * time.Duration(n)
	totalScore := ia.AvgScore * n
	totalSatisfaction := ia.AvgSatisfaction * n

	// Add and contract to new average. Note const to prevent zero div.
	c := 0.00000001
	n++
	ia.N = int(n)
	ia.NFailed += int(math.Floor(1 - i.Satisfaction))
	ia.AvgLatency = (time.Duration(totalLatency) + i.Latency) / time.Duration(n)
	ia.AvgScore = (totalScore + i.AvgScore) / n
	ia.AvgScoreNoFails = (totalScore + i.AvgScore) / (n - float64(ia.NFailed) + c)
	ia.AvgSatisfaction = (totalSatisfaction + i.Satisfaction) / n
}

// mergeKNNMonItemAvg merges another KNNMonItemAvg instance with this instance,
// only 'this' is changed. The merging is done as follows:
// - this.Created is set to be the oldest.
// - other.N is added to this.N.
// - All other field pairs are simply added, divided by 2, then set to this.
func (ia *KNNMonItemAvg) mergeKNNMonItemAvg(other *KNNMonItemAvg) {
	if !ia.isSet {
		*ia = *other
		return
	}
	if !other.isSet {
		return
	}

	// Set to earliest timestamp.
	if other.Created.Before(ia.Created) {
		ia.Created = other.Created
	}

	ia.Span = (ia.Span + other.Span) / 2
	ia.N = ia.N + other.N
	ia.NFailed = (ia.NFailed + other.NFailed)
	ia.AvgLatency = (ia.AvgLatency + other.AvgLatency) / 2
	ia.AvgScore = (ia.AvgScore + other.AvgScore) / 2
	ia.AvgScoreNoFails = (ia.AvgScoreNoFails + other.AvgScoreNoFails) / 2
	ia.AvgSatisfaction = (ia.AvgSatisfaction + other.AvgSatisfaction) / 2
}

// knnMonitor is intended for monitoring KNN requests in this pkg. It operates
// on the principle that current entries are added at the head of a linked list,
// and over time the entries are pushed towards the tail. This gives averages
// of monitoring items (T KNNMonItemAvg) in particular time frames.. The entries
// themselves are registered with:
//  knnMonitor.register(...)
// The method accepts a KNNEnqueueResults, which is intended to be what a KNN
// requester gets after making a query. That T contains a chan with results;
// the idea here is to put a 'man in the middle' listener between the user
// and the sender end of that chan, then capture the results that way.
// Intended setup:
// - User makes a knn request.
// - A instance of KNNEnqueueResult is made, by default it goes to both the
//   requester and the internal request processing (latter sends results to
//   the former).
// - That KNNEnqueueResult (A) is put into knnMonitor.register(...), which
//   returns another KNNEnqueueResult (B).
// - Internal request processing gets A, requester gets B.
// - Read with knnMonitor.average(...)
//
// Note; thread safe.
type knnMonitor struct {
	mx       sync.Mutex
	averages *timedLinkedList[KNNMonItemAvg]
}

// registerMonItem merges a knnMonItem into the head of the internal linked list.
func (m *knnMonitor) registerMonItem(item knnMonItem) {
	m.mx.Lock()
	defer m.mx.Unlock()

	// Garantee head.
	m.averages.maintain()
	monItem := &m.averages.inner.head.payload.inner
	if !monItem.isSet {
		monItem.isSet = true
		monItem.Created = m.averages.inner.head.payload.created
		monItem.Span = m.averages.minChainLinkSize
	}

	monItem.mergeKNNMonItem(item)
}

// average merges together all internal KNNMonItemAvg in the given period,
// then returns the result. Note that 'start' should be _after_ 'end', this
// might be counter-intuitive. The reason being that the underlying linked
// list goes from head to tail in a reverse chronological order, so the
// layout is [now-x*0]-[now-x*1]-[now-x*2] etc.
//
// Example: to get items created the last minute, one should do this:
//  now := time.Now()
//  x.timeRange(now, now.Add(-time.Minute))
//
// Note; thread safe.
func (m *knnMonitor) average(start, end time.Time) KNNMonItemAvg {
	m.mx.Lock()
	defer m.mx.Unlock()

	items := m.averages.timeRange(start, end)
	if len(items) == 0 {
		return KNNMonItemAvg{}
	}

	result := items[0].inner
	items = items[1:]
	for _, itemAvg := range items {
		if !itemAvg.inner.isSet {
			continue
		}
		result.mergeKNNMonItemAvg(&itemAvg.inner)
	}

	m.averages.maintain()
	return result
}
