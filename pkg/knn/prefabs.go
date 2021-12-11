package knn

import "github.com/crunchypi/ddrop/pkg/mathx"

/*
This file contains prefabs / convenience wrappers for core KNN funcs defined in knn.go
*/

// VecPoolGenerator is used for clarity. It represents a generator function
// which is supposed iterate over- and return vectors ([]float) one by one.
type VecPoolGenerator func() ([]float64, bool)

// KNNBruteFloatsArgs are used as args for the KNNBrute func in this pkg. Run
// the 'Ok' method of this type to checks that it's valid.
type KNNBruteFloatsArgs struct {
	// Query is the search vector in the knn routine.
	Query []float64
	// VecPoolGenerator is a func that generates vectors which will be compared
	// to the Query (this struct) in the knn routine. Note that the
	// generated vectors might have to be of equal len as the Query,
	// though this depends on how the DistanceFunc (this struct) handles that.
	VecPoolGenerator VecPoolGenerator
	// DistanceFunc compares the Query (this struct) against vecs yielded
	// from VecPoolGenerator (also this struct) and finds their distance.
	// Examples are Euclidean distance, or cosine similarity, which can be found
	// in the mathx pkg at the time of writing (211110). Do note that a false
	// return will stop the KNN routine abruptly, even if the next item in
	// VecPoolGenerator might be valid.
	DistanceFunc func(v1, v2 []float64) (float64, bool)
	// K stands for the k in k-nearest-neighbours.
	K int
	// Ascending specifies the result order in the func which this struct is
	// used as an argument for. This is intended for two purposes: (A) for the
	// use of different distance functions, as specified with DistanceFunc (this
	// struct). For instance, higher is better with cosine similarity, while the
	// inverse is true for Euclidean distance. The other purpose (B) is to get
	// k-furthest-neighbours instead of nearest.
	Ascending bool
}

// Ok checks that there are no nils and k > 0.
func (args *KNNBruteFloatsArgs) Ok() bool {
	return boolsOk([]bool{
		args.Query != nil,
		args.VecPoolGenerator != nil,
		args.DistanceFunc != nil,
		args.K > 0,
	})
}

// KNNBruteFloats is a general-purpose _lazy_  k-nearest-neighbours function
// which wraps KNNBrute of this pkg, and uses []float64 as input. For details
// about the argument, see comments for/in KNNBruteFloatsArgs (type in this pkg).
// Returns false if the args.Ok() check fails. The []int return will represent
// index pointers to the 'nearest' items in args.VecPoolGenerator.
func KNNBruteFloats(args KNNBruteFloatsArgs) ([]int, bool) {
	if !args.Ok() {
		return nil, false
	}

	return KNNBrute(KNNBruteArgs{
		ScoreGenerator: func() (float64, bool) {
			v, cont := args.VecPoolGenerator()
			if !cont {
				return 0, cont
			}

			return args.DistanceFunc(args.Query, v)
		},
		K:         args.K,
		Ascending: args.Ascending,
	})
}

// KNNEucFloats finds k nearest neighbours using Euclidean distance and []float64.
// It is a convenience wrapper around KNNBruteFloats (this pkg).
func KNNEucFloats(query []float64, pool VecPoolGenerator, k int) ([]int, bool) {
	return KNNBruteFloats(KNNBruteFloatsArgs{
		Query:            query,
		VecPoolGenerator: pool,
		DistanceFunc:     mathx.EuclideanDistance,
		K:                k,
		Ascending:        true,
	})
}

// KFNEucFloats finds k furthest neighbours using Euclidean distance and []float64.
// It is a convenience wrapper around KNNBruteFloats (this pkg).
func KFNEucFloats(query []float64, pool VecPoolGenerator, k int) ([]int, bool) {
	return KNNBruteFloats(KNNBruteFloatsArgs{
		Query:            query,
		VecPoolGenerator: pool,
		DistanceFunc:     mathx.EuclideanDistance,
		K:                k,
		Ascending:        false,
	})
}

// KNNCosFloats finds k nearest neighbours using cosine similarity and []float64.
// It is a convenience wrapper around KNNBruteFloats (this pkg).
func KNNCosFloats(query []float64, pool VecPoolGenerator, k int) ([]int, bool) {
	return KNNBruteFloats(KNNBruteFloatsArgs{
		Query:            query,
		VecPoolGenerator: pool,
		DistanceFunc:     mathx.CosineSimilarity,
		K:                k,
		Ascending:        false,
	})
}

// KFNCosFloats finds k furthest neighbours using cosine similarity and []float64.
// It is a convenience wrapper around KNNBrute (this pkg).
func KFNCosFloats(query []float64, pool VecPoolGenerator, k int) ([]int, bool) {
	return KNNBruteFloats(KNNBruteFloatsArgs{
		Query:            query,
		VecPoolGenerator: pool,
		DistanceFunc:     mathx.CosineSimilarity,
		K:                k,
		Ascending:        true,
	})
}

// DistancePoolGenerator is used for clarity. It represents a generator function
// which is supposed iterate over- and return mathx.Distancer one by one.
type DistancerPoolGenerator func() (mathx.Distancer, bool)

// KNNBruteDistArgs are used as args for the KNNBruteDist func in this pkg.
// Run the 'Ok' method method of this type to check that it's valid.
type KNNBruteDistArgs struct {
	// Query is the query in the knn routine.
	Query mathx.Distancer
	// DistancerPoolGenerator is a func that generates mathx.Distancer instances;
	// these will be compared to the Query (this struct) in the knn routine.
	DistancerPoolGenerator DistancerPoolGenerator
	// DistanceFunc compared the Query (this struct) against distancers yielded
	// from DistancerPoolGenerator and finds their distance. The distance calc
	// support will naturally depend on the mathx.Distancer interface. Note that
	// a false return will stop the KNN routine abruptly, even if the next item in
	// VecPoolGenerator might be valid.
	DistanceFunc func(d1, d2 mathx.Distancer) (float64, bool)
	// K is the k in knn.
	K int
	// Ascending specifies the result order in the func which this struct is
	// used as an argument for. This is intended for two purposes: (A) for the
	// use of different distance functions, as specified with DistanceFunc (this
	// struct). For instance, higher is better with cosine similarity, while the
	// inverse is true for Euclidean distance. The other purpose (B) is to get
	// k-furthest-neighbours instead of nearest.
	Ascending bool
}

// Ok checks that there are no nils and k > 0.
func (args *KNNBruteDistArgs) Ok() bool {
	return boolsOk([]bool{
		args.Query != nil,
		args.DistancerPoolGenerator != nil,
		args.DistanceFunc != nil,
		args.K > 0,
	})
}

// KNNBruteDist is _lazy_ k-nearest-neighbours function which wraps KNNBrute of
// this pkg, and uses mathx.Distancer(s) as input. For details about the argument,
// see the comments for/in KNNBruteDistArgs. The []int return will represent
// index pointers to the 'nearest' items in args.DistancerPoolGenerator.
func KNNBruteDist(args KNNBruteDistArgs) ([]int, bool) {
	if !args.Ok() {
		return nil, false
	}

	return KNNBrute(KNNBruteArgs{
		ScoreGenerator: func() (float64, bool) {
			d, cont := args.DistancerPoolGenerator()
			if !cont {
				return 0, cont
			}
			return args.DistanceFunc(args.Query, d)
		},
		K:         args.K,
		Ascending: args.Ascending,
	})
}

// KNNEucDist finds k nearest neighbours using Euclidean distance and mathx.Distancer.
// It is a convenience wrapper around KNNBruteDist (this pkg).
func KNNEucDist(query mathx.Distancer, pool DistancerPoolGenerator, k int) ([]int, bool) {
	return KNNBruteDist(KNNBruteDistArgs{
		Query:                  query,
		DistancerPoolGenerator: pool,
		DistanceFunc: func(d1, d2 mathx.Distancer) (float64, bool) {
			if d1 == nil || d2 == nil {
				return 0, false
			}
			return d1.EuclideanDistance(d2)
		},
		K:         k,
		Ascending: true,
	})
}

// KFNEucDist finds k furthest neighbours using Euclidean distance and mathx.Distancer.
// It is a convenience wrapper around KNNBruteDist (this pkg).
func KFNEucDist(query mathx.Distancer, pool DistancerPoolGenerator, k int) ([]int, bool) {
	return KNNBruteDist(KNNBruteDistArgs{
		Query:                  query,
		DistancerPoolGenerator: pool,
		DistanceFunc: func(d1, d2 mathx.Distancer) (float64, bool) {
			if d1 == nil || d2 == nil {
				return 0, false
			}
			return d1.EuclideanDistance(d2)
		},
		K:         k,
		Ascending: false,
	})
}

// KNNCosDist finds k nearest neighbours using cosine similarity and mathx.Distancer.
// It is a convenience wrapper around KNNBruteDist (this pkg).
func KNNCosDist(query mathx.Distancer, pool DistancerPoolGenerator, k int) ([]int, bool) {
	return KNNBruteDist(KNNBruteDistArgs{
		Query:                  query,
		DistancerPoolGenerator: pool,
		DistanceFunc: func(d1, d2 mathx.Distancer) (float64, bool) {
			if d1 == nil || d2 == nil {
				return 0, false
			}
			return d1.CosineSimilarity(d2)
		},
		K:         k,
		Ascending: false,
	})
}

// KFNCosDist finds k furthest neighbours using cosine similarity and mathx.Distancer.
// It is a convenience wrapper around KNNBruteDist (this pkg).
func KFNCosDist(query mathx.Distancer, pool DistancerPoolGenerator, k int) ([]int, bool) {
	return KNNBruteDist(KNNBruteDistArgs{
		Query:                  query,
		DistancerPoolGenerator: pool,
		DistanceFunc: func(d1, d2 mathx.Distancer) (float64, bool) {
			if d1 == nil || d2 == nil {
				return 0, false
			}
			return d1.CosineSimilarity(d2)
		},
		K:         k,
		Ascending: true,
	})
}
