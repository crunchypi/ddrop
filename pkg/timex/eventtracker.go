package timex

import (
	"time"

	"github.com/crunchypi/ddrop/pkg/linkedlist"
)

/*
This file contains an "event tracker", which tracks some occurances over time.
It is based on a linked list impl (/pkg/linkedlist) where each node represents
a discrete amount of time.
*/

// EventTrackerItem is intended as a node in the EventTracker linked list.
type EventTrackerItem[T any] struct {
	// Payload is what is being tracked.
	Payload T
	// N is the number of times the Payload has been tracked.
	N       int
	Created time.Time
}

// EventTrackerConfig is used as configuration for EventTracker. It specifies
// the max nodes in the underlying linked list. It also specifies the discrete
// amount of time a single node represents.
type EventTrackerConfig struct {
	// MinStep specifies the discrete amount of time a single node represents
	// in the underlying linked list.
	MinStep time.Duration
	// MaxN specifies the max length of the underlying linked list.
	MaxN int
}

// Ok validates the configurations. The following must be true:
//  - EventTrackerConfig.MinStep > 0
//  - EventTrackerConfig.MaxN > 0
func (cfg *EventTrackerConfig) Ok() bool {
	ok := true
	ok = ok && cfg.MinStep > 0
	ok = ok && cfg.MaxN > 0
	return ok
}

// EventTracker is intended for tracking some occurences over time. It is based
// on a linked list where each node represents a discrete amount of time.
type EventTracker[T any] struct {
	ll  linkedlist.LinkedList[*EventTrackerItem[T]]
	cfg EventTrackerConfig
}

// NewEventTracker is a factory func for EventTracker; see docs for that type and
// EventTrackerConfig for more details. Returns false if cfg.Ok() returns false.
func NewEventTracker[T any](cfg EventTrackerConfig) (*EventTracker[T], bool) {
	return &(EventTracker[T]{cfg: cfg}), cfg.Ok()
}

// Cfg returns the EventTrackerConfig that was used when creating this instance.
func (et *EventTracker[T]) Cfg() EventTrackerConfig {
    return et.cfg
}

// Iter iterates over the internal linked list.
func (et *EventTracker[T]) Iter(rcv func(index int, item *EventTrackerItem[T]) bool) {
	et.maintain()

	et.ll.Iter(func(i int, node linkedlist.Node[*EventTrackerItem[T]]) bool {
		return rcv(i, node.Val)
	})
}

// maintain does the following maintenance:
//  - Add a new linked list node if the time delta between now and when the
//    last node was created exceeds cfg.MinStep.
//  - Trim the linked list so the length does not exceed cfg.MaxN.
func (et *EventTracker[T]) maintain() {
	// Handle unset.
	if et.ll.Len() == 0 {
		et.ll.Put(0, &EventTrackerItem[T]{Created: time.Now()})
		return
	}

	// New head if enough time has passed.
	head, _ := et.ll.Get(0)
	if time.Now().Sub(head.Val.Created) >= et.cfg.MinStep {
		et.ll.Put(0, &EventTrackerItem[T]{Created: time.Now()})
	}

	// Trim tail.
	numNodesToDelete := et.ll.Len() - et.cfg.MaxN
	for i := 0; i < numNodesToDelete; i++ {
		et.ll.Del(et.ll.Len())
	}
}

// Register registers some event on the newest/latest linked list node (at the
// head). Specifically, it does the following:
//  v := the registered value.
//  n := amount of registries/changes made for v (with this method).
//  m := f(n, v), where f is the accepted func, and m is the new v.
func (et *EventTracker[T]) Register(f func(created time.Time, n int, v T) T) {
	et.maintain()

	node, _ := et.ll.Get(0)
	node.Val.Payload = f(node.Val.Created, node.Val.N, node.Val.Payload)
	node.Val.N++
}

// Collect collects, from the internal linked list, all EventTrackerItem instances which
// were created between time.Now() and time.Now().Sub(period). For example, if period is
// 1 minute, then the returned slice will contain EventTrackerItem instances that were
// created the last minute, in reverse chronological order.
func (et *EventTracker[T]) Collect(period time.Duration) []EventTrackerItem[T] {
	stamp := time.Now()
	et.maintain()

	n := int(period / et.cfg.MinStep)
	r := make([]EventTrackerItem[T], 0, n)
	et.ll.Iter(func(i int, node linkedlist.Node[*EventTrackerItem[T]]) bool {
		include := stamp.Sub(node.Val.Created) <= period
		if include {
			r = append(r, *node.Val)
		}
		return include
	})

	return r
}
