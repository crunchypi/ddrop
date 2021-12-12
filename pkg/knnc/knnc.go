/*
knnc is a package for doing KNN (k nearest neighbour) searching with high
concurrency.

*/
package knnc

import "github.com/crunchypi/ddrop/pkg/mathx"

// Distancer is an alias for mathx.Distancer.
type Distancer = mathx.Distancer

// DistancerContainer is any type that can provide a mathx.Distancer, which can
// do distance calculation (related to KNN), and some ID that can reference it.
// The intent of this pkg is to enable KNN queries, using the mathx.Distancer,
// and eventually giving some result containing this ID, which could be used for
// some lookup elsewhere.
type DistancerContainer interface {
	// See docs for mathx.Distancer and/or the surrounding interface (
	// DistancerContainer). The concrete returned type here should be
	// thread-safe, or nil when it is no longer needed -- this will mark
	// it as deletable.
	Distancer() mathx.Distancer
	// See docs for the surroinding interface (DistancerContainer). This
	// will be part of KNN queries and should reference the Distancer given
	// by the Distancer() func of this interface.
	ID() string
}
