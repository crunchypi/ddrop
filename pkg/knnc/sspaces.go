package knnc

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/crunchypi/ddrop/pkg/syncx"
)

/*
File contains code for a 'SearchSpaces' (plural) type, which represents a
collection of SearchSpace (singular) instances. The intended responsebility of
SearchSpaces T is to manage data, for instance cleaning and scanning.
*/

// SearchSpaces is intended for managing a collection of SearchSpace (singular)
// instances and their data, specifically in the context of cleaning and scanning.
type SearchSpaces struct {
	searchSpaces       []*SearchSpace
	searchSpacesMaxCap int // Max cap per SearchSpace.
	uniformVecDim      int // Vector dimension consistency.

	// For task loop.
	maintenanceTaskInterval time.Duration
	maintenanceActive       bool // If task loop started. Not for each step.

	mx sync.RWMutex
}

// NewSearchSpacesArgs is intended as an argument to the NewSearchSpaces func.
type NewSearchSpacesArgs struct {
	// SearchSpacesMaxCap represents the max data capacity in each SearchSpace
	// instance. This is 'inherited' each time those instances are instantiated.
	SearchSpacesMaxCap int
	// SearchSpacesMaxN represents the maximum amount of allowed SearchSpace
	// instances kept in SearchSpaces (plural).
	SearchSpacesMaxN int
	// MaintenanceTaskInterval is a _suggestion_ of how often the internal task
	// loop is ran. See SearchSpaces.StartMaintenance method for more info.
	MaintenanceTaskInterval time.Duration
}

// Ok validates NewSearchSpaceArgs. Returns true iff:
//	(1) args.SearchSpacesMaxCap > 0
//	(2) args.SearchSpacesMaxN > 0
//	(3)	args.MaintenanceTaskInterval > 0
func (args *NewSearchSpacesArgs) Ok() bool {
	ok := true
	ok = ok && args.SearchSpacesMaxN > 0
	ok = ok && args.SearchSpacesMaxCap > 0
	ok = ok && args.MaintenanceTaskInterval > 0
	return ok
}

// NewSearchSpaces is a factory func for SearchSpaces T. Returns (nil, false)
// if args.Ok() == false.
func NewSearchSpaces(args NewSearchSpacesArgs) (*SearchSpaces, bool) {
	if !args.Ok() {
		return nil, false
	}

	ss := SearchSpaces{
		searchSpaces:            make([]*SearchSpace, 0, args.SearchSpacesMaxN),
		searchSpacesMaxCap:      args.SearchSpacesMaxCap,
		maintenanceTaskInterval: args.MaintenanceTaskInterval,
	}
	return &ss, true
}

// Len returns a tuple where [0] = number of internal SearchSpace instances,
// and [1] = sum of all their Len method returns (i.e num of all data).
func (ss *SearchSpaces) Len() (int, int) {
	ss.mx.RLock()
	defer ss.mx.RUnlock()

	distancersN := 0
	for _, searchSpace := range ss.searchSpaces {
		distancersN += searchSpace.Len()
	}

	return len(ss.searchSpaces), distancersN
}

// Cap returns the capacity of the internal slice of SearchSpace instances.
func (ss *SearchSpaces) Cap() int {
	ss.mx.RLock()
	defer ss.mx.RUnlock()

	return cap(ss.searchSpaces)
}

// Dim returns the dimension of all internal data of all internal SearchSpace
// instances. Not that the dim can/will be overridden if SearchSpaces.Len()
// returns 0 on the first int (i.e no SearchSpace instances). This is handled
// automatically in SearchSpaces.AddSearchable(...).
func (ss *SearchSpaces) Dim() int {
	ss.mx.RLock()
	defer ss.mx.RUnlock()
	return ss.uniformVecDim
}

// AddSearchable is the only way of adding data to this instance; specifically
// to internal SearchSpace instances. There is a set of conditions wehre data
// can't be added:
// -	dc must not be nil.
// -	dc.Distancer() must not be nil.
// -	All distancer dimensions must be equal (dc.Distancer().Dim()); this rule
//  	does not apply if SearchSpaces.Len() == 0 and a new one is created.
// -	None internal SearchSpace (singular) could add, due to their capacities,
// -	Same as above _and_ if a new SearchSpace instance can't be created due
//		to the capacity limit of this SearchSpaces instance.
func (ss *SearchSpaces) AddSearchable(dc DistancerContainer) bool {
	ss.mx.RLock()
	defer ss.mx.RUnlock()

	if dc == nil {
		return false
	}

	d := dc.Distancer() // For validation.
	// == nil does not work as expected.
	if d == nil || reflect.ValueOf(d).IsNil() {
		return false
	}

	// All vecs in this ss must have an equal dimension. This is naturally not
	// enforced if ss.searchSpaces is empty.
	if d.Dim() != ss.uniformVecDim && len(ss.searchSpaces) != 0 {
		return false
	}

	// Try adding to any.
	for _, searchSpace := range ss.searchSpaces {
		if ok := searchSpace.AddSearchable(dc); ok {
			return true
		}
	}

	// Tried to add in any (block above), but none could (no capacity, or some
	// other issue). If none sub-searchspaces could add and the max cap here is
	// reached, then new additions are restricted.
	if len(ss.searchSpaces) >= cap(ss.searchSpaces) {
		return false
	}

	// Capacities of all searchspaces reached (or none could add, simply), but
	// the capacity ss.searchSpaces is not reached -- try creating a new.
	newSearchSpace, ok := NewSearchSpace(ss.searchSpacesMaxCap)
	if !ok {
		return false
	}
	if ok := newSearchSpace.AddSearchable(dc); !ok {
		return false

	}
	ss.searchSpaces = append(ss.searchSpaces, newSearchSpace)

	// Allow new uniform vec dim if the new searchSpace is the only one.
	if len(ss.searchSpaces) == 1 {
		ss.uniformVecDim = d.Dim()
	}

	return true
}

// Clean is a controlled way of deleting data in this instance. It calls the
// method with the same name on all internal SearchSpace (singular) instances
// and deletes the ones which get completely emptied (len of 0).
func (ss *SearchSpaces) Clean() {
	ss.mx.Lock()
	defer ss.mx.Unlock()

	i := 0
	for i < len(ss.searchSpaces) {
		ss.searchSpaces[i].Clean()
		if ss.searchSpaces[i].Len() == 0 {
			// NOTE: It may be better to leave them empty because creating and
			// deleting them (allocation) is constly, though that comes with its
			// own disadvantages (keeping track of vectpr dimensions and unused
			// memory. Leaving this as a Note.
			ss.searchSpaces = append(ss.searchSpaces[:i], ss.searchSpaces[i+1:]...)
			continue
		}
		i++
	}
}

// Clear will reset the internal SearchSpace slice and return the old one.
func (ss *SearchSpaces) Clear() []*SearchSpace {
	ss.mx.Lock()
	defer ss.mx.Unlock()
	old := ss.searchSpaces
	ss.searchSpaces = make([]*SearchSpace, 0, cap(ss.searchSpaces))
	return old
}

// SearchSpacesScanArgs is intended for SearchSpaces.Scan(). Note that some of
// these fields will get passed to each internal SearchSpace (singular) when
// their 'Scan()' method is called. Those shared and 'inherited' fields are
// args.Extent and args.BaseStageArgs.BaseWorkerArgs, as those are required
// for SearchSpaceScanArgs (again, singular).
type SearchSpacesScanArgs struct {
	NWorkers int
	SearchSpaceScanArgs
}

// Ok validates SearchSpacesScanArgs. Returns true iff:
//  - args.Extent >= 0.0 and <= 1.0.
//	- args.SearchSpaceArgs.Ok() is true.
func (args *SearchSpacesScanArgs) Ok() bool {
	ok := true
	ok = ok && args.NWorkers > 0
	ok = ok && args.SearchSpaceScanArgs.Ok()
	return ok
}

func (ss *SearchSpaces) Scan(args SearchSpacesScanArgs) (ScanChan, bool) {
	if !args.Ok() {
		return nil, false
	}

	out := make(chan ScanItem, args.Buf)
	ctx, ctxStop := context.WithDeadline(args.Ctx, time.Now().Add(args.TTL))
	args.Ctx = ctx

	// Used for constraining the max amount of goroutines running at a time.
	ticker := ActiveGoroutinesTicker{}

	go func() {
		defer close(out)
		ss.mx.RLock()
		defer ss.mx.RUnlock()
		defer ctxStop()
		defer ticker.BlockUntilBelowN(1)

		for _, searchSpace := range ss.searchSpaces {
			ticker.BlockUntilBelowN(args.NWorkers)
			ch, ok := searchSpace.Scan(args.SearchSpaceScanArgs)
			if !ok {
				continue
			}

			decrement := ticker.AddAwait()
			go func(ch ScanChan, decrement func()) {
				defer decrement()
				syncx.ChanIter(syncx.ChanIterArgs[ScanItem]{
					In:  ch,
					Ctx: ctx,
					Rcv: func(element ScanItem) bool {
						return syncx.ChanSend(syncx.ChanSendArgs[ScanItem]{
							Out: out,
							Ctx: ctx,
							Elm: element,
						})
					},
				})
			}(ch, decrement)
		}
	}()
	return out, true
}

// StartMaintenance starts a task loop where internal data is cleaned and stale
// data is removed. Specifically, each step will run at approximately the interval
// specified when creating this instance (NewSearchSpacesArgs.MaintenanceTaskInterval).
// Each step will call the Clean() method on a _single_ SearchSpace instance, after
// which the instance will be removed if it does not have any data in it.
// Note, one maintenance task loop can be ran at a time, so calling this method twice
// in a row (without calling ss.StopMaintenance) will only spawn one worker.
func (ss *SearchSpaces) StartMaintenance() {
	ss.mx.Lock()
	defer ss.mx.Unlock()

	if ss.maintenanceActive {
		return
	}

	ss.maintenanceActive = true
	go func() {
		// Cleanup, covering all exit paths.
		defer func() {
			ss.mx.Lock()
			defer ss.mx.Unlock()
			ss.maintenanceActive = false
		}()

		// Each step in the cleaning process. Simply used for scope and the
		// convenience of deferring mutex unlock which covers all exit paths.
		// The return is for exiting the outer func, i.e StartMaintenance.
		cursor := 0
		stepf := func() bool {
			ss.mx.Lock()
			defer ss.mx.Unlock()

			// No maintenance if empty.
			if len(ss.searchSpaces) == 0 {
				return ss.maintenanceActive
			}

			// Wraparound.
			if cursor >= len(ss.searchSpaces) {
				cursor = 0
			}

			ss.searchSpaces[cursor].Clean()
			// Delete empty.
			if ss.searchSpaces[cursor].Len() == 0 {
				slice := ss.searchSpaces // Alias for shorter line length.
				ss.searchSpaces = append(slice[:cursor], slice[cursor+1:]...)
				return ss.maintenanceActive // Slice changed, so no cursor++ here.
			}

			cursor++
			return ss.maintenanceActive
		}

		for {
			time.Sleep(ss.maintenanceTaskInterval)

			if !stepf() {
				return
			}
		}
	}()
}

// StopMaintenance stops the internal maintenance task loop (if running).
func (ss *SearchSpaces) StopMaintenance() {
	ss.mx.Lock()
	defer ss.mx.Unlock()

	ss.maintenanceActive = false
}

// CheckMaintenance returns true if the maintenance task loop is active.
func (ss *SearchSpaces) CheckMaintenance() bool {
	ss.mx.RLock()
	defer ss.mx.RUnlock()

	return ss.maintenanceActive
}
