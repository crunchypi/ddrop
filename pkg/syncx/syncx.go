/*
Syncx is a package which extends the standard library by providing utils related
to concurrency, given Go v1.18 (generics), with tools such as a concurrent stage.
*/
package syncx

import (
	"context"
	"sync"
	"time"
)

// ChanToSlice eagerly exhausts the given chan and puts all elements into the
// returned slice. Useful for testing purposes.
func ChanToSlice[T any](ch <-chan T) []T {
	s := make([]T, 0, 10) // 10 is arbitrary.
	for elm := range ch {
		s = append(s, elm)
	}

	return s
}

// ChanFromSlice returns a chan through which all elements of the given slice
// are pushed through. Useful for testing purposes.
func ChanFromSlice[T any](s []T) <-chan T {
	ch := make(chan T)
	go func() {
		defer close(ch)
		for _, elm := range s {
			ch <- elm
		}
	}()

	return ch
}

// ChanIterArgs is intended as args for the ChanIter func.
type ChanIterArgs[T any] struct {
	In  <-chan T             // In is chan to iterate over.
	Ctx context.Context      // Ctx is a way of aborting iteration of the In chan.
	Rcv func(element T) bool // Rcv func gets elements from In. Return false to stop.
}

// Ok is used for validation and returns true if all the following is true:
//  - args.ch  != nil
//  - args.ctx != nil
//  - args.rcv != nil
func (args *ChanIterArgs[T]) Ok() bool {
	ok := true
	ok = ok && args.In != nil
	ok = ok && args.Ctx != nil
	ok = ok && args.Rcv != nil
	return ok
}

// ChanIter is a way of iterating over a chan while having the ability to mitigate
// hang through the use of a context.Context (e.g context.WithDeadline). See docs
// for ChanIterArgs for more details. Returns false if args.Ok() returns false.
func ChanIter[T any](args ChanIterArgs[T]) bool {
	if !args.Ok() {
		return false
	}

	for {
		select {
		case elm, ok := <-args.In:
			if !ok || !args.Rcv(elm) {
				return true
			}
		case <-args.Ctx.Done():
			return true
		}
	}
}

// ChanSendArgs is intended as args for the ChanSend func.
type ChanSendArgs[T any] struct {
	Out chan<- T
	Ctx context.Context
	Elm T
}

// Ok is used for validation and returns true if the following is true:
//  - args.Out != nil
//  - args.Ctx != nil
func (args *ChanSendArgs[T]) Ok() bool {
	ok := true
	ok = ok && args.Out != nil
	ok = ok && args.Ctx != nil
	return ok
}

// ChanSend is a way of sending items through a chan while having the ability to
// mitigate hang through the use of a context.Context (e.g. context.WithDeadline).
// See docs for ChanSendArgs for more details. Returns true if:
//  - args.Ok() returns false.
//  - args.Ctx.Done() finishes before args.Elm can be sent through args.Out.
func ChanSend[T any](args ChanSendArgs[T]) bool {
	if !args.Ok() {
		return false
	}
	select {
	case args.Out <- args.Elm:
	case <-args.Ctx.Done():
		return false
	}

	return true
}

// StageArgsPartial is intended as partial args which, in combination with
// StageArgs, is intended as arg for the (concurrent) Stage func. The reason
// args are split is because StageArgsPartial can be shared between multiple
// different stages.
type StageArgsPartial struct {
	// Ctx is used to explicitly cancel- and exit the concurrent stage.
	Ctx context.Context
	// TTL specifies how long the stage can be alive.
	TTL time.Duration
	// Buf specifies the output chan buffer for this concurrent stage.
	Buf int
	// UnsafeDoneCallback is called when a gorougine/worker is done. It is
	// named unsafe because it is done in a goroutine (i.e concurrently) and
	// the safety safety depends on usage.
	UnsafeDoneCallback func()
}

// Ok is used for validation and returns true if the following is true:
//  - args.Ctx != nil
//  - args.TTL > 0
//  - args.Buf >= 0
func (args *StageArgsPartial) Ok() bool {
	ok := true
	ok = ok && args.Ctx != nil
	ok = ok && args.TTL > 0
	ok = ok && args.Buf >= 0
	return ok
}

// StageArgs is intended as args for the Stage func. T is intput, U is output.
type StageArgs[T, U any] struct {
	// NWorkers specifies the number of workers to use in this stage.
	NWorkers int
	// In is the input chan for this concurrent stage.
	In <-chan T
	// TaskFunc specifies what each worker should do in this concurrent stage.
	// Specifically, elements of type T are read through the In chan, given to
	// this TaskFunc, then the returned U is written to the output chan _if_
	// the returned bool is true. Note that, althought the stage has the capability
	// of being cancelled (fields Ctx and TTL of StageArgsPartial), it does not
	// necessarily apply to this TaskFunc specifically. So if
	//  TaskFunc: func(x int) {
	//      time.Sleep(hour)
	//      return x, true
	//  },
	// ... then this goroutine will still block for an hour.
	TaskFunc func(T) (U, bool)
	StageArgsPartial
}

// Ok is used for validation and returns true if the following is true:
//  - args.NWorkers > 0
//  - args.In != nil
//  - args.TaskFunc != nil
//  - args.StageArgsPartial.Ok() is true.
func (args *StageArgs[T, U]) Ok() bool {
	ok := true
	ok = ok && args.NWorkers > 0
	ok = ok && args.In != nil
	ok = ok && args.TaskFunc != nil
	ok = ok && args.StageArgsPartial.Ok()
	return ok
}

// Stage is a concurrent stage, using a specified number of goroutines to read
// elements from a chan of type T, transform them to type U, then send them out
// through the output chan. See docs for StageArgs and StageArgsPartial for more
// info. Returns false if args.Ok() returns false.
//
// Note that all chan read and writes are done with ChanIter and ChanSend.
func Stage[T, U any](args StageArgs[T, U]) (<-chan U, bool) {
	if !args.Ok() {
		return nil, false
	}

	out := make(chan U, args.Buf)
	wg := sync.WaitGroup{}
	wg.Add(args.NWorkers)

	ctx, ctxCancel := context.WithDeadline(args.Ctx, time.Now().Add(args.TTL))

	for i := 0; i < args.NWorkers; i++ {
		go func() {
			defer wg.Done()
			// Try iterating over input ch.
			ChanIter(ChanIterArgs[T]{
				In:  args.In,
				Ctx: ctx,
				// Transform element of input chan.
				Rcv: func(element T) bool {
					conv, ok := args.TaskFunc(element)
					if !ok {
						return true
					}

					// Try sending transformed element out.
					return ChanSend(ChanSendArgs[U]{
						Out: out,
						Ctx: ctx,
						Elm: conv,
					})
				},
			})

			if args.UnsafeDoneCallback != nil {
				args.UnsafeDoneCallback()
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
		ctxCancel()
	}()

	return out, true
}
