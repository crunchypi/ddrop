package knnc

import (
	"reflect"

	"github.com/crunchypi/ddrop/pkg/syncx"
)

// MapStageArgs are intended as args for the MapStage func.
type MapStageArgs struct {
	// NWorkers specifies the number of workers to use in this stage.
	NWorkers int
	// In is the input chan for this concurrent stage.
	In <-chan ScanItem
	// MapFunc specifies what each worker should do in this concurrent stage.
	// Specifically, elements from "In" (field of this struct) are read, given
	// to this MapFunc, then the returned results are written to the output chan
	// _if_ the bool is true.
	MapFunc func(Distancer) (ScoreItem, bool)

	syncx.StageArgsPartial
}

// Ok is used for validation and returns true if the following is true:
//  - args.NWorkers > 0
//  - args.In != nil
//  - args.MapFunc != nil
//  - args.StageArgsPartial.Ok() returns true.
func (args *MapStageArgs) Ok() bool {
	ok := true
	ok = ok && args.NWorkers > 0
	ok = ok && args.In != nil
	ok = ok && args.MapFunc != nil
	ok = ok && args.StageArgsPartial.Ok()
	return ok
}

// MapStage is a concurrent stage where elements from a chan are read, transformed
// with a mapping function, then potentially outputted to the returned chan. See
// docs of MapStageArgs for more details. Also note that this is a front-end for
// syncx.Stage, so docs for that are relevant as well. Returns false if args.Ok()
// returns false.
func MapStage(args MapStageArgs) (<-chan ScoreItem, bool) {
	if !args.Ok() {
		return nil, false
	}

	type T = ScanItem  // In
	type U = ScoreItem // Out

	return syncx.Stage(syncx.StageArgs[T, U]{
		StageArgsPartial: args.StageArgsPartial,
		In:               args.In,
		NWorkers:         args.NWorkers,
		TaskFunc: func(scanItem T) (U, bool) {
			d := scanItem.Distancer
			// Distancer might have become nil while in the queue.
			// == nil check does not work as expected.
			if d == nil || reflect.ValueOf(d).IsNil() {
				return U{}, false
			}

			u, ok := args.MapFunc(d)
			u.Distancer = d
			u.Set = true

			return u, ok
		},
	})
}

// FilterStageArgs is intended as args for the FilterStage func.
type FilterStageArgs struct {
	// NWorkers specifies the number of workers to use in this stage.
	NWorkers int
	// In is the input chan for this concurrent stage.
	In <-chan ScoreItem
	// FilterFunc specifies what each worker should do in this concurrent stage.
	// Specifically, elements from "In" (field of this struct) are read, given
	// to this FilterFunc, then passed along to the output chan if the filter func
	// returns true.
	FilterFunc func(ScoreItem) bool

	syncx.StageArgsPartial
}

// Ok is used for validation and returns true if the following is true:
//  - args.NWorkers > 0
//  - args.In != nil
//  - args.FilterFunc != nil
//  - args.StageArgsPartial.Ok() returns true.
func (args *FilterStageArgs) Ok() bool {
	ok := true
	ok = ok && args.NWorkers > 0
	ok = ok && args.In != nil
	ok = ok && args.FilterFunc != nil
	ok = ok && args.StageArgsPartial.Ok()
	return ok
}

// FilterStage is a concurrent stage where elements from a chan are either dropped
// or passed along to the output chan, depending on args.FilterFunc. See docs of
// FilterStageArgs for more details. Also note that this is a front-end for
// syncx.Stage, so docs for that are relevant as well. Returns false if args.Ok()
// returns false.
func FilterStage(args FilterStageArgs) (<-chan ScoreItem, bool) {
	if !args.Ok() {
		return nil, false
	}

	type T = ScoreItem // In
	type U = ScoreItem // Out

	return syncx.Stage(syncx.StageArgs[T, U]{
		StageArgsPartial: args.StageArgsPartial,
		In:               args.In,
		NWorkers:         args.NWorkers,
		TaskFunc: func(scoreItem T) (U, bool) {
			return scoreItem, args.FilterFunc(scoreItem)
		},
	})
}

// MergeStageArgs is intended as args for the MergeStage func.
type MergeStageArgs struct {
	// In is the input chan for this 'stage'.
	In <-chan ScoreItem
	// K is the number of elements of the ordered slice that is returned.
	K int
	// Ascending specifies whether the resulting scores (from the 'In' chan)
	// should be ordered in ascending (or descending) order.
	Ascending bool
	// SendInterval specifies how often the result slice should be sent to the
	// output chan, 1=on each insert, 2=on every second insert, etc. Do note that
	// one send will send the slice, then reset the internal slice such that no
	// sent elements are duplicated.
	SendInterval int

	syncx.StageArgsPartial
}

// Ok is used for validation and returns true if the following is true:
//  - args.In != nil
//  - args.K > 0
//  - args.SendInterval >= 1
//  - args.StageArgsPartial.Ok() returns true.
func (args *MergeStageArgs) Ok() bool {
	ok := true
	ok = ok && args.In != nil
	ok = ok && args.K > 0
	ok = ok && args.SendInterval >= 1
	ok = ok && args.StageArgsPartial.Ok()
	return ok
}

// MergeStage ScoreItem elements from the input chan, merges them into an ordered
// slice (using ScoreItems.BubbleInsert), then sends the items through the output
// chan at a given interval. See MergeStageArgs for more detail. Also note that
// this is a front-end for syncx.Stage (NWorkers=1), so docs for that are relevant
// as well. Returns false if args.Ok() returns false.
func MergeStage(args MergeStageArgs) (<-chan ScoreItems, bool) {
	if !args.Ok() {
		return nil, false
	}

	type T = ScoreItem  // In
	type U = ScoreItems // Out

	i := 0
	scoreItems := make(U, args.K)

    // TODO: currently, this MergeStage func can exit without sending the
    // remainders in scoreItems.
	return syncx.Stage(syncx.StageArgs[T, U]{
		StageArgsPartial: args.StageArgsPartial,
		In:               args.In,
		// NOTE; should not be anything else than 1. The reasons is that (1) the
		// scoreItems slice above is not mutex protected and (2) having more than
		// one workers makes this slower (verified with benchmarks, but the result
		// might differ with a very high args.K).
		NWorkers: 1,
		TaskFunc: func(scoreItem T) (U, bool) {
			scoreItems.BubbleInsert(scoreItem, args.Ascending)
			i++

			// Don't send yet.
			if i%args.SendInterval != 0 {
				return make(U, 0), false
			}

			// Block for sending. But clear old first so duplicate data isn't sent.
			send := scoreItems.Trim()
			scoreItems = make(U, args.K)

			return send, true
		},
	})
}
