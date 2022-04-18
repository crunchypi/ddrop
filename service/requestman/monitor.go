package requestman

import (
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
	// Handle nil head.
	if tll.inner.head == nil {
		tll.inner.add(timed[T]{created: time.Now()})
	}

	// Handle expired head.
	headDelta := time.Now().Sub(tll.inner.head.payload.created)
	if headDelta > tll.minChainLinkSize {
		newHead := linkedListItem[timed[T]]{}
		newHead.payload.created = time.Now()
		newHead.next = tll.inner.head
		tll.inner.head = &newHead
	}

	// Handle tail limit.
	tll.inner.trim(func(i int, item *linkedListItem[timed[T]]) bool {
		return i < tll.maxChainLinkN
	})
}

// timeRange returns all links that were created since time.Now() - period.
// So period=time.Minute will return all nodes creted the last minute.
func (tll *timedLinkedList[T]) timeRange(period time.Duration) []timed[T] {
	stamp := time.Now()
	tll.maintain()

	result := make([]timed[T], 0, tll.maxChainLinkN)
	tll.inner.trim(func(i int, item *linkedListItem[timed[T]]) bool {
		include := stamp.Sub(item.payload.created) <= period

		if include {
			result = append(result, item.payload)
		}

		return include
	})

	return result
}
