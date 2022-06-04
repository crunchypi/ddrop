package requestman

import (
	"context"
	"time"

	"github.com/crunchypi/ddrop/pkg/knnc"
	"github.com/crunchypi/ddrop/pkg/mathx"
	"github.com/crunchypi/ddrop/pkg/syncx"
)

/*
File for primary types:
    KNNArgs         : Fairly customizable arguments for making KNN requests.
    KNNRequest      : Helper type for interfacing with pkg/knnc
    KNNEnqueueResult: Where knn request results go.
*/

// KNNMethod specifies the distance function used for a request.
type KNNMethod int

const (
	KNNMethodEuclideanDistance KNNMethod = iota
	KNNMethodCosineSimilarity
)

// Ok returns true if it the KNNMethod is defined in this pkg.
func (m *KNNMethod) Ok() bool {
	ok := false
	ok = ok || (*m) == KNNMethodEuclideanDistance
	ok = ok || (*m) == KNNMethodCosineSimilarity
	return ok
}

// KNNArgs are used as arguments for making KNN requests. Check if all the
// requirements are met with calling KNNArgs.Ok().
type KNNArgs struct {
	// Namespace is used to group search spaces together, based on logical
	// meaning, but also for having uniform vector dimensions.
	Namespace string
	// Priority specifies how important a KNN query is -- higher is better.
	// It influences the number of goroutines used, though not necessarily
	// a one-to-one mapping. Must be > 0.
	Priority int
	// QueryVec is used for similarity searching. Must not be nil, with a
	// length of > 0. Also, make sure the dimension is appropriate for the
	// KNNArgs.namespace field.
	QueryVec []float64
	// KNNMethod specifies the distance function used for the query.
	// KNNMethod.Ok() must return true.
	KNNMethod KNNMethod
	// Ascending plays a role with ordering _and_ the meaning is dependent
	// somewhat on the KNNArgs.KNNMethod field.
	//
	// Euclidean distance, for instance, works on the principle that lower
	// is better, so then it would make sense to have Ascending=true for
	// KNN. For K-furthest-neighs, Ascending=false has to be used, as that
	// would reverse the order. The exact opposite is true for Cosine simi.
	Ascending bool
	// K is the K in KNN. However, the actual result might be less than this
	// number, for multiple reasons. One of them is that there simply might
	// not be enough data to search. Another reason is that the underlying
	// knn pkg uses a few optimization tricks to trade accuracy for speed,
	// the reamainding fields below give more documentation.
	K int
	// Extent specifies the extent of a search, in a range (0, 1]. For
	// example, 0.5 will search half the search space. This is used to
	// trade accuracy for speed.
	Extent float64
	// Accept is another optimization trick; the search will be aborted
	// when there are KNNArgs.K results with better than KNNArgs.Accept
	// accuracy.
	Accept float64
	// Reject is another optimization trick; the knn search pipeline will
	// drop all values worse than this fairly early on, such that the
	// load on downstream processes/pipes gets alleviated. Do note that
	// this is evaluated before KNNArgs.Accept, so Accept can be cancelled
	// out by Reject.
	Reject float64
	// TTL specifies the deadline for a knn request. The pipeline will
	// start shutting down for this request after the deadline, but it
	// is a good idea to cancel it manually. After this duration, the
	// best-found results are given. Must be > 0.
	TTL time.Duration

	// Monitor true will register the KNN request (and results).
	Monitor bool
}

// Ok checks if KNNArgs meets the minimum configuration requirement.
// Returns true if:
//  r.Priority > 0,
//  r.QueryVec != nil,
//  len(r.QueryVec) > 0,
//  r.KNNMethod.Ok(),
//  r.K > 0,
//  r.Extent > 0 && r.Extent <= 1
//  r.TTL > 0
func (r *KNNArgs) Ok() bool {
	ok := true
	ok = ok && r.Priority > 0
	ok = ok && r.QueryVec != nil && len(r.QueryVec) > 0
	ok = ok && r.KNNMethod.Ok()
	ok = ok && r.K > 0
	ok = ok && r.Extent > 0 && r.Extent <= 1
	ok = ok && r.TTL > 0
	return ok
}

// KNNEnqueueResult is used to receive the results of a KNN request/query.
type KNNEnqueueResult struct {
	// Pipe is the destination of a KNN request/query.
	Pipe chan knnc.ScoreItems
    // Cancel can be used to cancel a request. This is done automatically
    // with KNNArgs.TTL when making a query, but the cancel func should
    // be called nontheless.
	Cancel context.CancelFunc
}

// knnRequest is a wrapper around KNNArgs and its primary purpose is to
// contain methods that directly interfaces pkg/knnc. In other words,
// it is the type uses KNNArgs to create a knn result, which is sent
// through knnRequest.enqueueResult.Pipe. Set it up with newKNNRequest.
type knnRequest struct {
	args *KNNArgs
	// Converted from args.QueryVec.
	queryVec mathx.Distancer
	//----------------------------------------------------------------
	// NOTE: For internal operations, these must be set for a query
	// to be processed with the KNNRequest.process() method.
	//----------------------------------------------------------------

	// When the query is created. Will be used in combination with
	// KNNArgs.TTL to know when to cancel a pipeline.
	created time.Time
	// Destination of the request.
	enqueueResult    KNNEnqueueResult
	enqueueResultCtx context.Context
}

// newKNNRequest is a convenience func for creating a knnRequest instance.
// It sets up the internal KNNEnqueueResult instance safely and sets the
// 'created' field to now.
//
// Note that this does not check the args. For safety, use knnRequest.Ok(),
// if that is needed.
func newKNNRequest(args *KNNArgs) knnRequest {
	ctx, ctxCancel := context.WithDeadline(context.Background(), time.Now().Add(args.TTL))

	return knnRequest{
		args:     args,
		queryVec: mathx.NewSafeVec(args.QueryVec...),
		created:  time.Now(),
		enqueueResult: KNNEnqueueResult{
			Pipe:   make(chan knnc.ScoreItems),
			Cancel: ctxCancel,
		},
		enqueueResultCtx: ctx,
	}
}

// Ok checks if the instance meets the minimum safety requirements.
// Returns true if:
//  r.args.Ok(),
//  r.enqueueResult.Cancel.Ok(),
//  r.enqueueResult.Pipe != nil,
//  r.created != default (time.Time{})
func (r *knnRequest) Ok() bool {
	ok := true
	ok = ok && r.args.Ok()
	ok = ok && r.queryVec != nil
	ok = ok && r.created != time.Time{}
	ok = ok && r.enqueueResult.Pipe != nil
	ok = ok && r.enqueueResult.Cancel != nil
	ok = ok && r.enqueueResultCtx != nil
	return ok
}

/*
--------------------------------------------------------------------------------
Below are methods that convert the request into convenience types and funcs that
interact with the knnc package.
--------------------------------------------------------------------------------
*/

// Shorthand def

// mapStageF is compatible with knnc.MapStage.
type mapStageF = func(knnc.ScanChan) (<-chan knnc.ScoreItem, bool)

// filterStageF is compatible with knnc.FilterStage.
type filterStageF = func(<-chan knnc.ScoreItem) (<-chan knnc.ScoreItem, bool)

// mergeStageF is compatible with knnc.MergeStage.
type mergeStageF = func(<-chan knnc.ScoreItem) (<-chan knnc.ScoreItems, bool)

// toStageArgsPartial tries to convert internal state into syncx.StageArgsPartial.
//  - Ctx: knnRequest.enqueueResultCtx
//  - TTL: knnRequest.args.TTL - time.Now().Sub(r.created)
//  - Buf: 100. See inline documentation for reasoning.
func (r *knnRequest) toStageArgsPartial() syncx.StageArgsPartial {
	return syncx.StageArgsPartial{
		Ctx: r.enqueueResultCtx,
		TTL: r.args.TTL - time.Now().Sub(r.created),

		// ---------------------------------------------------------------------
		// Tests show that the "Buf" field (chan buf in all knnc stages) plays a
		// large role in performance. Here is some stats for change in "Buf",
		// and changes in vector pool size (and dimensions). Note that all of
		// these were ran with 50 retries, using an _exhaustive_ knn search, so
		// no performance tricks were used (such as changing search extent).
		// Also, k=3 but 30 was checked as well -- marginal difference.
		// Also, test machine is m1 macbook air 2020.

		// Table 1: 100k, 3 dim.
		//
		// Buf of 0  : ~200ms
		// Buf of 1  : ~140ms
		// Buf of 5  : ~ 70ms
		// Buf of 10 : ~ 60ms
		// Buf of 100: ~ 45ms
		// Buf of 200: ~ 45ms

		// Table 2: 10k vecs, 3 dim
		//
		// Buf of 0  : ~20ms
		// Buf of 1  : ~13ms
		// Buf of 5  : ~8ms
		// Buf of 10 : ~7ms
		// Buf of 100: ~5ms
		// Buf of 200: ~5ms

		// Table 3: 10k vecs, 100 dim
		// Buf of 0  : ~25ms
		// Buf of 1  : ~16ms
		// Buf of 5  : ~10ms
		// Buf of 10 : ~10ms
		// Buf of 100: ~7ms
		// Buf of 200: ~7ms

		// Table 4: 100 vecs, 100 dim
		//
		// Buf of 0  : ~300 us
		// Buf of 1  : ~250 us
		// Buf of 5  : ~200 us
		// Buf of 10 : ~200 us
		// Buf of 100: ~150 us
		// Buf of 200: ~150 us

		// Almost all cases (tables 1,2,3) show that performance is increased
		// dramatically up to- and including Buf=100, with diminishing returns
		// after. There is surely a more precise way of finding optimal buffer,
		// but 100 is chosen here, based on the data shown above.
		// ---------------------------------------------------------------------

		Buf: 100,
	}
}

// toScanChan tries to use the internal state to create a knnc.ScanChan.
// Specifically:
//  knnc.SearchSpacesScanArgs.NWorkers = knnRequest.Priority.
//  knnc.SearchSpacesScanArgs.SearchSpacesScanArgs.Extent = knnRequest.args.Extent.
//  knnc.SearchSpacesScanArgs.SearchSpacesScanArgs.StageArgsPartial
//      = knnRequest.toStageArgsPartial().
func (r *knnRequest) toScanChan(ss *knnc.SearchSpaces) (knnc.ScanChan, bool) {
	if ss == nil {
		return nil, false
	}

	return ss.Scan(knnc.SearchSpacesScanArgs{
		NWorkers: r.args.Priority,
		SearchSpaceScanArgs: knnc.SearchSpaceScanArgs{
			Extent:           r.args.Extent,
			StageArgsPartial: r.toStageArgsPartial(),
		},
	})
}

// toMapFunc converts a knnRequest into a func that can be used with
// knnc.MapStageArgs.MapFunc. It is a func where 'other' is compared
// against the internal knnRequest.queryVec to produce a distance score, using
// distance method specifies with knnRequest.KNNMethod. That distance score is
// returned in the form of knnc.ScoreItem. The bool is whether the distance
// function succeeded or not.
func (r *knnRequest) toMapFunc() func(other mathx.Distancer) (knnc.ScoreItem, bool) {
	return func(other mathx.Distancer) (knnc.ScoreItem, bool) {
		score := 0.
		ok := true

		switch r.args.KNNMethod {
		case KNNMethodEuclideanDistance:
			score, ok = r.queryVec.EuclideanDistance(other)
		case KNNMethodCosineSimilarity:
			score, ok = r.queryVec.CosineSimilarity(other)
		default:
			return knnc.ScoreItem{}, false
		}

		return knnc.ScoreItem{Score: score}, ok
	}
}

// toMapStage uses the internal state in order to return a func which is equivalent
// to knnc.MapStage, but accepts only an input channel. The following cfg is used
// in knnc.MapStageArgs:
//  - NWorkers: knnRequest.args.Priority
//  - In: specified as args in the func returned here.
//  - MapFunc: knnRequest.toMapFunc()
//  - StageArgsPartial: knnRequest.toStageArgsPartial()
func (r *knnRequest) toMapStage() mapStageF {
	return func(in knnc.ScanChan) (<-chan knnc.ScoreItem, bool) {
		return knnc.MapStage(knnc.MapStageArgs{
			NWorkers:         r.args.Priority,
			In:               in,
			MapFunc:          r.toMapFunc(),
			StageArgsPartial: r.toStageArgsPartial(),
		})
	}
}

// toFilterFunc simply converts a knnRequest into a func that can be used with
// knnc.FilterStageArgs.FilterFunc. The returned func uses the internal
// knnRequest.args.Reject to filter out scores 'worse' than score.Score. The
// 'worse' part (higher/lower) is dependent on knnRequest.args.Ascending. To
// give a clearer picture of the behavior:
//  If Reject=1 and score.Score=2 and Ascending=true  -> return false
//  If Reject=2 and score.Score=1 and Ascending=true  -> return true
//
// ... and flipping the Ascending flag gives the opposite results.
func (r *knnRequest) toFilterFunc() func(score knnc.ScoreItem) bool {
	return func(score knnc.ScoreItem) bool {
		keep := false
		keep = keep || score.Score < r.args.Reject && r.args.Ascending
		keep = keep || score.Score > r.args.Reject && !r.args.Ascending
		return keep
	}
}

// toFilterStage uses the internal state in order to return a func which is
// equivalent to knnc.FilterStage, but accepts only an input channel. The following
// cfg is used in knnc.FilterStageArgs:
//  - NWorkers: knnRequest.args.Priority
//  - In: specified as args in the func returned here.
//  - FilterFunc: knnRequest.toFilterFunc()
//  - StageArgsPartial: knnRequest.toStageArgsPartial() 
func (r *knnRequest) toFilterStage() filterStageF {
	return func(in <-chan knnc.ScoreItem) (<-chan knnc.ScoreItem, bool) {
		return knnc.FilterStage(knnc.FilterStageArgs{
			NWorkers:         r.args.Priority,
			In:               in,
			FilterFunc:       r.toFilterFunc(),
			StageArgsPartial: r.toStageArgsPartial(),
		})
	}
}

// toMergeStage uses the internal state in order to a return a func which is 
// equivalent to knnc.MergeStage, but accepts only an input channel. The following
// cfg is used in knnc.MergeStageArgs:
//  - In: specified as args in the func returned here.
//  - K: knnRequest.args.K
//  - Ascending: knnRequest.args.Ascending
//  - SendInterval: knnRequest.args.K
//  - StageArgsPartial: knnRequest.toStageArgsPartial
func (r *knnRequest) toMergeStage() mergeStageF {
	return func(in <-chan knnc.ScoreItem) (<-chan knnc.ScoreItems, bool) {
		return knnc.MergeStage(knnc.MergeStageArgs{
			In:               in,
			K:                r.args.K,
			Ascending:        r.args.Ascending,
			SendInterval:     r.args.K, // TODO: rethink.
			StageArgsPartial: r.toStageArgsPartial(),
		})
	}
}

// startPipeline starts a concurrent pipeline, using the following stages:
//  - knnRequest.toScanChan
//  - knnRequest.toMapStage
//  - knnRequest.toFilterStage
//  - knnRequest.toMergeStage
func (r *knnRequest) startPipeline(ss *knnc.SearchSpaces) (<-chan knnc.ScoreItems, bool) {
	chScan, ok := r.toScanChan(ss)
	if !ok {
		return nil, false
	}

	chMap, ok := r.toMapStage()(chScan)
	if !ok {
		return nil, false
	}

	chFilter, ok := r.toFilterStage()(chMap)
	if !ok {
		return nil, false
	}

	chMerge, ok := r.toMergeStage()(chFilter)
	if !ok {
		return nil, false
	}

	return chMerge, true
}

// consume tries to use knnRequest.args in order to produce a knn result, using
// knnRequest.startPipeline (functionality of pkg/knnc), which are then forwarded 
// to knnRequest.enqueueResult.Pipe (closed on completion/fail). Fail cases are:
//  - knnRequest.Ok() return false
//  - knnRequest.enqueueResult.Pipe == nil
//  - knnRequest.enqueueResult.Cancel == nil
//  - knnRequest.startPipeline() returns false
func (r *knnRequest) consume(ss *knnc.SearchSpaces) bool {
	defer close(r.enqueueResult.Pipe)

	// Check args.
	if !r.Ok() {
		return false
	}

	// Check destination.
	if r.enqueueResult.Pipe == nil || r.enqueueResult.Cancel == nil {
		return false
	}

	chScoreItems, ok := r.startPipeline(ss)
	if !ok {
		return false
	}

	result := make(knnc.ScoreItems, r.args.K)
	// As a way of breaking outer loop from inner.
	func() {
		for scoreItems := range chScoreItems {
			for _, scoreItem := range scoreItems {
				// Mechanism for stopping the query when r.K amount of scores
				// are found with better than r.Accept scores.
				// TODO: move this to knnc.MergeStage.
				worst := result[len(result)-1]
				done := false
				done = done || worst.Score <= r.args.Accept && r.args.Ascending
				done = done || worst.Score >= r.args.Accept && !r.args.Ascending

				// done && worst.Set = true if there are k results and all of them
				// satisfy the qi.request.Accept scores. !(true) = stop.
				if done && worst.Set {
					r.enqueueResult.Cancel()
					return
				}
				result.BubbleInsert(scoreItem, r.args.Ascending)
			}
		}
	}()

	r.enqueueResult.Pipe <- result
	return true
}
