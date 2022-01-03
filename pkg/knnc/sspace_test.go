package knnc

import (
	"testing"
	"time"
)

func TestSearchSpaceAddSearchable(t *testing.T) {
	ss, ok := NewSearchSpace(2)
	if !ok {
		t.Fatal("didn't get a new searchspace")
	}

	if ss.AddSearchable(&data{}) {
		t.Fatal("added a DistancerContainer with nil internal Distancer ")
	}

	if !ss.AddSearchable(&data{newTVecRand(3)}) {
		t.Fatal("could not add to fresh search space")
	}

	if ss.AddSearchable(&data{newTVecRand(9)}) {
		t.Fatal("vec dim consistency check failed")
	}

	if !ss.AddSearchable(&data{newTVecRand(3)}) {
		t.Fatal("could not reach search space cap")
	}

	ss.items = make([]DistancerContainer, 0, 10)
	if !ss.AddSearchable(&data{newTVecRand(9)}) {
		t.Fatal("vec dim consistency enforced even though ss is empty")
	}
}

func TestSearchSpaceClear(t *testing.T) {
	ss := SearchSpace{
		items: []DistancerContainer{
			&data{newTVecRand(1)},
			&data{newTVecRand(2)},
			&data{newTVecRand(3)},
		},
	}

	ss.Clear()
	if ss.Len() != 0 {
		t.Errorf("unexpected len after clear: %v", ss.Len())
	}
}

func TestSearchSpaceClean(t *testing.T) {
	e1 := 1.
	forDelete := &data{newTVec(e1)}
	ss := SearchSpace{
		items: []DistancerContainer{
			&data{newTVec(2)},
			forDelete,
			&data{newTVec(3)},
		},
	}

	ss.Clean()
	if ss.Len() != 3 {
		t.Fatal("first clean removed non-nil item(s)")
	}

	forDelete.v = nil // Mark for deletion.
	ss.Clean()
	if ss.Len() != 2 {
		t.Fatal("second clean did not remove the nil item")
	}

	e2, _ := ss.items[0].Distancer().Peek(0)
	e3, _ := ss.items[1].Distancer().Peek(0)
	if e2 == e1 || e3 == e1 {
		t.Error("incorrect item deleted")
	}
}

// Validate basic scanner functionality.
func TestSearchSpaceScanFull(t *testing.T) {
	ss := SearchSpace{
		items: []DistancerContainer{
			&data{newTVec(1)},
			&data{newTVec(2)},
			&data{newTVec(3)},
		},
	}

	ch, ok := ss.Scan(SearchSpaceScanArgs{
		Extent: 1.,
		BaseWorkerArgs: BaseWorkerArgs{
			Buf:           1,
			Cancel:        NewCancelSignal(),
			BlockDeadline: time.Second,
		},
	})

	if !ok {
		t.Fatal("scan setup failed; invalid args")
	}

	// Just chek the amount for this basic test.
	n := 0
	for range ch {
		n++
	}

	if n != len(ss.items) {
		t.Errorf("didn't scan all items, got only %v", n)
	}
}

// Validate the extent (search percent) functionality of a scanner.
func TestSearchSpaceScanPartial(t *testing.T) {
	ss := SearchSpace{
		items: []DistancerContainer{
			&data{newTVec(1)},
			&data{newTVec(2)},
		},
	}

	// Ask for 50%, so only one item.
	extent := 0.5
	ch, ok := ss.Scan(SearchSpaceScanArgs{
		Extent: extent,
		BaseWorkerArgs: BaseWorkerArgs{
			Buf:           1,
			Cancel:        NewCancelSignal(),
			BlockDeadline: time.Second,
		},
	})

	if !ok {
		t.Fatal("scan setup failed; invalid args")
	}

	n := 0
	for scanItem := range ch {
		scanItem.Distancer.Dim() // Just to use the variable.
		n++
	}

	if n != int(float64(len(ss.items))*extent) {
		t.Error("scanned all items")
	}
}

// Validate that the scanner stops after sending the stop signal.
func TestSearchSpaceScanStopped(t *testing.T) {
	ss := SearchSpace{
		items: []DistancerContainer{
			&data{newTVec(1)},
			&data{newTVec(2)},
		},
	}

	cancel := NewCancelSignal()
	ch, ok := ss.Scan(SearchSpaceScanArgs{
		Extent: 1,
		BaseWorkerArgs: BaseWorkerArgs{
			// Must not be buffered or else the block below won't work,
			// since one item might be put in the chan before close.
			Buf:           0,
			Cancel:        cancel,
			BlockDeadline: time.Second,
		},
	})

	if !ok {
		t.Fatal("scan setup failed; invalid args")
	}

	<-ch
	cancel.Cancel()
	_, ok = <-ch
	if ok {
		t.Error("scanner didn't stop after signal")
	}
}

// Validate that a scanner can be used with goroutines.
func TestSearchSpaceScanConcurrent(t *testing.T) {
	ss := SearchSpace{
		// Need more than one item so neither scanner goroutines gets freed
		// after only one item/iteration; this makes the test more correct.
		items: []DistancerContainer{
			&data{newTVec(1)},
			&data{newTVec(1)},
		},
	}

	args := SearchSpaceScanArgs{
		Extent: 1,
		BaseWorkerArgs: BaseWorkerArgs{
			Buf:           0,
			Cancel:        NewCancelSignal(),
			BlockDeadline: time.Second,
		},
	}

	ch1, ok := ss.Scan(args)
	if !ok {
		t.Fatal("scan setup (1) failed; invalid args")
	}

	ch2, ok := ss.Scan(args)
	if !ok {
		t.Fatal("scan setup (2) failed; invalid args")
	}

	// Simultaneous read/check will make it hang if it's locked, but not
	// with a RWMutex (.RLock()). So using another goroutine here to prevent
	// halting the entire test in case this is implemented incorrectly.
	done := make(chan struct{})
	go func() {
		if (<-ch1) == (<-ch2) {
			close(done)
		}
	}()

	select {
	case <-done:
		return
	case <-time.After(time.Second):
		t.Fatal("one of the scanners hung")
	}
}
