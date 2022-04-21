package requestman

import (
	"math/rand"
	"testing"
	"time"

	"github.com/crunchypi/ddrop/pkg/mathx"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

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

	// Trim everything, one at a time from tail.
	for i := nItems - 1; i >= 0; i-- {
		ll.trim(func(j int, item *linkedListItem[int]) bool {
			cond := i > j
			return cond
		})
		l := ll.len()
		if l != i {
			t.Fatalf("unexpected len of ll, want %v, have %v (iter %v)", i, l, i)
		}
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
	maxN := 88
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
	links := tll.timeRange(stamp, stamp.Add(-time.Duration(float64(allD)*span)))

	if len(links) != int(float64(maxN)*span) {
		t.Fatal("unexpected result len:", len(links))
	}
}

/*
--------------------------------------------------------------------------------
Tests for monitor.
--------------------------------------------------------------------------------
*/

func TestMonItemAvgMergeKNNMonItem(t *testing.T) {
	kmi1 := knnMonItem{Latency: 1, AvgScore: 0.5, Satisfaction: 0.4}
	kmi2 := knnMonItem{Latency: 1, AvgScore: 0.0, Satisfaction: 0.0}

	kmia := KNNMonItemAvg{}
	kmia.mergeKNNMonItem(kmi1)
	kmia.mergeKNNMonItem(kmi2)
	kmia.AvgScoreNoFails = mathx.RoundF64(kmia.AvgScoreNoFails, 2)

	if kmia.N != 2 {
		t.Fatal("unexpected N:", kmia.N)
	}
	if kmia.NFailed != 1 {
		t.Fatal("unexpected NFailed:", kmia.NFailed)
	}
	if kmia.AvgLatency != (kmi1.Latency+kmi2.Latency)/2 {
		t.Fatal("unexpected AvgLatency:", kmia.AvgLatency)
	}
	if kmia.AvgScore != (kmi1.AvgScore+kmi2.AvgScore)/2 {
		t.Fatal("unexpected AvgScore:", kmia.AvgScore)
	}
	if kmia.AvgScoreNoFails != kmi1.AvgScore {
		t.Fatal("unexpected AvgScoreNoFails:", kmia.AvgScoreNoFails)
	}
	if kmia.AvgSatisfaction != (kmi1.Satisfaction+kmi2.Satisfaction)/2 {
		t.Fatal("unexpected AvgSatisfaction:", kmia.AvgSatisfaction)
	}
}

func TestMonItemAvgMergeKNNMonItemAvg(t *testing.T) {
	n := 10 // Must be even.

	knnMonItems := make([]knnMonItem, n)
	for i := 0; i < n; i++ {
		knnMonItems[i] = knnMonItem{
			Latency:      time.Millisecond * time.Duration(rand.Int63n(10)),
			AvgScore:     rand.Float64(),
			Satisfaction: rand.Float64(),
		}
	}

	kmiaExpect := KNNMonItemAvg{}
	for _, item := range knnMonItems {
		kmiaExpect.mergeKNNMonItem(item)
	}

	half1 := knnMonItems[:n]
	kmia1 := KNNMonItemAvg{}
	for _, item := range half1 {
		kmia1.mergeKNNMonItem(item)
	}

	half2 := knnMonItems[n:]
	kmia2 := KNNMonItemAvg{}
	for _, item := range half2 {
		kmia2.mergeKNNMonItem(item)
	}

	kmiaCombo := KNNMonItemAvg{}
	kmiaCombo.mergeKNNMonItemAvg(&kmia1)
	kmiaCombo.mergeKNNMonItemAvg(&kmia2)

	if kmiaCombo.Created != kmia1.Created {
		s := "unexpected time; want %v, have %v \n"
		t.Fatalf(s, kmia1.Created, kmiaCombo.Created)
	}

	if kmiaCombo.N != kmiaExpect.N {
		s := "unexpected N field val; want %v, got %v\n"
		t.Fatalf(s, kmiaExpect.N, kmiaCombo.N)
	}

	if kmiaCombo.NFailed != kmiaExpect.NFailed {
		s := "unexpected NFailed field val; want %v, got %v\n"
		t.Fatalf(s, kmiaExpect.NFailed, kmiaCombo.NFailed)
	}

	if kmiaCombo.AvgLatency != kmiaExpect.AvgLatency {
		s := "unexpected AvgLatency field val; want %v, got %v\n"
		t.Fatalf(s, kmiaExpect.AvgLatency, kmiaCombo.AvgLatency)
	}

	if kmiaCombo.AvgScore != kmiaExpect.AvgScore {
		s := "unexpected AvgScore field val; want %v, got %v\n"
		t.Fatalf(s, kmiaExpect.AvgScore, kmiaCombo.AvgScore)
	}

	if kmiaCombo.AvgScoreNoFails != kmiaExpect.AvgScoreNoFails {
		s := "unexpected AvgScoreNoFails field val; want %v, got %v\n"
		t.Fatalf(s, kmiaExpect.AvgScoreNoFails, kmiaCombo.AvgScoreNoFails)
	}

	if kmiaCombo.AvgSatisfaction != kmiaExpect.AvgSatisfaction {
		s := "unexpected AvgSatisfaction field val; want %v, got %v\n"
		t.Fatalf(s, kmiaExpect.AvgSatisfaction, kmiaCombo.AvgSatisfaction)
	}
}

func TestMonitorAveragePrecice(t *testing.T) {
	d := time.Millisecond * 100
	testStarted := time.Now()

	kmi1 := knnMonItem{Latency: 1, AvgScore: 0.0, Satisfaction: 0.0}
	kmi2 := knnMonItem{Latency: 1, AvgScore: 0.5, Satisfaction: 0.4}

	monitor := knnMonitor{averages: &timedLinkedList[KNNMonItemAvg]{
		maxChainLinkN:    10,
		minChainLinkSize: d,
	}}

	// ll layout, starting with head:  [kmi2]-[kmi1]-[kmi1x2]
	monitor.registerMonItem(kmi1)
	monitor.registerMonItem(kmi1)
	time.Sleep(d)
	monitor.registerMonItem(kmi1)
	time.Sleep(d)
	monitor.registerMonItem(kmi2)

	// All nodes.
	r := monitor.average(testStarted, testStarted.Add(-time.Hour))
	// Note, not checking all stats, that is done in one of the tests above,
	// related to KNNMonItemAvg.
	if r.N != 4 {
		t.Fatal("unexpected N field val:", r.N)
	}
	if r.AvgScore != 0.125 {
		t.Fatal("unexpected AvgScore field val:", r.AvgScore)
	}

	// 2/3 nodes in the linked list (so kmi1+kmi2) and take their average.
	r = monitor.average(testStarted, testStarted.Add(-d*2))
	if r.N != 2 {
		t.Fatal("unexpected N field val:", r.N)
	}
	if r.AvgScore != 0.25 {
		t.Fatal("unexpected AvgScore field val:", r.AvgScore)
	}
}
