package requestman

import (
	"context"
)

// safeChanIterArgs is intended as args for func safeChanIter.
type safeChanIterArgs[T any] struct {
	ch  <-chan T             // ch is chan to iterate over.
	ctx context.Context      // ctx is a way of aborting iteration of ch.
	rcv func(element T) bool // rcv func gets elements from ch. Return false to stop.
}

// ok returns true if all the following conditions are true:
// - args.ch  != nil
// - args.ctx != nil
// - args.rcv != nil
func (args *safeChanIterArgs[T]) ok() bool {
	ok := true
	ok = ok && args.ch != nil
	ok = ok && args.ctx != nil
	ok = ok && args.rcv != nil

	return ok
}

// safeChanIter is a way of iterating somewhat safely over a channel. The reason
// it is implemented is because the normal range iteration over a chan can hang
// very happily. The difference here is that a context.Context can be used to
// mitigate leaks (using context.WithDeadline(...), for instance).
//
// Returns false if args.ok() returns false See doc for args T for more details.
func safeChanIter[T any](args safeChanIterArgs[T]) bool {
	if !args.ok() {
		return false
	}

	for {
		select {
		case elm, ok := <-args.ch:
			if !ok || !args.rcv(elm) {
				return true
			}
		case <-args.ctx.Done():
			return true
		}
	}
}

// safeChanSendArgs is intended as args for func safeChanSend.
type safeChanSendArgs[T any] struct {
	ch  chan<- T
	ctx context.Context
	elm T
}

// ok return true if all the following conditions are true:
// - args.ch  != nil
// - args.ctx != nil
func (args *safeChanSendArgs[T]) ok() bool {
	ok := true
	ok = ok && args.ch != nil
	ok = ok && args.ctx != nil
	return ok
}

// safeChanSend is a way of (somewhat) safely sending a item to a chan. The reason
// it is implemented is because attempting to send something to a chan, normally
// with the arrow syntax, might hang. The difference here is that a context.Context
// can be used to mitigate leaks (using context.WithDeadline(...), for instance).
//
// Returns false if args.ok() returns false. See doc for args T for more details.
func safeChanSend[T any](args safeChanSendArgs[T]) bool {
	if !args.ok() {
		return false
	}
	select {
	case args.ch <- args.elm:
	case <-args.ctx.Done():
	}

	return true
}

// ctxDone is non-blocking and returns true if the given ctx is done, else false.
func ctxDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
