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
