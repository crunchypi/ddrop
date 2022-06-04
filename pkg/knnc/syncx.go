package knnc

import "sync"

/*
File for things that could have been an extension of std/sync.
*/

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
