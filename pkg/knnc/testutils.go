package knnc

import "github.com/crunchypi/ddrop/pkg/mathx"

type tVec = mathx.SafeVec

func newTVec(elements ...float64) *tVec {
	return mathx.NewSafeVec(elements...)
}

func newTVecRand(dim int) *tVec {
	v, _ := mathx.NewSafeVecRand(dim)
	return v
}

var _ DistancerContainer = new(data) // Hint.
type data struct {
	v *tVec
}

func (d *data) Distancer() mathx.Distancer {
	return d.v
}

func (d *data) ID() string { return "" } // Not used for tests here.
