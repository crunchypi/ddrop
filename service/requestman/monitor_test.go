package requestman

import (
	"testing"
)

func TestLLIter(t *testing.T) {
	nItems := 5

	// Add 'nItems' linked items, where data=index.
	ll := linkedList[int]{head: &linkedListItem[int]{data: 0}}
	current := ll.head
	for i := 1; i < nItems; i++ {
		current.next = &linkedListItem[int]{data: i}
		current = current.next
	}

	// Set this to the 'data' field of tail
	lastData := 0
	ll.iter(func(_ int, item *linkedListItem[int]) bool {
		lastData = item.data
		return true
	})

	// -1 because zero indexed.
	if lastData != (nItems - 1) {
		t.Fatal("unexpected last data item from linked list:", lastData)
	}
}

func TestLLTail(t *testing.T) {
	nItems := 5

	// Add 'nItems' linked items, where data=index.
	ll := linkedList[int]{head: &linkedListItem[int]{data: 0}}
	current := ll.head
	for i := 1; i < nItems; i++ {
		current.next = &linkedListItem[int]{data: i}
		current = current.next
	}

	tailIndex, tail := ll.tail()

	// -1 because zero indexed.
	if tailIndex != (nItems - 1) {
		t.Fatal("unexpected tail index:", tailIndex)
	}
	// -1 because zero indexed.
	if tail.data != (nItems - 1) {
		t.Fatal("unexpected tail data:", tail.data)
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
	if current.data != (nItems - 1) {
		t.Fatal("unexpected tail data:", current.data)
	}
}

func TestLLTrim(t *testing.T) {
	nItems := 5

	// Add 'nItems' linked items, where data=index.
	ll := linkedList[int]{head: &linkedListItem[int]{data: 0}}
	current := ll.head
	for i := 1; i < nItems; i++ {
		current.next = &linkedListItem[int]{data: i}
		current = current.next
	}

	// Trim everything besides item.data == 0, i.e keep only head.
	ll.trim(func(index int, item *linkedListItem[int]) bool {
		return item.data == 0
	})

	tailIndex, tail := ll.tail()

	// Check for zero since tail should now be head.
	if tailIndex != 0 {
		t.Fatal("unexpected tail index:", tailIndex)
	}
	if tail.data != 0 {
		t.Fatal("unexpected tail data:", tail.data)
	}
}
