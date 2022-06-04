package syncx

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestChanToSlice(t *testing.T) {
	n := 10

	ch := make(chan int)
	go func() {
		defer close(ch)
		for i := 0; i < n; i++ {
			ch <- i
		}
	}()

	s := ChanToSlice(ch)
	if len(s) != n {
		t.Fatal("unexpected slice len:", len(s))
	}
	for i := 0; i < n; i++ {
		x := s[i]
		if x != i {
			s := "unexpected slice element on index %v, want %v, have %v"
			t.Fatalf(s, i, i, x)
		}
	}
}

func TestChanFromSlice(t *testing.T) {
	s := []int{1, 2, 3}

	ch := ChanFromSlice(s)

	i := 0
	for element := range ch {
		if element != s[i] {
			s := "unexpected chan element on index %v, want %v, have %v"
			t.Fatalf(s, i, i, element)
		}
		i++
	}

	if i != len(s) {
		t.Fatal("unexpected num of chan elements:", i)
	}
}

// Scenario: Completely drain a chan.
func TestChanIterCompleted(t *testing.T) {
	s := []int{1, 2, 3}
	ch := ChanFromSlice(s)

	i := 0
	ChanIter(ChanIterArgs[int]{
		In:  ch,
		Ctx: context.Background(),
		Rcv: func(element int) bool {
			if element != s[i] {
				s := "unexpected chan element on index %v, want %v, have %v"
				t.Fatalf(s, i, i, element)
			}
			i++
			return true
		},
	})

	if i != len(s) {
		t.Fatal("unexpected num of chan elements:", i)
	}
}

// Scenario: Partially drain a chan, using the ChanIterArgs.Rcv func to abort.
func TestChanIterPartial(t *testing.T) {
	s := []int{1, 2, 3}
	ch := ChanFromSlice(s)

	i := 0
	j := 1 // Stop iter when i >= j.
	ChanIter(ChanIterArgs[int]{
		In:  ch,
		Ctx: context.Background(),
		Rcv: func(element int) bool {
			if element != s[i] {
				s := "unexpected chan element on index %v, want %v, have %v"
				t.Fatalf(s, i, i, element)
			}
			i++
			return i < j
		},
	})

	if i != j {
		t.Fatal("i != j, test impl error")
	}
	if i == 0 {
		t.Fatal("did not read any elements from chan")
	}
}

// Scenario: When chan read encounters a block.
func TestChanIterBlocked(t *testing.T) {
	chWork := make(chan int)      // To read from. Note; never closed.
	chDone := make(chan struct{}) // Used to prevent test hang.

	durationChWork := time.Second / 4 // Exit ChanIter after this.
	durationChFail := time.Second / 2 // Fail Test after this.

	ctx, ctxCancel := context.WithDeadline(
		context.Background(),
		time.Now().Add(durationChWork),
	)
	defer ctxCancel()

	go func() {
		// Signal done, i.e not hung.
		defer close(chDone)
		ChanIter(ChanIterArgs[int]{
			In:  chWork,
			Ctx: ctx,
			Rcv: func(element int) bool {
				return true
			},
		})
	}()

	// Check test hang.
	select {
	case <-chDone:
	case <-time.After(durationChFail):
		t.Fatal("test hung")
	}
}

// Scenario: ChanSend completed successfully.
func TestChanSendCompleted(t *testing.T) {
	ch := make(chan int)
	go func() {
		defer close(ch)
		ChanSend(ChanSendArgs[int]{
			Out: ch,
			Ctx: context.Background(),
			Elm: 1,
		})
	}()

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("test hung")
	}
}

// Scenario: ChanSend blocked.
func TestChanSendBlocked(t *testing.T) {
	chWork := make(chan int)      // To Send to. Note; never read from.
	chDone := make(chan struct{}) // Used to prevent test hang.

	durationChWork := time.Second / 4 // Exit ChanSend after this.
	durationChFail := time.Second / 2 // Fail Test after this.

	ctx, ctxCancel := context.WithDeadline(
		context.Background(),
		time.Now().Add(durationChWork),
	)
	defer ctxCancel()

	go func() {
		defer close(chDone)
		ChanSend(ChanSendArgs[int]{
			Out: chWork,
			Ctx: ctx,
			Elm: 1,
		})
	}()

	// Check test hang.
	select {
	case <-chDone:
	case <-time.After(durationChFail):
		t.Fatal("test hung")
	}
}

// Scenario: complete stages without any block.
func TestStageCompleted(t *testing.T) {
	// In stage A: Add 1 to each element.
	// In stage B: Sub 1 from each element.
	work := []int{1, 2, 3}

	// Shared between stage A and B.
	stageArgsPartial := StageArgsPartial{
		Ctx:                context.Background(),
		TTL:                time.Hour,
		Buf:                0,
		UnsafeDoneCallback: nil,
	}

	// Stage A: Add 1 to each element from the work slice.
	chA, ok := Stage(StageArgs[int, int]{
		NWorkers:         10,
		In:               ChanFromSlice(work),
		TaskFunc:         func(element int) (int, bool) { return element + 1, true },
		StageArgsPartial: stageArgsPartial,
	})
	if !ok {
		t.Fatal("could not start stage A; test impl err")
	}

	// Stage B: Sub 1 from each element from stage A output.
	chB, ok := Stage(StageArgs[int, int]{
		NWorkers:         10,
		In:               chA,
		TaskFunc:         func(element int) (int, bool) { return element - 1, true },
		StageArgsPartial: stageArgsPartial,
	})
	if !ok {
		t.Fatal("could not start stage B; test impl err")
	}

	// Validate.
	result := ChanToSlice(chB)
	if len(result) != len(work) {
		t.Fatal("unexpected number of results:", len(result))
	}

	// Sum of work should be the same as sum of results since pipeline is a noop.
	sum := func(s []int) int {
		r := 0
		for _, x := range s {
			r += x
		}
		return r
	}
	if sum(work) != sum(result) {
		t.Fatalf("sum of work (%v) != sum of result (%v)", sum(work), sum(result))
	}
}

// Scenario: one stage blocks.
func TestStageBlocked(t *testing.T) {
	work := make(chan int) // Note, never closed.

	nWorkersA := 2 // N workers in stage A
	nWorkersB := 2 // N workers in stage B

	// Done through StageArgsPartial.UnsafeDoneCallback.
	wg := sync.WaitGroup{}
	wg.Add(nWorkersA + nWorkersB)

	durationWork := time.Second / 4 // Exit stages after this.
	durationFail := time.Second / 2 // Fail test after this.

	// Used to cancel stages.
	ctx, ctxCancel := context.WithDeadline(
		context.Background(),
		time.Now().Add(durationWork),
	)
	defer ctxCancel()

	// Shared between stage A and B.
	stageArgsPartial := StageArgsPartial{
		Ctx: ctx,
		TTL: time.Hour,
		Buf: 0,
		// NOTE; used to check that stage failed gracefully.
		UnsafeDoneCallback: func() { wg.Done() },
	}

	// Stage A: Add 1 to each element from the work slice.
	chA, ok := Stage(StageArgs[int, int]{
		NWorkers:         nWorkersA,
		In:               work,
		TaskFunc:         func(element int) (int, bool) { return element + 1, true },
		StageArgsPartial: stageArgsPartial,
	})
	if !ok {
		t.Fatal("could not start stage A; test impl err")
	}

	// Stage B: Sub 1 from each element from stage A output.
	chB, ok := Stage(StageArgs[int, int]{
		NWorkers: nWorkersB,
		In:       chA,
		TaskFunc: func(element int) (int, bool) {
			// NOTE; block.
			time.Sleep(time.Hour * 99)
			return element - 1, true
		},
		StageArgsPartial: stageArgsPartial,
	})
	if !ok {
		t.Fatal("could not start stage B; test impl err")
	}

	// Check test blocked.
	select {
	// This should close when the pipeline fails gracefully.
	case <-ChanFromSlice(ChanToSlice(chB)):
	case <-time.After(durationFail):
		t.Fatal("test hung")
	}
}
