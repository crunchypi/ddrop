package requestman

import (
	"time"

	"github.com/crunchypi/ddrop/pkg/knnc"
	"github.com/crunchypi/ddrop/pkg/mathx"
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
	// Cancel can be used to cancel a request. Should be called when
	// the deadline for a request (e.g KNNArgs.TTL is exceeded after
	// a request is made).
	Cancel *knnc.CancelSignal
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
	enqueueResult KNNEnqueueResult
}

// newKNNRequest is a convenience func for creating a knnRequest instance.
// It sets up the internal KNNEnqueueResult instance safely and sets the
// 'created' field to now.
//
// Note that this does not check the args. For safety, use knnRequest.Ok(),
// if that is needed.
func newKNNRequest(args *KNNArgs) knnRequest {
	return knnRequest{
		args:     args,
		queryVec: mathx.NewSafeVec(args.QueryVec...),
		enqueueResult: KNNEnqueueResult{
			Pipe:   make(chan knnc.ScoreItems),
			Cancel: knnc.NewCancelSignal(),
		},
		created: time.Now(),
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
	ok = ok && r.enqueueResult.Cancel.Ok()
	ok = ok && r.enqueueResult.Pipe != nil
	ok = ok && r.created != time.Time{}
	return ok
}

/*
--------------------------------------------------------------------------------
Below are methods that convert the request into convenience types and funcs that
interact with the knnc package.
--------------------------------------------------------------------------------
*/

// Shorthand def

// mapStageF is compatible with knnc.NewPipelineArgs.MapStage.
type mapStageF = func(knnc.ScanChan) (<-chan knnc.ScoreItem, bool)

// filterStageF is compatible with knnc.NewPipelineArgs.FilterStage.
type filterStageF = func(<-chan knnc.ScoreItem) (<-chan knnc.ScoreItem, bool)

// mergeStageF is compatible with knnc.NewPipelineArgs.MergeStage.
type mergeStageF = func(<-chan knnc.ScoreItem) (<-chan knnc.ScoreItems, bool)

// toBaseWorkerArgs simply converts knnRequest into knnc.BaseWorkerArgs, using
// some state from the internal knnRequest.args. Specifically:
//  Buf:    knnRequest.args.Priority
//  Cancel: knnRequest.enqueueResult.Cancel
//  TTL:    knnRequest.args.TTL - (time since knnRequest.created)
func (r *knnRequest) toBaseWorkerArgs() knnc.BaseWorkerArgs {
	return knnc.BaseWorkerArgs{
		Buf:    r.args.Priority,
		Cancel: r.enqueueResult.Cancel,
		// No point in keeping workers alive for longer than is acceptable by the
		// query, as it is assumed that it'll cancel after that point anyway.
		TTL: r.args.TTL - time.Now().Sub(r.created),
	}
}

//
func (r *knnRequest) toScanChans(
	ss *knnc.SearchSpaces,
) (
	<-chan knnc.ScanChan,
	bool,
) {
	if ss == nil {
		return nil, false
	}

	return ss.Scan(knnc.SearchSpacesScanArgs{
		Extent:        r.args.Extent,
		BaseStageArgs: r.toBaseStageArgs(),
	})
}

// toBaseStageArgs simply converts a knnRequest to knnc.BaseStageArgs, using
// some state from the internal knnRequest.args. Specifically:
//  NWorkers:       knnRequest.args.Priority
//  BaseWorkerArgs: knnRequest.toBaseWorkerArgs()
func (r *knnRequest) toBaseStageArgs() knnc.BaseStageArgs {
	return knnc.BaseStageArgs{
		NWorkers:       r.args.Priority,
		BaseWorkerArgs: r.toBaseWorkerArgs(),
	}
}

// toMapFunc simply converts a knnRequest into a func that can be used with
// knnc.MapStagePartialArgs.MapFunc. It is a func where 'other' is compared
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

// toMapStage simply converts a knnRequest into a func that is compatible with
// knnc.NewPipelineArgs.MapStage. It uses knnc.MapStage and constructs its args
// with the following:
//  - MapStagePartialArgs.MapFunc = knnRequest.toMapFunc()
//  - MapStagePartialArgs.BaseStageArgs = knnRequest.toMapFunc()
func (r *knnRequest) toMapStage() mapStageF {
	return func(in knnc.ScanChan) (<-chan knnc.ScoreItem, bool) {
		return knnc.MapStage(knnc.MapStageArgs{
			In: in,
			MapStagePartialArgs: knnc.MapStagePartialArgs{
				MapFunc:       r.toMapFunc(),
				BaseStageArgs: r.toBaseStageArgs(),
			},
		})
	}
}

// toFilterFunc simply converts a knnRequest into a func that can be used with
// knnc.FilterStagePartialArgs.FilterFunc. The returned func uses the internal
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

// toFilterStage simply converts a knnRequest into a func that is compatible with
// knnc.NewPipelineArgs.FilterStage. It uses knnc.FilterStage and constructs its
// arguments with the following:
//  - knnc.FilterStagePartialArgs.FilterFunc = knnRequest.toFilterFunc()
//  - knnc.FilterStagePartialArgs.BaseStageArgs = knnRequest.toBaseStageArgs()
func (r *knnRequest) toFilterStage() filterStageF {
	return func(in <-chan knnc.ScoreItem) (<-chan knnc.ScoreItem, bool) {
		return knnc.FilterStage(knnc.FilterStageArgs{
			In: in,
			FilterStagePartialArgs: knnc.FilterStagePartialArgs{
				FilterFunc:    r.toFilterFunc(),
				BaseStageArgs: r.toBaseStageArgs(),
			},
		})
	}
}

// toMergeStage simply converts a knnRequest into a func that is compatible with
// knnc.NewPipelineArgs.MergeStage. It uses knnc.MergeStage and constructs its
// arguments with the following:
//  - knnc.MergeStagePartialArgs.K = knnRequest.args.K
//  - knnc.MergeStagePartialArgs.Ascending = knnRequest.args.Ascending
//  - knnc.MergeStagePartialArgs.BaseStageArgs = knnRequest.toBaseStageArgs()
func (r *knnRequest) toMergeStage() mergeStageF {
	return func(in <-chan knnc.ScoreItem) (<-chan knnc.ScoreItems, bool) {
		return knnc.MergeStage(knnc.MergeStageArgs{
			In: in,
			MergeStagePartialArgs: knnc.MergeStagePartialArgs{
				K:             r.args.K,
				Ascending:     r.args.Ascending,
				SendInterval:  2, // TODO, arbitrary.
				BaseStageArgs: r.toBaseStageArgs(),
			},
		})
	}
}

// toPipeline simply converts a knnRequest into a knnc.NewPipelineArgs that is
// fed into knnc.Pipeline, from which both the returns are returned here. The
// args are constructed as follows:
//  knnc.NewPipelineArgs.BaseWorkerArgs = knnRequest.toBaseWorkerArgs()
//  knnc.NewPipelineArgs.MapStage = knnRequest.toMapStage()
//  knnc.NewPipelineArgs.FilterStage = knnRequest.toFilterStage()
//  knnc.NewPipelineArgs.MergeStage = knnRequest.toMergeStage()
func (r *knnRequest) toPipeline() (*knnc.Pipeline, bool) {
	return knnc.NewPipeline(knnc.NewPipelineArgs{
		BaseWorkerArgs: r.toBaseWorkerArgs(),
		MapStage:       r.toMapStage(),
		FilterStage:    r.toFilterStage(),
		MergeStage:     r.toMergeStage(),
	})
}

// consume tries to create a pkg/knnc pipeline, send knnRequest.args (KNNArgs)
// through it, then forward the result to knnRequest.enqueueResult.Pipe. Fail
// cases are (where r=this instance of knnRequest):
//  - 1 r.Ok() == false
//  - 2 r.enqueueResult.Pipe == nil
//  - 3 r.enqueueResylt.Cancel == nil
//  - 4 (arg) ss.Scan(...) failed (using r.args.Extent and r.toBaseStageARgs())
//  - 5 r.toPipeline() returned false
//
// In all cases, the r.enqueueResult.Pipe chan will be closed. In case 5,
// r.enqueueResult.Cancel will be cancelled.
//
// Additionally, this method also uses the r.args.Accept field to abort a search
// when enough (r.args.K) elements of sufficient quality are found.
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

	// Try start scan(ners).
	scanChans, ok := ss.Scan(knnc.SearchSpacesScanArgs{
		Extent:        r.args.Extent,
		BaseStageArgs: r.toBaseStageArgs(),
	})
	if !ok {
		return false
	}

	// Try start pipeline.
	pipeline, ok := r.toPipeline()
	if !ok {
		// Setup of at least one stage (in toPipeline) failed, kill all of them,
		// including the scanner in the previous block.
		r.enqueueResult.Cancel.Cancel()
		return false
	}

	// Push faucet -> pipeline.
	go func() {
		defer pipeline.WaitThenClose()
		for scanChan := range scanChans {
			if !pipeline.AddScanner(scanChan) {
				return
			}
		}
	}()

	result := make(knnc.ScoreItems, r.args.K)
	pipeline.ConsumeIter(func(scoreItems knnc.ScoreItems) bool {
		for _, scoreItem := range scoreItems {
			// Mechanism for stopping the query when r.K amoung of scores
			// are found with better than r.Accept scores.
			worst := result[len(result)-1]
			done := false
			done = done || worst.Score <= r.args.Accept && r.args.Ascending
			done = done || worst.Score >= r.args.Accept && !r.args.Ascending

			// done && worst.Set = true if there are k results and all of them
			// satisfy the qi.request.Accept scores. !(true) = stop.
			if done && worst.Set {
				r.enqueueResult.Cancel.Cancel()
				return false
			}
			result.BubbleInsert(scoreItem, r.args.Ascending)
		}
		return true
	})

	r.enqueueResult.Pipe <- result
	return true
}
