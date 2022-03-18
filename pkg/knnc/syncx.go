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

// Ok returns true if the instance was created correctly (with NewCancelSignal()).
func (cs *CancelSignal) Ok() bool { return cs.c != nil }

// ActiveGoroutinesTicker simply wraps an int and a RWMutes and helps tracking
// the number of currently running goroutines. Usage is simple, increment the
// ticker by calling the AddAwait() method, then invoke the returned callback
// to decrement the ticker. Check the current ticker with the CurrentN() method.
type ActiveGoroutinesTicker struct {
	n     int
	mutex sync.RWMutex
}

// AddAwait increments the internal ticker and returns a func that decrements it
// in a concurrency-safe way.
func (a *ActiveGoroutinesTicker) AddAwait() func() {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.n++

	return func() {
		a.mutex.Lock()
		defer a.mutex.Unlock()
		a.n--
	}
}

// CurrentN returns the value of the internal ticker.
func (a *ActiveGoroutinesTicker) CurrentN() int {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	return a.n
}

// BlockUntilBelowN is a convenience method; it blocks until the internal ticker
// is below the specified 'n'.
func (a *ActiveGoroutinesTicker) BlockUntilBelowN(n int) {
	for a.CurrentN() >= n {
	}
}
