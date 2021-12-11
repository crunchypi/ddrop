/*
This pkg contains a general-purpose k-nearest-neighbours implementation.
It is based on the use of generators, which makes it especially flexible.
*/
package knn

import (
	"math"
)

// KNNBruteArgs are used as args for the KNNBrute func of this pkg. Run the
// 'Ok' method of this type to check that it's valid.
type KNNBruteArgs struct {
	// ScoreGenerator is intended to be a generator that yields scores and
	// bools indicating continue(true)/stop(false). These scores are intended
	// to be created from some distance calculation, using two vectors.
	ScoreGenerator func() (score float64, cont bool)
	// K stands for the k in k-nearest-neighbours.
	K int
	// Ascending specifies the ordering of scores generated with ScoreGenerator.
	// For instance, if the distance func used in ScoreGenerator is cosine
	// similarity, then higher is better and the scores should be ordered in
	// a descending (e.g 1, 0.5, 0.1) way for k-nearest-neighbours, so this
	// field var must be false. For k-furthest-neighbours, it should be true,
	// since the order is reversed. For Euclidean distance, the rules are
	// reversed since lower is better.
	Ascending bool
}

// Ok checks if values in the struct are ok, specifically that ScoreGenerator
// is not nil and : is > 0.
func (args *KNNBruteArgs) Ok() bool {
	return boolsOk([]bool{
		args.ScoreGenerator != nil,
		args.K > 0,
	})
}

// KNNBrute is a general-purpose _lazy_ k-nearest-neighbours func, where rules
// of distance calculation is handled by the user -- see KNNBruteArgs for argument
// details. Returns false if args.Ok() == false. The []int return will represent
// index pointers to the 'best' scores yielded from args.ScoreGenerator. Result
// will be cut short if the generator func signals stop.
func KNNBrute(args KNNBruteArgs) ([]int, bool) {
	if !args.Ok() {
		return nil, false
	}

	r := make(resultItems, args.K)

	// Apply 'worst' score to all resultItems
	similarity := math.MaxFloat64
	if !args.Ascending {
		similarity *= -1
	}
	for i := 0; i < args.K; i++ {
		r[i].score = similarity
	}

	// Unusual loop since the bounds of args.scoreGenerator is unknown.
	i := 0
	for {
		// Next vector.
		score, cont := args.ScoreGenerator()
		if !cont {
			break
		}

		// Eval include.
		r.bubbleInsert(resultItem{i, score, true}, args.Ascending)
		i++
	}

	return r.toIndexes(), true
}
