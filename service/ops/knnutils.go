package ops

import (
	"reflect"

	"github.com/crunchypi/ddrop/pkg/knnc"
	"github.com/crunchypi/ddrop/pkg/mathx"
)

// Distancer2Vec simply converts a mathx.Distancer (collection of float64)
// into a []float64. The reason this is needed is that Go rpc doesn't seem
// to work with interfaces.
func Distancer2Vec(d mathx.Distancer) []float64 {
	l := d.Dim()

	r := make([]float64, l)
	for i := 0; i < l; i++ {
		r[i], _ = d.Peek(i)
	}

	return r
}

// KNNRespItemFromScoreItem converts KNN results (pkg knnc and requestman)
// into a KNNRespItem. See docs for Distancer2Vec for why this is needed.
func KNNRespItemFromScoreItem(scoreItem knnc.ScoreItem) KNNRespItem {
	return KNNRespItem{
		Vec:   Distancer2Vec(scoreItem.Distancer),
		Score: scoreItem.Score,
	}
}

// KNNRespItemsFromScoreItems converts KNN results (pkg knnc and requestman)
// into a KNNResp. See docs for Distancer2Vec for why this is needed.
func KNNRespItemsFromScoreItems(scoreItems knnc.ScoreItems) []KNNRespItem {
	scoreItems = scoreItems.Trim()
	r := make([]KNNRespItem, 0, len(scoreItems))
	for _, scoreItem := range scoreItems {
		ok := true
		ok = ok && scoreItem.Distancer != nil
		ok = ok && !reflect.ValueOf(scoreItem.Distancer).IsNil()
		if !ok {
			continue
		}

		r = append(r, KNNRespItemFromScoreItem(scoreItem))
	}

	return r
}

// sortItem is intended to be used as an item that can be ordered.
// Originally intended for bubbleInsert(...).
type sortItem[T any] struct {
	score float64
	set   bool
	data  T
}

// bubbleInsert simply does a bubble insert operation of the "insertee" into the
// given slice "s", based on sortItem.score. The "ascending" bool specifies if
// it's bubble up or bubble down. This func assumes that all elements in the
// slice are already ordered in the same manner as specifies with "ascending".
// Do note that sortItem.set = false will be ignored.
func bubbleInsert[T any](s []sortItem[T], insertee sortItem[T], ascending bool) {
	for i := 0; i < len(s); i++ {
		// Either the caller tried to insert an item that is not set,
		// or 'i' > 0 and a swap happened which replaced an unset item.
		// In any case, insertee does not belong anywhere anymore.
		if !insertee.set {
			return
		}

		condA := !s[i].set
		condB := insertee.score < s[i].score && ascending
		condC := insertee.score > s[i].score && !ascending
		if condA || condB || condC {
			insertee, s[i] = s[i], insertee
		}
	}
}
