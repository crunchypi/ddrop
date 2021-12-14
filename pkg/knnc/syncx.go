package knnc

/*
File for things that could have been an extension of std/sync.
*/

// CancelSignal is a wrapper for 'chan struct{}' and is intended to make the
// idiomatic 'close signal' (i.e close(chan struct{})) clear, and is as such
// the only use-case for this. Note that the only valid way of setting it up
// is with the NewCancelSignal func, this is enforced throughout this pkg.
type CancelSignal struct {
	c chan struct{}
}

// NewCancelSignal is a factory func for CancelSignal -- the only valid way
// of setting it up is by using this.
func NewCancelSignal() CancelSignal {
	return CancelSignal{make(chan struct{})}
}

// Cancel is the only api of CancelSignal, used to send a cancel signal.
func (cs *CancelSignal) Cancel() {
	// TODO: Race condition if concurrent, is that something that is relevant?
	// Check double close?
	close(cs.c)
}
