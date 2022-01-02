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

// boolsOk returns true if all bools in the slice are true.
func boolsOk(bs []bool) bool {
	for _, b := range bs {
		if !b {
			return false
		}
	}
	return true
}

type ScoreItem struct {
	ID    string
	Score float64
	set   bool
}

// ScoreItems is <[]ScoreItem>, used for method attachment.
type ScoreItems []ScoreItem

// BubbleInsert either bubbles up- or bubbles down the insertee into the slice,
// based on the 'ascending' arg and the 'score' within _all_ 'ScoreItems',
// including the ones in the slice this method is attached to. It assumes that
// all elements in the slice are already sorted in the way that is specified by
// the ascending arg, otherwise it won't work as expected, so be sure to insert
// any ScoreItem into the slice with this method.
func (items ScoreItems) BubbleInsert(insertee ScoreItem, ascending bool) {
	for i := 0; i < len(items); i++ {
		// Either the caller tried to insert an item that is not set,
		// or 'i' > 0 and a swap happened which replaced an unset item.
		// In any case, insertee does not belong anywhere anymore.
		if !insertee.set {
			return
		}

		condA := !items[i].set
		condB := insertee.Score < items[i].Score && ascending
		condC := insertee.Score > items[i].Score && !ascending
		if condA || condB || condC {
			insertee, items[i] = items[i], insertee
		}
	}
}

// Trim removes zero-value elements from the slice.
func (items ScoreItems) Trim() ScoreItems {
	r := make(ScoreItems, 0, len(items))
	for _, item := range items {
		if !item.set {
			continue
		}
		r = append(r, item)
	}

	return r
}
