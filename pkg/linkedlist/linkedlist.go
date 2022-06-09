/*
Package implements a generic linked list. It is inspired by Rust's implementation,
found at https://github.com/TheAlgorithms/Rust, but ultimately a bit simpler
and with a couple extra convenience methods.
*/
package linkedlist

// Node is intended as a node in the linked list of this pkg.
type Node[T any] struct {
	Val  T
	next *Node[T]
	prev *Node[T]
}

// LinkedList is a double-linked list implementation.
type LinkedList[T any] struct {
	length int
	head   *Node[T]
	tail   *Node[T]
}

func New[T any]() *LinkedList[T] {
	return &LinkedList[T]{}
}

// Len returns the number of nodes in the linked list.
func (ll *LinkedList[T]) Len() int {
	return ll.length
}

// IterMut allows mutable iteration over the linked list.
func (ll *LinkedList[T]) IterMut(rcv func(i int, n *Node[T]) bool) {
	i := 0
	n := ll.head
	for n != nil {
		if !rcv(i, n) {
			return
		}
		i++
		n = n.next
	}
}

// Iter allows non-mutable iteration over the linked list.
func (ll *LinkedList[T]) Iter(rcv func(i int, n Node[T]) bool) {
	ll.IterMut(func(i int, n *Node[T]) bool {
		return rcv(i, *n)
	})
}

// putAtHead is a helper method for putting a new head node.
func (ll *LinkedList[T]) putAtHead(val T) {
	node := Node[T]{Val: val}
	node.next = ll.head

	switch ll.head {
	case nil:
		ll.tail = &node
	default:
		ll.head.prev = &node
	}

	ll.head = &node
	ll.length++
}

// putAtTail is a helper method for putting a new tail node.
func (ll *LinkedList[T]) putAtTail(val T) {
	node := Node[T]{Val: val}
	node.prev = ll.tail

	switch ll.tail {
	case nil:
		ll.head = &node
	default:
		ll.tail.next = &node
	}

	ll.tail = &node
	ll.length++
}

// Put places a value at an index, using a new node. Returns false if index is
// out of bounds.
func (ll *LinkedList[T]) Put(index int, val T) bool {
	if ll.length < index || index < 0 {
		return false
	}
	if index == 0 || ll.head == nil {
		ll.putAtHead(val)
		return true
	}
	if ll.length == index {
		ll.putAtTail(val)
		return true
	}

	ll.IterMut(func(i int, n *Node[T]) bool {
		if i < index {
			return true
		}

		node := Node[T]{Val: val}
		node.prev = n.prev
		node.next = n

		n.prev = &node
		if node.prev != nil {
			node.prev.next = &node
		}

		return false
	})

	ll.length++
	return true
}

// delHead is a helper method for deleting the linked list head.
func (ll *LinkedList[T]) delHead() (Node[T], bool) {
	if ll.head == nil {
		return Node[T]{}, false
	}

	oldHead := ll.head
	ll.head = oldHead.next

	ll.length--
	return *oldHead, true
}

// delTail is a helper method for deleting the linked list tail.
func (ll *LinkedList[T]) delTail() (Node[T], bool) {
	if ll.tail == nil {
		return Node[T]{}, false
	}

	oldTail := ll.tail
	ll.tail = oldTail.prev

	ll.length--
	return *oldTail, true
}

// Del deletes and returns the node at the given index. Returns false if the
// index is out of bounds.
func (ll *LinkedList[T]) Del(index int) (Node[T], bool) {
	if ll.length < index {
		return Node[T]{}, false
	}
	if index == 0 || ll.head == nil {
		return ll.delHead()
	}
	if index == ll.length {
		return ll.delTail()
	}

	oldNode := Node[T]{}
	ll.IterMut(func(i int, n *Node[T]) bool {
		if i < index {
			return true
		}

		oldNode = *n
		if oldPrev := n.prev; oldPrev != nil {
			oldPrev.next = n.next
		}
		if oldNext := n.next; oldNext != nil {
			oldNext.prev = n.prev
		}

		return false
	})

	ll.length--
	return oldNode, true
}

// Get returns a node at the given index and a true if the index was within bounds.
func (ll *LinkedList[T]) Get(index int) (Node[T], bool) {
	b := false
	r := Node[T]{}
	ll.Iter(func(i int, n Node[T]) bool {
		if i == index {
			b = true
			r = n
		}
		return i <= i
	})

	return r, b
}

// ToSlice is a convenience method which converts the linked list into a slice.
func (ll *LinkedList[T]) ToSlice() []Node[T] {
	r := make([]Node[T], ll.length)
	ll.Iter(func(i int, n Node[T]) bool {
		r[i] = n
		return true
	})

	return r
}
