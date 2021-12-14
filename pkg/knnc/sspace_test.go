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
	ss, ok := NewSearchSpace(5)
	if !ok {
		t.Fatal("didn't get a new searchspace")
	}

	ss.items = []DistancerContainer{
		&data{newTVecRand(1)},
		&data{newTVecRand(2)},
		&data{newTVecRand(3)},
	}

	ss.Clear()
	if ss.Len() != 0 {
		t.Errorf("unexpected len after clear: %v", ss.Len())
	}

}

func TestSearchSpaceClean(t *testing.T) {
	ss, ok := NewSearchSpace(5)
	if !ok {
		t.Fatal("didn't get a new searchspace")
	}

	e := 2.
	d := &data{newTVec(e)}

	ss.items = []DistancerContainer{
		&data{newTVec(e - 1)},
		d,
		&data{newTVec(e + 1)},
	}

	ss.Clean()
	if ss.Len() != 3 {
		t.Fatal("first clean removed non-nil item(s)")
	}

	d.v = nil // Mark for deletion.
	ss.Clean()
	if ss.Len() != 2 {
		t.Fatal("second clean did not remove the nil item")
	}

	e2, _ := ss.items[0].Distancer().Peek(0)
	e3, _ := ss.items[1].Distancer().Peek(0)
	if e2 == e || e3 == e {
		t.Error("incorrect item deleted")
	}
}

// Validate basic scanner functionality.
func TestSearchSpaceScanFull(t *testing.T) {
	ss, ok := NewSearchSpace(5)
	if !ok {
		t.Fatal("didn't get a new searchspace")
	}

	ss.items = []DistancerContainer{
		&data{newTVec(1)},
		&data{newTVec(2)},
		&data{newTVec(3)},
	}

	ch, ok := ss.Scan(ScanArgs{1., BaseWorkerArgs{1, NewCancelSignal(), time.Second}})
	if !ok {
		t.Fatal("scan setup failed; invalid args")
	}
	// Just chek the amount for this basic test.
	n := 0
	for scanItem := range ch {
		scanItem.Distancer.Dim() // Just to use the variable.
		n++
	}
	if n != len(ss.items) {
		t.Errorf("didn't scan all items, got only %v", n)
	}
}

// Validate the extent (search percent) functionality of a scanner.
func TestSearchSpaceScanPartial(t *testing.T) {
	ss, ok := NewSearchSpace(5)
	if !ok {
		t.Fatal("didn't get a new searchspace")
	}

	ss.items = []DistancerContainer{
		&data{newTVec(1)},
		&data{newTVec(2)},
	}

	// Ask for only one item.
	extent := 0.5
	ch, ok := ss.Scan(ScanArgs{extent, BaseWorkerArgs{1, NewCancelSignal(), time.Second}})
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
	ss, ok := NewSearchSpace(5)
	if !ok {
		t.Fatal("didn't get a new searchspace")
	}

	ss.items = []DistancerContainer{
		&data{newTVec(1)},
		&data{newTVec(2)},
	}

	// Must not be buffered or else the block below won't work,
	// since one item might be put in the chan before close.
	buf := 0
	cancel := NewCancelSignal()
	ch, ok := ss.Scan(ScanArgs{1, BaseWorkerArgs{buf, cancel, time.Second * 2}})
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
	ss, ok := NewSearchSpace(5)
	if !ok {
		t.Fatal("didn't get a new searchspace")
	}

	// Need more than one item so neither scanner goroutines gets freed
	// after only one item/iteration; this makes the test more correct.
	ss.items = []DistancerContainer{
		&data{newTVec(1)},
		&data{newTVec(1)},
	}

	ch1, ok := ss.Scan(ScanArgs{1, BaseWorkerArgs{0, NewCancelSignal(), time.Second}})
	if !ok {
		t.Fatal("scan setup (1) failed; invalid args")
	}
	ch2, ok := ss.Scan(ScanArgs{1, BaseWorkerArgs{0, NewCancelSignal(), time.Second}})
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
	case <-time.After(time.Second * 3):
		t.Fatal("one of the scanners hung")
	}
}
