package knnc

import (
	"sync"
	"time"
)

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

// MapStageArgs is intended for the MapStage func.
type MapStageArgs struct {
	// In is a readable ScanItem chan. Workers will read from this.
	In <-chan ScanItem
	// Each worker will read from the 'In' field of this struct (<-chan ScanItem),
	// then use this func to transform the ScanItem. Note; false will drop ScanItem.
	MapFunc func(Distancer) (ScoreItem, bool)
	BaseStageArgs
}

// Ok validates MapStageArgs. Returns true iff:
// 	(1) args.In != nil
//	(2) args.MapFunc != nil,
//	(3) args.BaseStageArgs returns true on its Ok().
func (args *MapStageArgs) Ok() bool {
	return boolsOk([]bool{
		args.In != nil,
		args.MapFunc != nil,
		args.BaseStageArgs.Ok(),
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
			for scanItem := range args.In {
				// Distancer might have become nil while in the queue.
				if scanItem.Distancer == nil {
					continue
				}

				scoreItem, ok := args.MapFunc(scanItem.Distancer)
				if !ok {
					continue
				}
				scoreItem.set = true
				scoreItem.ID = scanItem.ID

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

// FilterStageArgs is intended for the FilterStage func.
type FilterStageArgs struct {
	// In is a readable ScoreItem chan. Workers will read from this.
	In <-chan ScoreItem
	// FilterFunc is what each worker will use to evaluate whether or not
	// to filter a ScoreItem out (i.e drop). Return false = drop.
	FilterFunc func(ScoreItem) bool
	BaseStageArgs
}

// Ok valiadtes FilterStageArgs. Returns true iff:
//	(1) args.In != nil
//	(2)	args.FilterFunc != nil,
//	(3) args.BaseStageArgs returns true on its Ok().
func (args *FilterStageArgs) Ok() bool {
	return boolsOk([]bool{
		args.In != nil,
		args.FilterFunc != nil,
		args.BaseStageArgs.Ok(),
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

// MergeStageArgs is intended for the MergeStage func.
type MergeStageArgs struct {
	// In is a readable ScoreItem chan. Workers will read from this.
	In <-chan ScoreItem
	// K as the K in KNN.
	K int
	// Ascending specifies whether the resulting scores (from the 'In' chan)
	// should be ordered in ascending (or descending) order.
	Ascending bool
	// SendInterval specifies how often the MergeStage should stream out results.
	// This is included because the function is particularly costly, as it streams
	// using <chan ScoreItems> (plural). Each worker will receive ScoreItem instances
	// through the 'In' chan, then merge them into _each_their_own ScoreItems. These
	// slices will be sent into the output stream with the interval specified here.
	// 1 = on each recv and merge.
	// 2 = every second recv and merge.
	// 3 = etc.
	SendInterval int
	BaseStageArgs
}

// Ok validates MergeStageArgs. Returns true iff:
//	(1) args.In != nil
//	(2) args.K > 0
//	(3) args.SendInterval > 0
//	(4) args.BaseStageArgs.Ok() == true
func (args *MergeStageArgs) Ok() bool {
	return boolsOk([]bool{
		args.In != nil,
		args.K > 0,
		args.SendInterval > 0,
		args.BaseStageArgs.Ok(),
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
		// Safety in copy since it's uncertain how the slice will be used downstream.
		cp := make(ScoreItems, len(scoreItems))
		copy(cp, scoreItems)

		select {
		case out <- cp:
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

			scoreItems := make(ScoreItems, args.K)
			i := 0
			for scoreItem := range args.In {
				scoreItems.BubbleInsert(scoreItem, args.Ascending)

				if i%args.SendInterval == 0 {
					if !trySend(scoreItems.Trim()) {
						return
					}
				}
				i++
			}

			trySend(scoreItems.Trim())
		}()
	}

	go func() { wg.Wait(); close(out) }()

	return out, true
}
