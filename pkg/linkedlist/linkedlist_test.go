package linkedlist

import (
	"math/rand"
	"testing"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestIter(t *testing.T) {
	ll := LinkedList[int]{}
	ll.head = &Node[int]{Val: 0}
	ll.head.next = &Node[int]{Val: 1}

	nodeIndex := 0
	nodeVal := 0
	ll.Iter(func(i int, node Node[int]) bool {
		nodeIndex = i
		nodeVal = node.Val
		return true
	})

	if nodeIndex != nodeVal {
		t.Fatal("nodeIndex != nodeVal")
	}

	if nodeIndex != 1 {
		t.Fatal("nodeIndex is expected to go up to 1, was", nodeIndex)
	}
}

func TestPutAtHead(t *testing.T) {
	ll := LinkedList[int]{}

	ll.putAtHead(1)
	ll.putAtHead(0)

	if ll.Len() != 2 {
		t.Fatal("unexpected len:", ll.Len())
	}
	if ll.head.Val != 0 {
		t.Fatal("unexpected head val:", ll.head.Val)
	}
}

func TestPutAtTail(t *testing.T) {
	ll := LinkedList[int]{}

	ll.putAtTail(0)
	ll.putAtTail(1)

	if ll.Len() != 2 {
		t.Fatal("unexpected len:", ll.Len())
	}
	if ll.head.Val != 0 {
		t.Fatal("unexpected head val:", ll.head.Val)
	}
}

func TestPut(t *testing.T) {
	ll := LinkedList[int]{}

	// Put at tail such that layout is [2]-[3]-[4].
	for i := 2; i < 5; i++ {
		ll.Put(ll.Len(), i)
	}
	// Put at head such that layout is [0]-[2]-[3]-[4].
	ll.Put(0, 0)

	// Put in the middle (ish), new layout should be [0]-[1]-[2]-[3]-[4] (sequential).
	ll.Put(1, 1)

	if ll.Len() != 5 {
		t.Fatal("unexpected len:", ll.ToSlice())
	}

	ll.Iter(func(i int, n Node[int]) bool {
		if i != n.Val {
			t.Fatalf("unexpected val at index %v, want %v, have %v", i, i, n.Val)
		}
		return true
	})
}

func TestDelHead(t *testing.T) {
	ll := LinkedList[int]{}

	// Make ll [0]-[1]-[2]-[3].
	for i := 0; i < 4; i++ {
		ll.Put(ll.Len(), i)
	}

	n, ok := ll.delHead()
	if !ok {
		t.Fatal("could not delete head")
	}
	if n.Val != 0 {
		t.Fatal("unexpected deleted node val:", n.Val)
	}
	if ll.Len() != 3 {
		t.Fatal("unexpected new len:", ll.Len())
	}
	if ll.head.Val != 1 {
		t.Fatal("unexpected new head val:", ll.tail.Val)
	}
}

func TestDelTail(t *testing.T) {
	ll := LinkedList[int]{}

	// Make ll [0]-[1]-[2]-[3].
	for i := 0; i < 4; i++ {
		ll.Put(ll.Len(), i)
	}

	n, ok := ll.delTail()
	if !ok {
		t.Fatal("could not delete tail")
	}
	if n.Val != 3 {
		t.Fatal("unexpected deleted node val:", n.Val)
	}
	if ll.Len() != 3 {
		t.Fatal("unexpected new len:", ll.Len())
	}
	if ll.tail.Val != 2 {
		t.Fatal("unexpected new tail val:", ll.tail.Val)
	}
}

func TestDel(t *testing.T) {
	ll := LinkedList[int]{}

	// Make sequential ll [0]-[1]-[2]-[3]-[4].
	for i := 0; i < 5; i++ {
		ll.Put(ll.Len(), i)
	}

	// At head such that ll [1]-[2]-[3]-[4].
	n, ok := ll.Del(0)
	if !ok {
		t.Fatal("could not delete at head")
	}
	if n.Val != 0 {
		t.Fatal("unexpected deleted val:", n.Val)
	}

	// At tail such that ll [1]-[2]-[3].
	n, ok = ll.Del(ll.Len())
	if !ok {
		t.Fatal("could not delete at tail")
	}
	if n.Val != 4 {
		t.Fatal("unexpected deleted val:", n.Val)
	}

	// At center such that ll [1]-[3].
	n, ok = ll.Del(1)
	if !ok {
		t.Fatal("could not delete at center")
	}
	if n.Val != 2 {
		t.Fatal("unexpected deleted val:", n.Val)
	}

}

func TestGet(t *testing.T) {
	ll := LinkedList[int]{}

	// Make sequential ll [0]-...-[n-1].
	n := 1000
	for i := 0; i < n; i++ {
		ll.Put(ll.Len(), i)
	}

	// Make random indexes.
	indexes := make([]int, n)
	for i := 0; i < n; i++ {
		indexes[i] = i
	}
	for i := 0; i < n; i++ {
		j := rand.Intn(n)
		indexes[i], indexes[j] = indexes[j], indexes[i]
	}

	for _, i := range indexes {
		n, ok := ll.Get(i)
		if !ok {
			t.Fatal("could not get at index", i)
		}
		if i != n.Val {
			t.Fatalf("unexpected val at index %v: %v", i, n.Val)
		}
	}
}
