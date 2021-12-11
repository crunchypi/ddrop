/*
This pkg contains a general-purpose k-nearest-neighbours implementation.
It is based on the use of generators, which makes it especially flexible.
*/
package knn

import (
	"math"

	"github.com/crunchypi/ddrop/pkg/mathx"
)

// VecPoolGenerator is used for clarity. It represents a generator function
// which is supposed iterate over- and return vectors ([]float) one by one.
type VecPoolGenerator func() ([]float64, bool)

// KNNBruteArgs are used as args for the KNNBrute func in this pkg. Run the 'ok'
// method of this type to checks that it's valid (no nils and k > 0).
type KNNBruteArgs struct {
	// SearchVec is the search vector in the knn routine.
	SearchVec []float64
	// VecPoolGenerator is a func that generates vectors which will be compared
	// to the SearchVec (this struct) in the knn routine. Note that the
	// generated vectors might have to be of equal len as the SearchVector,
	// though this depends on how the DistanceFunc (this struct) handles that.
	VecPoolGenerator VecPoolGenerator
	// DistanceFunc compares the SearchVec (this struct) against vecs yielded
	// from VecPoolGenerator (also this struct) and finds their distance.
	// Examples are Euclidean distance, or cosine similarity, which can be found
	// in the mathx pkg at the time of writing (211110). Do note that a false
	// return will skip the current vector generated from VecPoolGenerator.
	DistanceFunc func(v1, v2 []float64) (float64, bool)
	// K stands for the k in k-nearest-neighbours.
	K int
	// Ascending specifies the result order in the func which this struct is
	// used as an argument for. This is intended for two purposes: (A) for the
	// use of different distance functions, as specified with DistanceFunc (this
	// struct). For instance, lower is better with cosine similarity, while the
	// inverse is true for Euclidean distance. The other purpose (B) is to get
	// k-furthest-neighbours instead of nearest.
	Ascending bool
}

// Ok checks that there are no nils and k > 0.
func (args *KNNBruteArgs) Ok() bool {
	return boolsOk([]bool{
		args.SearchVec != nil,
		args.VecPoolGenerator != nil,
		args.DistanceFunc != nil,
		args.K > 0,
	})
}

// KNNBrute is a general-purpose k-nearest-neighbours function. For details
// about the argument, see comments for KNNBruteArgs (type in this pkg).
// Returns false if the args.Ok() check fails. The []int return will represent
// index pointrs to the 'nearest' items in args.VecPoolGenerator.
func KNNBrute(args KNNBruteArgs) ([]int, bool) {
	if !args.Ok() {
		return nil, false
	}

	r := make(resultItems, args.K)

	// Apply worst score to all resultItems
	similarity := math.MaxFloat64
	if !args.Ascending {
		similarity *= -1
	}
	for i := 0; i < args.K; i++ {
		r[i].score = similarity
	}

	// Unusual loop since the bounds of args.VecPoolGenerator is unknown.
	i := 0
	for {
		// Next vector.
		v, cont := args.VecPoolGenerator()
		if !cont {
			break
		}

		// Next score.
		score, ok := args.DistanceFunc(args.SearchVec, v)
		if !ok {
			i++
			continue
		}

		// Eval include.
		r.bubbleInsert(resultItem{i, score, true}, args.Ascending)
		i++
	}
	return r.toIndexes(), true
}

// KNNBruteDistArgs are used as args for the KNNBruteDist func in this pkg.
// Run the 'Ok' method method of this type to check that it's valid.
type KNNBruteDistArgs struct {
	// Query is the query in the knn routine.
	Query mathx.Distancer
	// Pool are all vectors that will be compared against the Query.
	Pool []mathx.Distancer
	// Mapper is used to compare the Query to each item in Pool. There, the
	// intent is to call some distance function (such as d1.EuclideanDistance(d2)).
	Mapper func(d1, d2 mathx.Distancer) (float64, bool)
	// K is the k in knn.
	K int
	// Ascending specifies the result order in the func which this struct is
	// used as an argument for. This is intended for two purposes: (A) for the
	// use of different distance functions, as specified with DistanceFunc (this
	// struct). For instance, lower is better with cosine similarity, while the
	// inverse is true for Euclidean distance. The other purpose (B) is to get
	// k-furthest-neighbours instead of nearest.
	Ascending bool
}

// Ok checks that there are no nils and k > 0.
func (args *KNNBruteDistArgs) Ok() bool {
	return boolsOk([]bool{
		args.Query != nil,
		args.Pool != nil,
		args.Mapper != nil,
		args.K > 0,
	})
}

// KNNBruteDist is similar to KNNBrute func (this pkg) but operates on mathx.Distancer
// instances instead and does so eagerly -- see docs for KNNBruteDistArgs for details
// about arguments. Returns false if the args.Ok() check fails. The []int return will
// represent index pointrs to the 'nearest' items in args.Pool.
func KNNBruteDist(args KNNBruteDistArgs) ([]int, bool) {
	if !args.Ok() {
		return nil, false
	}

	r := make(resultItems, args.K)

	// Apply worst score to all resultItems
	similarity := math.MaxFloat64
	if !args.Ascending {
		similarity *= -1
	}
	for i := 0; i < args.K; i++ {
		r[i].score = similarity
	}

	for i, other := range args.Pool {
		if other == nil {
			continue
		}

		score, ok := args.Mapper(args.Query, other)
		if !ok {
			continue
		}
		r.bubbleInsert(resultItem{i, score, true}, args.Ascending)
	}

	return r.toIndexes(), true
}
