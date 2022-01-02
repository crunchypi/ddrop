package knnc

import (
	"reflect"
	"sync"
	"time"

	"github.com/crunchypi/ddrop/pkg/mathx"
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
	// == nil does not work as expected.
	if reflect.ValueOf(d).IsNil() {
		return false
	}

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

// ScanItem is a single/atomic item output from a SearchSpace.Scan.
type ScanItem struct {
	ID        string
	Distancer mathx.Distancer
}

// ScanChan is the return of SearchSpace.Scan. It is a chan of ScanItem.
type ScanChan <-chan ScanItem

// ScanArgs is intended for SearchSpace.Scan().
type ScanArgs struct {
	// Extend refers to the search extent. 1=scan whole searchspace, 0.5=half.
	// Must be >= 0.0 and <= 1.0.
	Extent float64
	BaseWorkerArgs
}

// Ok validates ScanArgs. Returns true iff:
//	(1) args.Extend >= 0.0 and <= 1.0.
//	(2) Embedded BaseWorkerArgs.Ok() is true.
func (args *ScanArgs) Ok() bool {
	return boolsOk([]bool{
		// Not strinctly needed but is an indicator of logic flaw.
		args.Extent >= 0.0 && args.Extent <= 1.0,
		args.BaseWorkerArgs.Ok(),
	})
}

// Scan starts a scanner worker which scans the SearchSpace (i.e not blocking).
// Returns is (ScanChan, true) if args.Ok() == true, else return is (nil, false).
// See ScanArgs and BaseWorkerArgs (embedded in ScanArgs) for argument details.
// Note, scanner uses 'read mutex', so will not block multiple concurrent scans.
func (ss *SearchSpace) Scan(args ScanArgs) (ScanChan, bool) {
	if !args.Ok() {
		return nil, false
	}

	out := make(chan ScanItem, args.Buf)
	go func() {
		defer close(out)
		ss.RLock()
		defer ss.RUnlock()

		// Adjusted loop iteration to accommodate the specified search extent.
		l := len(ss.items)
		checkN := float64(l) * args.Extent
		iterStep := l / int(checkN)
		remainder := l % int(checkN)

		i := 0
		for i < l {
			if distancer := ss.items[i].Distancer(); distancer != nil {
				select {
				case out <- ScanItem{ID: ss.items[i].ID(), Distancer: distancer}:
				case <-args.Cancel.c:
					return
				case <-time.After(args.BlockDeadline):
					return
				}
			}

			i += iterStep
			if remainder > 0 {
				remainder--
				i++
			}
		}
	}()
	return out, true
}
