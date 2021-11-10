/*
This pkg contains a general-purpose k-nearest-neighbours implementation.
*/
package knn

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
		r = append(r, items[i].index)
	}

	return r
}
