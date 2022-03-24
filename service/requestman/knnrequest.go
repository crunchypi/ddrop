package requestman

import (
    "time"
)

/*
File for primary types:
    KNNArgs         : Fairly customizable arguments for making KNN requests.
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
	Priority  int
    // QueryVec is used for similarity searching. Must not be nil, with a
    // length of > 0. Also, make sure the dimension is appropriate for the
    // KNNArgs.namespace field.
	QueryVec  []float64
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
	K         int
    // Extent specifies the extent of a search, in a range (0, 1]. For
    // example, 0.5 will search half the search space. This is used to
    // trade accuracy for speed. 
	Extent    float64
    // Accept is another optimization trick; the search will be aborted
    // when there are KNNArgs.K results with better than KNNArgs.Accept
    // accuracy.
	Accept    float64
    // Reject is another optimization trick; the knn search pipeline will
    // drop all values worse than this fairly early on, such that the
    // load on downstream processes/pipes gets alleviated. Do note that
    // this is evaluated before KNNArgs.Accept, so Accept can be cancelled
    // out by Reject.
	Reject    float64
    // TTL specifies the deadline for a knn request. The pipeline will
    // start shutting down for this request after the deadline, but it
    // is a good idea to cancel it manually. After this duration, the
    // best-found results are given. Must be > 0.
	TTL       time.Duration
}

// Ok checks if KNNArgs meets the minimum requirement.
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
