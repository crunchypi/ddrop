package knnc

import (
	"reflect"
	"sync"
	"time"
)

/*
File contains prefab stages (concurrency context) relevant to this pkg. They
are not a necessity, but defined nontheless due to their usefulness, relative
complexity and because they are easy to get wrong (everything is tested).
Specifically, they are:
- MapStage		(maps ScanItem instances into ScoreItem instances)
- FilterStage	(filters ScoreItem instances)
- MergeStage	(merges ScoreItem instances)

All stage functions listed above have each their argument types (because that
makes func signatures shorter and separates out argument validation), which are
composed in multiple layers for flexibility purposes.
*/

/*
--------------------------------------------------------------------------------
BaseWorkerArgs and BaseStageArgs, along with argument validation methods.
--------------------------------------------------------------------------------
*/

// BaseWorkerArgs contains arguments for a single worker (concurrency).
type BaseWorkerArgs struct {
	// Buf specifies the output chan buffer for this worker. Must be >= 0.
	Buf int
	// Cancel is a way of explicitly cancelling this worker and making it exit.
	// Also see 'BlockDeadline' (this struct), it is a time-based failsafe.
	// Must be initialized correctly (see CancelSignal doc).
	Cancel *CancelSignal
	// BlockDeadline is time-based failsafe for this worker, intended for leak
	// prevention. Also see 'Cancel' (this struct) for explicit cancellation.
	// Must be > 0.
	BlockDeadline time.Duration
	// UnsafeDoneCallback is called when a gorougine is done. It is named as
	// unsafe because it is done in a goroutine (i.e concurrently) and the
	// safety depends on usage. May be nil.
	UnsafeDoneCallback func()
}

// Ok validates BaseWorkerArgs. Returns true iff:
// 	(1) args.Buf >0 0
//	(2) args.CancelSignal was initialized correctly (with NewCancelSignal()).
//	(3) args.BlockDeadline > 0.
func (args *BaseWorkerArgs) Ok() bool {
	return boolsOk([]bool{
		args.Buf >= 0,
		args.Cancel.c != nil,
		args.BlockDeadline > 0,
	})
}

// BaseStageArgs contains arguments common for stages (concurrency). Specifically,
// it nests BaseWorkerArgs (this pkg) and NWorkers int.
type BaseStageArgs struct {
	// NWorkers represents number of workers used in a stage.
	NWorkers int
	BaseWorkerArgs
}

// Ok validates BaseStageArgs. Returns true iff:
//	(1) args.NWorkers >= 1
//	(2)	args.BaseWorkerArgs returns true on its Ok().
func (args *BaseStageArgs) Ok() bool {
	return boolsOk([]bool{
		args.NWorkers >= 1,
		args.BaseWorkerArgs.Ok(),
	})
}

/*
--------------------------------------------------------------------------------
MapStage func and argument types (along with validation methods).
--------------------------------------------------------------------------------
*/

// MapStagePartialArgs is intended as partial args for MapStageArgs.
// Extracted as a separate struct for additional flexibility.
type MapStagePartialArgs struct {
	// Each worker will read from the 'In' field of this struct (<-chan ScanItem),
	// then use this func to transform the ScanItem. Note; false will drop ScanItem.
	MapFunc func(Distancer) (ScoreItem, bool)
	BaseStageArgs
}

// Ok validates MapStagePartialArgs. Returns true iff:
//	(1) args.MapFunc != nil
//	(2) args.BaseStageArgs (embedded) returns true on its Ok().
func (args *MapStagePartialArgs) Ok() bool {
	return boolsOk([]bool{
		args.MapFunc != nil,
		args.BaseStageArgs.Ok(),
	})
}

// MapStageArgs is intended for the MapStage func.
type MapStageArgs struct {
	// In is a readable ScanItem chan. Workers will read from this.
	In <-chan ScanItem
	MapStagePartialArgs
}

// Ok validates MapStageArgs. Returns true iff:
// 	(1) args.In != nil
//	(2) args.MapStagePartial (embedded) returns true on its Ok().
func (args *MapStageArgs) Ok() bool {
	return boolsOk([]bool{
		args.In != nil,
		args.MapStagePartialArgs.Ok(),
	})
}

// MapStage is a stage (concurrency context) where some input is transformed to
// some output. To be specific, data will be read from args.In, transformed with
// args.MapFunc, and passed along to the channel returned from this func. See
// documentation for MapStageArgs and the nested structs to get more details
// about the different parameters (such as MapStageArgs.BaseStageArgs.NWorkers).
// Note; return here will be (nil, false) if args.Ok() == false.
func MapStage(args MapStageArgs) (<-chan ScoreItem, bool) {
	if !args.Ok() {
		return nil, false
	}

	out := make(chan ScoreItem, args.Buf)
	wg := sync.WaitGroup{}
	wg.Add(args.NWorkers)

	for i := 0; i < args.NWorkers; i++ {
		go func() {
			defer wg.Done()
			if args.UnsafeDoneCallback != nil {
				defer args.UnsafeDoneCallback()
			}

			for scanItem := range args.In {
				// Distancer might have become nil while in the queue.
				// == nil check does not work as expected.
				if reflect.ValueOf(scanItem.Distancer).IsNil() {
					continue
				}

				scoreItem, ok := args.MapFunc(scanItem.Distancer)
				if !ok {
					continue
				}
				scoreItem.Distancer = scanItem.Distancer
				scoreItem.Set = true

				select {
				case out <- scoreItem:
				case <-args.Cancel.c:
					return
				case <-time.After(args.BlockDeadline):
					return
				}
			}
		}()
	}

	go func() { wg.Wait(); close(out) }()

	return out, true
}

/*
--------------------------------------------------------------------------------
FilterStage func and argument types (along with validation methods).
--------------------------------------------------------------------------------
*/

// FilterStagePartialArgs is intended as partial args for FilterStageArgs.
// Extracted as a separate struct for additional flexibility.
type FilterStagePartialArgs struct {
	// FilterFunc is what each worker will use to evaluate whether or not
	// to filter a ScoreItem out (i.e drop). Return false = drop.
	FilterFunc func(ScoreItem) bool
	BaseStageArgs
}

// Ok validates FilterStagePartialArgs. Returns true iff:
//	(1) args.FilterFunc != nil,
//	(2) args.BaseStageArgs (embedded) returns true on its Ok().
func (args *FilterStagePartialArgs) Ok() bool {
	return boolsOk([]bool{
		args.FilterFunc != nil,
		args.BaseStageArgs.Ok(),
	})
}

// FilterStageArgs is intended for the FilterStage func.
type FilterStageArgs struct {
	// In is a readable ScoreItem chan. Workers will read from this.
	In <-chan ScoreItem
	FilterStagePartialArgs
}

// Ok valiadtes FilterStageArgs. Returns true iff:
//	(1) args.In != nil
//	(2) args.FilterStagePartialArgs (embedded) return true on its Ok().
func (args *FilterStageArgs) Ok() bool {
	return boolsOk([]bool{
		args.In != nil,
		args.FilterStagePartialArgs.Ok(),
	})
}

// FilterStage is a stage (concurrency context) where some input can be filtered
// out, based on some criteria. To be specific, data will be read from args.In,
// then fed into args.FilterFunc, where a false return will cause data to be
// dropped -- everything else is pushed into the chan returned here. See
// documentation for FilterStageArgs and the nested structs to get more details
// about the different parameters (such as FilterStageArgs.BaseStageArgs.NWorkers).
// Note; return here will be (nil, false) if args.Ok() == false.
func FilterStage(args FilterStageArgs) (<-chan ScoreItem, bool) {
	if !args.Ok() {
		return nil, false
	}

	out := make(chan ScoreItem, args.Buf)
	wg := sync.WaitGroup{}
	wg.Add(args.NWorkers)

	for i := 0; i < args.NWorkers; i++ {
		go func() {
			defer wg.Done()
			if args.UnsafeDoneCallback != nil {
				defer args.UnsafeDoneCallback()
			}

			for scoreItem := range args.In {
				if !args.FilterFunc(scoreItem) {
					continue
				}

				select {
				case out <- scoreItem:
				case <-args.Cancel.c:
					return
				case <-time.After(args.BlockDeadline):
					return
				}
			}
		}()
	}

	go func() { wg.Wait(); close(out) }()

	return out, true
}

/*
--------------------------------------------------------------------------------
MergeStage func and argument types (along with validation methods).
--------------------------------------------------------------------------------
*/

// MergeStagePartialArgs is intended as partial args for MergeStageArgs.
// Extracted as a separate struct for additional flexibility.
type MergeStagePartialArgs struct {
	// K as the K in KNN.
	K int
	// Ascending specifies whether the resulting scores (from the 'In' chan)
	// should be ordered in ascending (or descending) order.
	Ascending bool
	// SendInterval specifies how often the MergeStage should stream out results.
	// This is included because the function is particularly costly, as it streams
	// using <chan ScoreItems> (plural). Each worker will receive ScoreItem instances
	// through the 'In' chan, then merge them into _each_their_own ScoreItems.
	// These ScoreItems instances will then be sent into the output stream with
	// the interval specified here:
	//	1 = send on each recv and merge.
	//	2 = send every second recv and merge.
	//	3 = etc.
	// Note that when a ScoreItems instance is sent, a new one will be created in
	// its place in a worker, so duplicate data will not be sent.
	SendInterval int
	BaseStageArgs
}

// Ok validates MergeStagePartialArgs. Returns true iff:
//	(1) args.K > 0
//	(2) args.SendInterval > 0
//	(3) args.BaseStageArgs (embedded) returns true on its Ok().
func (args *MergeStagePartialArgs) Ok() bool {
	return boolsOk([]bool{
		args.K > 0,
		args.SendInterval > 0,
		args.BaseStageArgs.Ok(),
	})
}

// MergeStageArgs is intended for the MergeStage func.
type MergeStageArgs struct {
	// In is a readable ScoreItem chan. Workers will read from this.
	In <-chan ScoreItem
	MergeStagePartialArgs
}

// Ok validates MergeStageArgs. Returns true iff:
//	(1) args.In != nil
//	(2) args.MergeStagePartialArgs (embedded) return strue on its Ok().
func (args *MergeStageArgs) Ok() bool {
	return boolsOk([]bool{
		args.In != nil,
		args.MergeStagePartialArgs.Ok(),
	})
}

// MergeStage is a stage (concurrency context) where input (args.In) is merged
// in an ordered fashion into ScoreItems (plural) instances, which are pushed
// out through the returned chan periodically. Specifically, it spawns workers
// (args.NWorkers), all merging the input into their _individual_ ScoreItems
// using the ScoreItems.BubbleInsert method (ascending arg = args.Ascending).
// Copies of these ordered ScoreItems are then pushed into the returned chan at
// the interval specified in args.SendInterval. As such, this is a particularly
// costly function and should be treated as such. For more information, see
// documentation for MergeStageArgs and the nested structs. Also note that
// the only condition for a false return is if args.Ok() == false.
func MergeStage(args MergeStageArgs) (<-chan ScoreItems, bool) {
	if !args.Ok() {
		return nil, false
	}

	out := make(chan ScoreItems, args.NWorkers)
	// Reduces code duplication. False means abort.
	trySend := func(scoreItems ScoreItems) bool {
		// No point in sending empty. Check before costly .Trim() call.
		if len(scoreItems) == 0 {
			return true
		}
		scoreItems = scoreItems.Trim()

		// Check again, might not be empty after trim.
		if len(scoreItems) == 0 {
			return true
		}

		select {
		case out <- scoreItems:
			return true
		case <-args.Cancel.c:
			return false
		case <-time.After(args.BlockDeadline):
			return false
		}
	}

	wg := sync.WaitGroup{}
	wg.Add(args.NWorkers)

	// Each goroutine will receive through the chan and merge into _each_their_own_
	// slice of ScoreItems, which will be streamed to the 'out' chan periodically.
	for i := 0; i < args.NWorkers; i++ {
		go func() {
			defer wg.Done()
			if args.UnsafeDoneCallback != nil {
				defer args.UnsafeDoneCallback()
			}

			scoreItems := make(ScoreItems, args.K)
			i := 1 // So it won't send on the first iter.
			for scoreItem := range args.In {
				scoreItems.BubbleInsert(scoreItem, args.Ascending)

				if i%args.SendInterval == 0 {
					if !trySend(scoreItems) {
						return
					}
					// A new copy _must_ be created; not doing so can lead to
					// the same ScoreItem instance to be sent multiple times.
					// That is a problem because the caller of this func can't
					// know whether or not the ScoreItems are duplicates or not,
					// and can't assume either case.
					scoreItems = make(ScoreItems, args.K)
				}
				i++
			}

			// No need in creating a duplicate as above (since this is last).
			trySend(scoreItems)
		}()
	}

	go func() { wg.Wait(); close(out) }()

	return out, true
}
