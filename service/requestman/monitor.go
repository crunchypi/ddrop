package requestman

// linkedListItem is intended as a link in T linkedList.
type linkedListItem[T any] struct {
	data T
	next *linkedListItem[T]
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
func (ll *linkedList[T]) add(data T) {
	newTail := linkedListItem[T]{data: data}
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
	if ll.head != nil && !f(0, ll.head) {
		ll.head = nil
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
