package knnc

import (
	"fmt"
	"reflect"
	"sync"
	"time"
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

	sync.RWMutex
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
	return boolsOk([]bool{
		args.SearchSpacesMaxCap > 0,
		args.SearchSpacesMaxN > 0,
		args.MaintenanceTaskInterval > 0,
	})
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
	ss.RLock()
	defer ss.RUnlock()

	distancersN := 0
	for _, searchSpace := range ss.searchSpaces {
		distancersN += searchSpace.Len()
	}

	return len(ss.searchSpaces), distancersN
}

// Cap returns the capacity of the internal slice of SearchSpace instances.
func (ss *SearchSpaces) Cap() int {
	ss.RLock()
	defer ss.RUnlock()

	return cap(ss.searchSpaces)
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
	ss.RLock()
	defer ss.RUnlock()

	if dc == nil {
		return false
	}

	d := dc.Distancer() // For validation.
	// == nil does not work as expected.
	if reflect.ValueOf(d).IsNil() {
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
			fmt.Println("added to searchspace")
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
	ss.Lock()
	defer ss.Unlock()

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
	ss.Lock()
	defer ss.Unlock()
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
	// Extent refers to the search extent. 1=scan all internal SearchSpace (singular)
	// instances _completely_, 0.5= scan 50% of all internal SearchSpace instances.
	Extent float64
	// The scanning routine counts as a concurrency stage, where each internal
	// SeachSpace instance counts as a worker, and will as such 'inherit' from
	// BaseStageArgs.BaseWorkerArgs.
	BaseStageArgs
}

// Ok validates SearchSpacesScanArgs. Returns true iff:
//	(1) args.Extent >= 0.0 and <= 1.0.
//	(2) args.BaseStageArgs.Ok() is true.
func (args *SearchSpacesScanArgs) Ok() bool {
	return boolsOk([]bool{
		args.Extent >= 0.0 && args.Extent <= 1.0,
		args.BaseStageArgs.Ok(),
	})
}

// Scan calls the method with the same name on internal SearchSpace instances
// and pushes their ScanChan returns to the chan returned here (i.e chan of chans).
// The process is done in a controlle way such that number of active scanners does
// not exceed args.BaseStageArgs.NWorkers. See documentation for SearchSpacesScanArgs
// for more details.
func (ss *SearchSpaces) Scan(args SearchSpacesScanArgs) (<-chan ScanChan, bool) {
	if !args.Ok() {
		return nil, false
	}

	// No point in proceeding if this is not ok (should be, but doing for more
	// robustness)- and no point in re-creating this on each loop iter below.
	inheritedArgs := SearchSpaceScanArgs{
		Extent:         args.Extent,
		BaseWorkerArgs: args.BaseWorkerArgs,
	}
	if ok := inheritedArgs.Ok(); !ok {
		return nil, false
	}
	// This method will keep a consistent number of goroutines active at a time.
	// To do this, the callback behaviour of SearchSpace.Scan workers is used.
	oldCallback := inheritedArgs.UnsafeDoneCallback
	ticker := ActiveGoroutinesTicker{}
	decrement := ticker.AddAwait()
	decrement() // ticker back to 0.
	inheritedArgs.UnsafeDoneCallback = func() {
		decrement()
		if oldCallback != nil {
			oldCallback()
		}
	}

	out := make(chan ScanChan, args.Buf)
	go func() {
		defer close(out)
		ss.RLock()
		defer ss.RUnlock()

		// Used for constraining the max amount of goroutines running at a time.
		for _, searchSpace := range ss.searchSpaces {
			ticker.BlockUntilBelowN(args.NWorkers + 1)
			ch, ok := searchSpace.Scan(inheritedArgs)
			if !ok {
				continue
			}
			// Increment ticker and ignore the returned decrement callback, as
			// SearchSpace.Scan workers will do that (defined in the block further
			// up, where UnsafeDoneCallback is set)l
			ticker.AddAwait()

			select {
			case out <- ch:
			case <-args.Cancel.c:
				return
			case <-time.After(args.BlockDeadline):
				return
			}
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
	ss.Lock()
	defer ss.Unlock()

	if ss.maintenanceActive {
		return
	}

	ss.maintenanceActive = true
	go func() {
		// Cleanup, covering all exit paths.
		defer func() {
			ss.Lock()
			defer ss.Unlock()
			ss.maintenanceActive = false
		}()

		// Each step in the cleaning process. Simply used for scope and the
		// convenience of deferring mutex unlock which covers all exit paths.
		// The return is for exiting the outer func, i.e StartMaintenance.
		cursor := 0
		stepf := func() bool {
			ss.Lock()
			defer ss.Unlock()

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
	ss.Lock()
	defer ss.Unlock()

	ss.maintenanceActive = false
}

// CheckMaintenance returns true if the maintenance task loop is active.
func (ss *SearchSpaces) CheckMaintenance() bool {
	ss.RLock()
	defer ss.RUnlock()

	return ss.maintenanceActive
}
