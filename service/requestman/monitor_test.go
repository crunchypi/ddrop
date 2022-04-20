package requestman

import (
	"testing"
	"time"
)

/*
--------------------------------------------------------------------------------
Tests for linked list.
--------------------------------------------------------------------------------
*/

func TestLLIter(t *testing.T) {
	nItems := 5

	// Add 'nItems' linked items, where payload=index.
	ll := linkedList[int]{head: &linkedListItem[int]{payload: 0}}
	current := ll.head
	for i := 1; i < nItems; i++ {
		current.next = &linkedListItem[int]{payload: i}
		current = current.next
	}

	// Set this to the 'payload' field of tail
	lastData := 0
	ll.iter(func(_ int, item *linkedListItem[int]) bool {
		lastData = item.payload
		return true
	})

	// -1 because zero indexed.
	if lastData != (nItems - 1) {
		t.Fatal("unexpected last payload item from linked list:", lastData)
	}
}

func TestLLTail(t *testing.T) {
	nItems := 5

	// Add 'nItems' linked items, where payload=index.
	ll := linkedList[int]{head: &linkedListItem[int]{payload: 0}}
	current := ll.head
	for i := 1; i < nItems; i++ {
		current.next = &linkedListItem[int]{payload: i}
		current = current.next
	}

	tailIndex, tail := ll.tail()

	// -1 because zero indexed.
	if tailIndex != (nItems - 1) {
		t.Fatal("unexpected tail index:", tailIndex)
	}
	// -1 because zero indexed.
	if tail.payload != (nItems - 1) {
		t.Fatal("unexpected tail payload:", tail.payload)
	}
}

func TestLLAdd(t *testing.T) {
	nItems := 5
	ll := linkedList[int]{}

	for i := 0; i < nItems; i++ {
		ll.add(i)
	}

	// Traverse until the end.
	current := ll.head
	for current != nil {
		next := current.next
		if next == nil {
			break
		}
		current = next
	}

	if current == nil {
		t.Fatal("unexpected nil as current")
	}

	// -1 because zero indexed.
	if current.payload != (nItems - 1) {
		t.Fatal("unexpected tail payload:", current.payload)
	}
}

func TestLLTrim(t *testing.T) {
	nItems := 5

	// Add 'nItems' linked items, where payload=index.
	ll := linkedList[int]{head: &linkedListItem[int]{payload: 0}}
	current := ll.head
	for i := 1; i < nItems; i++ {
		current.next = &linkedListItem[int]{payload: i}
		current = current.next
	}

	// Trim everything besides item.payload == 0, i.e keep only head.
	ll.trim(func(index int, item *linkedListItem[int]) bool {
		return item.payload == 0
	})

	tailIndex, tail := ll.tail()

	// Check for zero since tail should now be head.
	if tailIndex != 0 {
		t.Fatal("unexpected tail index:", tailIndex)
	}
	if tail.payload != 0 {
		t.Fatal("unexpected tail payload:", tail.payload)
	}
}

func TestLLLen(t *testing.T) {
	n := 9
	ll := linkedList[int]{}
	for i := 0; i < n; i++ {
		ll.add(i)
	}

	if ll.len() != n {
		t.Fatal("unexpected len:", ll.len())
	}
}

func TestTLLMaintain(t *testing.T) {
	maxN := 10
	minD := time.Millisecond * 10

	tll := timedLinkedList[int]{
		inner:            linkedList[timed[int]]{},
		maxChainLinkN:    maxN,
		minChainLinkSize: minD,
	}

	// Setting head.
	tll.maintain()
	if tll.Inner().len() != 1 {
		t.Fatal("didn't set head")
	}

	// New head, so len must be 2.
	time.Sleep(minD)
	tll.maintain()
	if tll.Inner().len() != 2 {
		t.Fatal("could not add to head")
	}

	// Add many, but excess should be trimmed.
	for i := 0; i < maxN*2; i++ {
		tll.maintain()
		time.Sleep(minD)
	}

	if tll.Inner().len() != maxN {
		t.Fatal("unexpected len after trim:", tll.Inner().len())
	}
}

func TestTLLTimeRange(t *testing.T) {
	maxN := 99
	minD := time.Microsecond * 11
	allD := time.Duration(maxN) * minD

	tll := timedLinkedList[int]{
		inner:            linkedList[timed[int]]{},
		maxChainLinkN:    maxN,
		minChainLinkSize: minD,
	}

	// Precise placement. Head: stamp, and so on.
	stamp := time.Now()
	for i := 0; i < maxN; i++ {
		item := timed[int]{}
		item.created = stamp.Add(-minD * time.Duration(i))
		tll.inner.add(item)
	}

	// Half.
	span := 0.5
	links := tll.timeRange(stamp, time.Duration(float64(allD)*span))

	if len(links) != int(float64(maxN)*span) {
		t.Fatal("unexpected result len:", len(links))
	}
}
