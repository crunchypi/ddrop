package knnc

import (
	"sync"
	"testing"
	"time"
)

func TestCancelSignalBasic(t *testing.T) {
	cs := NewCancelSignal()

	select {
	case <-cs.c:
		t.Fatal("false positive (cancelled) from internal channel")
	case <-time.After(time.Millisecond * 10):
	}

	if cs.Cancelled() {
		t.Fatal("false positive (cancelled) from 'Cancelled' method read")
	}

	cs.Cancel()
	select {
	case <-cs.c:
	case <-time.After(time.Millisecond * 10):
		t.Fatal("false negative (not cancelled) from internal channel")
	}

	if !cs.Cancelled() {
		t.Fatal("false negative (not cancelled) from 'Cancelled' method read")
	}
}

func TestCancelSignalConcurrent(t *testing.T) {
	cs := NewCancelSignal()

	n := 20
	wg := sync.WaitGroup{}
	wg.Add(n)

	ch := make(chan bool, n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			select {
			case <-cs.c:
				ch <- true
			case <-time.After(time.Millisecond * 100):
				ch <- false
			}
		}()
	}

	cs.Cancel()
	cs.Cancel() // Attempt double close, should not crash.

	wg.Wait()
	close(ch)

	bools := make([]bool, 0, n)
	for b := range ch {
		bools = append(bools, b)
	}

	if len(bools) != n {
		t.Fatal("unexpected number of results/bools:", len(bools))
	}

	if !boolsOk(bools) {
		t.Fatal("some workers timed out")
	}
}
