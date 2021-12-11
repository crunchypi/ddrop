package knn

import "github.com/crunchypi/ddrop/pkg/mathx"

/*
This file contains prefabs / convenience wrappers for core KNN funcs defined in knn.go
*/

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

// KFNEuc finds k furthest neighbours using Euclidean distance.
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

// KNNCos finds k nearest neighbours using cosine similarity.
// It is a convenience wrapper around KNNBrute (this pkg).
func KNNCos(searchVec []float64, pool VecPoolGenerator, k int) ([]int, bool) {
	return KNNBrute(KNNBruteArgs{
		SearchVec:        searchVec,
		VecPoolGenerator: pool,
		DistanceFunc:     mathx.CosineSimilarity,
		K:                k,
		Ascending:        false,
	})
}

// KFNCos finds k furthest neighbours using cosine similarity.
// It is a convenience wrapper around KNNBrute (this pkg).
func KFNCos(searchVec []float64, pool VecPoolGenerator, k int) ([]int, bool) {
	return KNNBrute(KNNBruteArgs{
		SearchVec:        searchVec,
		VecPoolGenerator: pool,
		DistanceFunc:     mathx.CosineSimilarity,
		K:                k,
		Ascending:        true,
	})
}

// KNNEucDist finds k nearest neighbours using Euclidean distance.
// It is a convenience wrapper around KNNBruteDist (this pkg).
func KNNEucDist(query mathx.Distancer, pool []mathx.Distancer, k int) ([]int, bool) {
	return KNNBruteDist(KNNBruteDistArgs{
		Query: query,
		Pool:  pool,
		Mapper: func(d1, d2 mathx.Distancer) (float64, bool) {
			if d1 == nil || d2 == nil {
				return 0, false
			}
			return d1.EuclideanDistance(d2)
		},
		K:         k,
		Ascending: true,
	})
}

// KFNEucDist finds k furthest neighbours using Euclidean distance.
// It is a convenience wrapper around KNNBruteDist (this pkg).
func KFNEucDist(query mathx.Distancer, pool []mathx.Distancer, k int) ([]int, bool) {
	return KNNBruteDist(KNNBruteDistArgs{
		Query: query,
		Pool:  pool,
		Mapper: func(d1, d2 mathx.Distancer) (float64, bool) {
			if d1 == nil || d2 == nil {
				return 0, false
			}
			return d1.EuclideanDistance(d2)
		},
		K:         k,
		Ascending: false,
	})
}

// KNNCosDist finds k nearest neighbours using cosine similarity.
// It is a convenience wrapper around KNNBruteDist (this pkg).
func KNNCosDist(query mathx.Distancer, pool []mathx.Distancer, k int) ([]int, bool) {
	return KNNBruteDist(KNNBruteDistArgs{
		Query: query,
		Pool:  pool,
		Mapper: func(d1, d2 mathx.Distancer) (float64, bool) {
			if d1 == nil || d2 == nil {
				return 0, false
			}
			return d1.CosineSimilarity(d2)
		},
		K:         k,
		Ascending: false,
	})
}

// KFNCosDist finds k furthest neighbours using cosine similarity.
// It is a convenience wrapper around KNNBruteDist (this pkg).
func KFNCosDist(query mathx.Distancer, pool []mathx.Distancer, k int) ([]int, bool) {
	return KNNBruteDist(KNNBruteDistArgs{
		Query: query,
		Pool:  pool,
		Mapper: func(d1, d2 mathx.Distancer) (float64, bool) {
			if d1 == nil || d2 == nil {
				return 0, false
			}
			return d1.CosineSimilarity(d2)
		},
		K:         k,
		Ascending: true,
	})
}
