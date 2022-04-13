package requestman

import (
	"context"
	"time"

	"github.com/crunchypi/ddrop/pkg/knnc"
	"github.com/crunchypi/ddrop/pkg/mathx"
	"github.com/crunchypi/ddrop/pkg/timex"
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

// Handle is the main way of interacting with this pkg. It handles data storage,
// KNN requests, info retrieval, etc.
type Handle struct {
	// knnNamespaces contains namespaced KNN search spaces.
	knnNamespaces *knnNamespaces
	// knnQueue processes KNN requests.
	knnQueue knnQueue

	// ctx is used to stop the KNN request queue. It will also be used to stop
	// the maintanence loop for each namespaced (KNN) search space (for more
	// info about this, see docs for T SearchSpaces of pkg/knnc).
	ctx context.Context
}

// NewHandleArgs is intended as args for func NewHandle.
type NewHandleArgs struct {
	// NewSearchSpaceArgs keeps instructions for how to create new search spaces.
	// This is done 'occasionally' on Handle.AddData(...). For more details, see
	// documentation for the knnc pkg (particularly for T SearchSpaces).
	NewSearchSpaceArgs knnc.NewSearchSpacesArgs
	// NewLatencyTrackerArgs keeps instructions for how to create new latency
	// trackers. This is used for the KNN request queue in T Handle, as well as
	// for each new namespaced (KNN) search space.
	NewLatencyTrackerArgs timex.NewLatencyTrackerArgs

	// KNNQueueBuf specifies the buffer for the KNN request queue in T Handle.
	KNNQueueBuf int
	// KNNQueueMaxConcurrent specifies the max number of _parent_ goroutines
	// available for the KNN request queue in T Handle. In other words, it
	// specifies how many KNN requests can be processed concurrently -- though
	// each KNN request can use multiple goroutines individually.
	KNNQueueMaxConcurrent int

	// Ctx is used to stop the KNN request queue. It will also be used to stop
	// the maintanence loop for each namespaced (KNN) search space (for more
	// info about this, see docs for T SearchSpaces of pkg/knnc).
	Ctx context.Context
}

// Ok returns true if the configuration kn NewHandleArgs is acceptable.
// Specifically:
// - NewHandleArgs.NewSearchSpaceArgs.Ok() == true
// - NewHandleArgs.NewLatencyTrackerArgs.Ok() == true
// - NewHandleArgs.KNNQueueBuf >= 0
// - NewHandleArgs.KNNQueueMaxConcurrent > 0
// - NewHandleArgs.Ctx != nil
func (args *NewHandleArgs) Ok() bool {
	ok := true
	ok = ok && args.NewSearchSpaceArgs.Ok()
	ok = ok && args.NewLatencyTrackerArgs.Ok()
	ok = ok && args.KNNQueueBuf >= 0
	ok = ok && args.KNNQueueMaxConcurrent > 0
	ok = ok && args.Ctx != nil
	return ok
}

// NewHandleArgs attempts to set up a new Handle. Returns (nil, false) if args.Ok
// returns false. For more details, see doc for T Handle and T NewHandleArgs.
func NewHandle(args NewHandleArgs) (*Handle, bool) {
	if !args.Ok() {
		return nil, false
	}

	lt, _ := timex.NewLatencyTracker(args.NewLatencyTrackerArgs)
	h := Handle{
		knnNamespaces: &knnNamespaces{
			items:                 make(map[string]knnNamespacesItem),
			newSearchSpaceArgs:    args.NewSearchSpaceArgs,
			newLatencyTrackerArgs: args.NewLatencyTrackerArgs,
		},
		knnQueue: knnQueue{
			latency:       lt,
			queue:         make(chan knnQueueItem, args.KNNQueueBuf),
			maxConcurrent: args.KNNQueueMaxConcurrent,
			ctx:           args.Ctx,
		},
		ctx: args.Ctx,
	}

	go h.knnQueue.startProcessing()
	go h.waitThenQuit()
	return &h, true
}

// waitThenQuit waits for Handle.ctx to be done, then stops the maintenance of
// all namespaced KNN search spaces. This method will block.
func (h *Handle) waitThenQuit() {
	select {
	case <-h.ctx.Done():
		for _, v := range h.knnNamespaces.items {
			if v.searchSpaces == nil {
				continue
			}

			v.searchSpaces.StopMaintenance()
		}
	}
}

// AddData adds data to a namespace, using a DistancerContainer(.Distancer()) as
// an index. A new namespace will be created if one does not already exist.
//
// TODO: currently, only the Distancer is stored, as any other means
// of persisting data is not yet implemented.
func (h *Handle) AddData(ns string, d DistancerContainer, data []byte) bool {
	return h.knnNamespaces.put(ns, d)
}

// KNN attempts to enqueue a KNN request, see docs for KNNEnqueueResult for more
// details. Returns a false bool on the following conditions:
// - args.Ok() == false
// - args.Namespace is unknown / not yet created with Handle.AddData(...).
// - args.TTL is lower than the estimated queue+query time.
func (h *Handle) KNN(args KNNArgs) (KNNEnqueueResult, bool) {
	if !args.Ok() {
		return KNNEnqueueResult{}, false
	}

	// Namespace check.
	nsItem, ok := h.knnNamespaces.get(args.Namespace)
	if !ok {
		return KNNEnqueueResult{}, false
	}

	// Latency check.
	avgQueueWait, _ := h.knnQueue.latency.AverageSTD()
	avgQueryWait, _ := nsItem.latency.AverageSTD()
	if avgQueueWait+avgQueryWait > args.TTL {
		return KNNEnqueueResult{}, false
	}

	request := newKNNRequest(&args)
	h.knnQueue.queue <- knnQueueItem{nsItem: nsItem, request: request}
	return request.enqueueResult, true
}
