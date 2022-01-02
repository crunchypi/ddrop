package knnc

import "sync"

/*
File for things that could have been an extension of std/sync.
*/

// CancelSignal is a wrapper for 'chan struct{}' and is intended to make the
// idiomatic 'close signal' (i.e close(chan struct{})) clear, and is as such
// the only use-case for this. Note that the only valid way of setting it up
// is with the NewCancelSignal func, this is enforced throughout this pkg.
type CancelSignal struct {
	c chan struct{}

	closed     bool
	closeMutex *sync.RWMutex
}

// NewCancelSignal is a factory func for CancelSignal -- the only valid way
// of setting it up is by using this.
func NewCancelSignal() *CancelSignal {
	return &CancelSignal{c: make(chan struct{}), closed: false, closeMutex: &sync.RWMutex{}}
}

// Cancel sends a cancel signal to all keepers of this instance.
func (cs *CancelSignal) Cancel() {
	cs.closeMutex.Lock()
	defer cs.closeMutex.Unlock()
	if cs.closed {
		return
	}

	cs.closed = true
	close(cs.c)
}

// Cancelled return true only if the Cancel method has been called.
func (cs *CancelSignal) Cancelled() bool {
	cs.closeMutex.RLock()
	defer cs.closeMutex.RUnlock()

	return cs.closed
}
