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

// Internal type for tracking searched elements that are 'best'.
type resultItem struct {
	// Index from VecPoolGenerator, acts as a pointer.
	index int
	// Derived from some distance function.
	score float64
	// Used to check if it's uninitialized.
	set bool
}

// Convenience with attached methods.
type resultItems []resultItem

// bubbleInsert ether bubbles up- or bubbles down the insertee into the slice,
// based on the 'ascending' arg and the 'score' within _all_ 'resultItems',
// including the ones in the slice this method is attached to. It assumes that
// all elements in the slice are already sorted in the way that is specified by
// the ascending arg, otherwise it won't work as expected, so be sure to insert
// any resultItem into the slice with this method.
func (items resultItems) bubbleInsert(insertee resultItem, ascending bool) {
	for i := 0; i < len(items); i++ {
		// Either the called tried to insert an item that is not set,
		// or 'i' > 0 and a swap happened which replaced an unset item.
		// In any case, insertee does not belong anywhere anymore.
		if !insertee.set {
			return
		}

		condA := !items[i].set
		condB := insertee.score < items[i].score && ascending
		condC := insertee.score > items[i].score && !ascending
		if condA || condB || condC {
			insertee, items[i] = items[i], insertee
		}
	}
}

// toIndexes converts extracts the index field from each element in resultItem.
func (items resultItems) toIndexes() []int {
	r := make([]int, 0, len(items))
	for i := 0; i < len(items); i++ {
		if !items[i].set {
			continue
		}
		r = append(r, items[i].index)
	}

	return r
}

// KNNBruteArgs are used as args for the KNNBrute func in this pkg Run the 'ok'.
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
func (k KNNBruteArgs) Ok() bool {
	checks := []bool{
		k.SearchVec != nil,
		k.VecPoolGenerator != nil,
		k.DistanceFunc != nil,
		k.K > 0,
	}

	for _, check := range checks {
		if !check {
			return false
		}
	}

	return true
}

// KNNBrute is a general-purpose k-nearest-neighbours function. For details
// about the argument, see comments for KNNBruteArgs (type in this pkg).
// Returns false if the args.OK() check fails.
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

// KNNEuc finds k nearest neighbours using Euclidean distance.
// It is a convenience wrapper around KNNBrute (this pkg).
func KNNEuc(searchVec []float64, pool VecPoolGenerator, k int) ([]int, bool) {
	return KNNBrute(KNNBruteArgs{
		SearchVec:        searchVec,
		VecPoolGenerator: pool,
		DistanceFunc:     mathx.EuclideanDistance,
		K:                k,
		Ascending:        true,
	})
}

// KNNEuc finds k furthest neighbours using Euclidean distance.
// It is a convenience wrapper around KNNBrute (this pkg).
func KFNEuc(searchVec []float64, pool VecPoolGenerator, k int) ([]int, bool) {
	return KNNBrute(KNNBruteArgs{
		SearchVec:        searchVec,
		VecPoolGenerator: pool,
		DistanceFunc:     mathx.EuclideanDistance,
		K:                k,
		Ascending:        false,
	})
}

// KNNEuc finds k nearest neighbours using cosine similarity.
// It is a convenience wrapper around KNNBrute (this pkg).
func KNNCos(searchVec []float64, pool VecPoolGenerator, k int) ([]int, bool) {
	return KNNBrute(KNNBruteArgs{
		SearchVec:        searchVec,
		VecPoolGenerator: pool,
		DistanceFunc:     mathx.CosineSimilarity,
		K:                k,
		Ascending:        true,
	})
}

// KNNEuc finds k furthest neighbours using cosine similarity.
// It is a convenience wrapper around KNNBrute (this pkg).
func KFNCos(searchVec []float64, pool VecPoolGenerator, k int) ([]int, bool) {
	return KNNBrute(KNNBruteArgs{
		SearchVec:        searchVec,
		VecPoolGenerator: pool,
		DistanceFunc:     mathx.CosineSimilarity,
		K:                k,
		Ascending:        false,
	})
}
