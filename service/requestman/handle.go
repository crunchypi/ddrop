package requestman

import (
	"time"

	"github.com/crunchypi/ddrop/pkg/knnc"
	"github.com/crunchypi/ddrop/pkg/mathx"
)

// DistancerContainer implements knnc.DistancerContainer.
type DistancerContainer struct {
	D mathx.Distancer
	// TODO: Check performance. As of now, each call to Distancer() method does
	// a time.Now() call; the alternative is to have a bool in addition, as that
	// is cheaper. But that would also require a sync.RWMutes due to how this
	// will be used concurrently in the knnc pkg.
	Expires time.Time
}

// Distancer returns the internal mathx.Distancer if the Expiration field is set
// and after time.Now().
func (d *DistancerContainer) Distancer() mathx.Distancer {
	if d.Expires != (time.Time{}) && time.Now().After(d.Expires) {
		return nil
	}
	return d.D
}

// Symbolic.
var _ knnc.DistancerContainer = &DistancerContainer{}
