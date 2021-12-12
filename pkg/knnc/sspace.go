package knnc

import (
	"reflect"
	"sync"
)

/*
File contains code for a 'SearchSpace' type, which will be the core keeper of
DistancerContainer (interface in this pkg). The intent for this SearchSpace T
is to be scannable in a concurrent context.
*/

// SearchSpace is a keeper and scanner of DistancerContainer(s). It is intended
// to be one of many and will as such have sizing limitations (max capacity) so
// it's easier to keep in main memory.
type SearchSpace struct {
	items  []DistancerContainer
	vecDim int // Only uniform vectors (mathx.Distancer).
	sync.RWMutex
	// TODO: Add locker bool?
}

// NewSearchSpace is a factory func for the SearchSpace T. Only requirement
// is a maximum capacity -- return will be false if that is < 1.
func NewSearchSpace(maxCap int) (*SearchSpace, bool) {
	if maxCap < 1 {
		return nil, false
	}

	ss := &SearchSpace{items: make([]DistancerContainer, 0, maxCap)}
	return ss, true
}

// Len gives the current len of the search space.
func (ss *SearchSpace) Len() int {
	ss.RLock()
	defer ss.RUnlock()
	return len(ss.items)
}

// Cap gives the current capacity of the search space.
func (ss *SearchSpace) Cap() int {
	ss.RLock()
	defer ss.RUnlock()
	return cap(ss.items)
}

// AddSearchable is the only way of adding data to this search space (do look
// at the clean() and clear() methods, those are the only way to delete data).
// There are a few rules for adding data here:
//	-	All of vectors must be of equal length. To be specific, all integers
//		from dc.Distancer().Dim() must be the same.
//	-	The rule above does not apply if the SearchSpace.Len() == 0.
//	-	SearchSpace.Len() will never be greater than SearchSpace.Cap(). So if
//		SearchSpace.Len() >= SearchSpace.Cap(), then theis will abort.
func (ss *SearchSpace) AddSearchable(dc DistancerContainer) bool {
	ss.Lock()
	defer ss.Unlock()

	if dc == nil {
		return false
	}

	d := dc.Distancer() // Validation.

	// All vecs in this ss must have an equal dimension. This is naturally not
	// enforced if ss.items is empty.
	if d.Dim() != ss.vecDim && len(ss.items) != 0 {
		return false
	}

	// Len of ss.items can never be higher than the capacity by design.
	if len(ss.items) >= cap(ss.items) {
		return false
	}

	// Can change the dim of this is to be the only member.
	if len(ss.items) == 0 {
		ss.vecDim = int(d.Dim())
	}

	ss.items = append(ss.items, dc)
	return true
}

// Clean is a controlled way of deleting data in this search space.
// DistancerContainer kept in this type can either give a valid
// mathx.Distancer or a nil -- the latter is interpreted as a mark for
// deletion and will be removed when calling this Clean() method.
func (ss *SearchSpace) Clean() {
	ss.Lock()
	defer ss.Unlock()
	i := 0
	for i < len(ss.items) {
		// NOTE: Checking nil with 'ss.items[i].Distancer() == nil'
		// will not work if it's actually nil, due to some odd
		// internatl (Go) behaviour. Do not change without running
		// the unit test for this func.
		if reflect.ValueOf(ss.items[i].Distancer()).IsNil() {
			// _Should_ be re-sliced with O(1) going by Go docs/code.
			ss.items = append(ss.items[:i], ss.items[i+1:]...)
			continue
		}
		i++
	}
}

// Clear will delete _all_ data in this search space.
func (ss *SearchSpace) Clear() {
	ss.Lock()
	defer ss.Unlock()
	ss.items = make([]DistancerContainer, 0, cap(ss.items))
}
