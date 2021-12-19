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
	Cancel CancelSignal
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
