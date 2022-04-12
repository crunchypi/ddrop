package requestman

import (
	"context"

	"github.com/crunchypi/ddrop/pkg/knnc"
	"github.com/crunchypi/ddrop/pkg/timex"
)

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
			v.searchSpaces.StopMaintenance()
		}
	}
}
