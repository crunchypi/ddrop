package mathx

import (
	"math"
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Distancer is an interface for types that can caluclate some select
// distance functions such as Euclidean or Cosine. This is meant to
// be used with some underlying vector such as []float64 or variants.
type Distancer interface {
	// Euclidean computes the Euclidean distance to another Distancer.
	Euclidean(other Distancer) (float64, bool)

	// Peek attempts to return an element of an underlying vector at
	// the given index. False return signals out-of-bounds.
	Peek(index uint) (float64, bool)
	// Dim is intended to return the dimension of an underlying vector.
	Dim() uint
}

// SafeVec is a read-only wrapper around an []float64; the intent is
// for it to be safe to pass around in a highly concurrent context.
// Note 1; it implements the 'Distancer' interface in this pkg.
// Note 2; no locking as it is read-only.
type SafeVec struct {
	vec []float64
}

// NewSafeVec is a constructor for SafeVec, which is initialized with
// the given elements.
func NewSafeVec(elements ...float64) *SafeVec {
	vec := make([]float64, len(elements))
	for i, elm := range elements {
		vec[i] = elm
	}

	return &SafeVec{vec: vec}
}

// NewSafeVecRand is a constructor for SafeVec, which is initialized
// with a specified dimention and elements in rand range [0,1].
func NewSafeVecRand(dim uint) *SafeVec {
	vec := make([]float64, dim)
	for i := 0; i < int(dim); i++ {
		vec[i] = rand.Float64()
	}

	return &SafeVec{vec: vec}
}

// Dim exposes the dimension of the underlying vector.
func (v *SafeVec) Dim() uint {
	return uint(len(v.vec))
}

// Clone returns a clone of the type.
func (v *SafeVec) Clone() *SafeVec {
	return NewSafeVec(v.vec...)
}

// Iter allows a safe read-only iteration of the underlying vector.
// Accepts a func which receives the index and value (i.e a range loop)
// of each element -- this func can return false to stop the itaration.
func (v *SafeVec) Iter(f func(uint, float64) bool) {
	for i, elm := range v.vec {
		if !f(uint(i), elm) {
			return
		}
	}
}

// Eq does an equality check with the other SafeVec.
func (v *SafeVec) Eq(other *SafeVec) bool {
	if uint(len(v.vec)) != other.Dim() {
		return false
	}

	eq := true
	other.Iter(func(i uint, elm float64) bool {
		eq = v.vec[i] == elm
		return eq
	})
	return eq
}

// In checks if this SafeVec is contained in a given slice. Equality
// checks are done with SafeVec.Eq(...), so not particularly fast.
func (v *SafeVec) In(others []*SafeVec) bool {
	for i := range others {
		if v.Eq(others[i]) {
			return true
		}
	}
	return false
}

// Peek returns the element of the underlying []float64 at a given index.
// Will return false if the index is out-of-bounds.
func (v *SafeVec) Peek(index uint) (float64, bool) {
	l := uint(len(v.vec))
	if index >= l || index < 0 {
		return 0, false
	}
	return v.vec[index], true
}

// Euclidean computes the Euclidean distance to another vec that implements
// the Distancer interface (this pkg). Returns false if dimensions are neq.
func (v *SafeVec) Euclidean(other Distancer) (float64, bool) {
	if other == nil || uint(len(v.vec)) != other.Dim() {
		return 0, false
	}

	r := 0.
	for i, vi := range v.vec {
		wi, ok := other.Peek(uint(i))
		if !ok {
			panic("ehh")
		}
		r += (vi - wi) * (vi - wi)
		//x := vi - wi
		//r += x * x
	}

	return math.Sqrt(r), true
}
