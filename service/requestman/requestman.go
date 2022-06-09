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

	// monitor keeps metadata about processed KNN requests, such as average
	// accuracy, latency, satisfaction, etc.
	monitor *knnMonitor
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
	NewLatencyTrackerArgs timex.EventTrackerConfig

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

	// NewKNNMonitorArgs keeps instructions for how to make a new monitor.
	// This includes same args as timex.NewLatencyArgs, as the internal
	// data structure works the same way.
	NewKNNMonitorArgs timex.EventTrackerConfig
}

// Ok returns true if the configuration kn NewHandleArgs is acceptable.
// Specifically:
// - NewHandleArgs.NewSearchSpaceArgs.Ok() == true
// - NewHandleArgs.NewLatencyTrackerArgs.Ok() == true
// - NewHandleArgs.KNNQueueBuf >= 0
// - NewHandleArgs.KNNQueueMaxConcurrent > 0
// - NewHandleArgs.Ctx != nil
// - NewKNNMonitorArgs.Ok == true
func (args *NewHandleArgs) Ok() bool {
	ok := true
	ok = ok && args.NewSearchSpaceArgs.Ok()
	ok = ok && args.NewLatencyTrackerArgs.Ok()
	ok = ok && args.KNNQueueBuf >= 0
	ok = ok && args.KNNQueueMaxConcurrent > 0
	ok = ok && args.Ctx != nil
	ok = ok && args.NewKNNMonitorArgs.Ok()
	return ok
}

// NewHandleArgs attempts to set up a new Handle. Returns (nil, false) if args.Ok
// returns false. For more details, see doc for T Handle and T NewHandleArgs.
func NewHandle(args NewHandleArgs) (*Handle, bool) {
	if !args.Ok() {
		return nil, false
	}

	lt, _ := timex.NewLatencyTracker(args.NewLatencyTrackerArgs)
	et, _ := timex.NewEventTracker[KNNMonItemAvg](args.NewKNNMonitorArgs)
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
		ctx:     args.Ctx,
		monitor: &knnMonitor{et: *et},
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
// Returns false on either of the following conditions:
// - ctx used when creating the Handle (NewHandle(...)) signalled done.
// - DistancerContainer.D == nil.
// - the knnc.SearchSpaces instance used for this namespace returns false
//   on the method AddSearchable(d).
//
// TODO: currently, only the Distancer is stored, as any other means
// of persisting data is not yet implemented.
func (h *Handle) AddData(ns string, d DistancerContainer, data []byte) bool {
	// Check if handle is shut down.
	select {
	case <-h.ctx.Done():
		return false
	default:
	}

	return h.knnNamespaces.put(ns, d)
}

// KNN attempts to enqueue a KNN request, see docs for KNNEnqueueResult for more
// details. Returns a false bool on the following conditions:
// - args.Ok() == false
// - ctx used when creating the Handle (NewHandle(...)) signalled done.
// - args.Namespace is unknown / not yet created with Handle.AddData(...).
// - args.TTL is lower than the estimated queue+query time.
func (h *Handle) KNN(args KNNArgs) (KNNEnqueueResult, bool) {
	if !args.Ok() {
		return KNNEnqueueResult{}, false
	}

	// Check if handle is shut down.
	select {
	case <-h.ctx.Done():
		return KNNEnqueueResult{}, false
	default:
	}

	// Namespace check.
	nsItem, ok := h.knnNamespaces.get(args.Namespace)
	if !ok {
		return KNNEnqueueResult{}, false
	}

	// Latency check.
	d := nsItem.latency.Cfg().MinStep * time.Duration(nsItem.latency.Cfg().MaxN) / 2
	avgQueueWait, _ := h.knnQueue.latency.Average(d)
	avgQueryWait, _ := nsItem.latency.Average(d)
	if avgQueueWait+avgQueryWait > args.TTL {
		return KNNEnqueueResult{}, false
	}

	request := newKNNRequest(&args)
	h.knnQueue.queue <- knnQueueItem{nsItem: nsItem, request: request}
	// Optional listen to result.
	if args.Monitor {
		enqueueResult := h.monitor.register(knnMonitorRegisterArgs{
			knnEnqueueResult: request.enqueueResult,
			k:                args.K,
			ttl:              args.TTL,
		})
		return enqueueResult, true
	}
	return request.enqueueResult, true
}

/*
--------------------------------------------------------------------------------
Below are info/metadata methods on top of T Handle, namespaced with T info.
--------------------------------------------------------------------------------
*/

// info is a namespacing for info methods related to T Handle.
// Do note that single-use is encouraged (but not enforced) in order to prevent
// copies of the nested *Handle, which can lead to leaks.
type info struct {
	h *Handle
}

// Info is used as namespacing for methods related to metadata / general info.
// Single-use is encouraged to prevent leaks, as the returned "info" instance
// contains a ptr to the Handle instance. So call h.Info().Xyz() directly,
// instead of something like "x := h.Info()".
func (h *Handle) Info() *info {
	return &info{h}
}

// SSpaceNamespaces returns all search space namespaces.
func (i *info) SSpaceNamespaces() []string {
	return i.h.knnNamespaces.keys()
}

// SSPaceNamespace checks if a particular search space namespace exists.
func (i *info) SSpaceNamespace(key string) bool {
	return i.h.knnNamespaces.key(key)
}

// SSpaceDim forwards the call to- and return from knnc.SearchSpaces.Dim for a
// search space associated with a namespace. Returns false if the namespace
// does not exist.
func (i *info) SSpaceDim(key string) (int, bool) {
	ssItem, ok := i.h.knnNamespaces.get(key)
	if !ok {
		return 0, false
	}

	return ssItem.searchSpaces.Dim(), true
}

// SSpaceLen forwards the call to- and return from knnc.SearchSpaces.Len for a
// search space associated with a namespace. Returns false if the namespace
// does not exist.
func (i *info) SSpaceLen(key string) (int, int, bool) {
	ssItem, ok := i.h.knnNamespaces.get(key)
	if !ok {
		return 0, 0, false
	}

	nSearchSpaces, nData := ssItem.searchSpaces.Len()
	return nSearchSpaces, nData, true
}

// SSpaceCap forwards the call to- and return from knnc.SearchSpaces.Cap for a
// search space associated with a namespace. Returns false if the namespace
// does not exist.
func (i *info) SSpaceCap(key string) (int, bool) {
	ssItem, ok := i.h.knnNamespaces.get(key)
	if !ok {
		return 0, false
	}

	return ssItem.searchSpaces.Cap(), true
}

// KNNQueueLatency forwards the call to- and return from the "Average" method
// of the timex.LatencyTracker instance associated with the KNN queue.
// In other words, it returns the average KNN queue latency for a given period.
func (i *info) KNNQueueLatency(d time.Duration) (time.Duration, bool) {
	return i.h.knnQueue.latency.Average(d)
}

// KNNQueryLatency forwards the call to- and return from the "Average" method
// of the timex.LatencyTracker instance associated with a searchspace namespace.
// In other words, it returns the average KNN query latency for a given period
// for a particular search space.
func (i *info) KNNQueryLatency(k string, d time.Duration) (time.Duration, bool) {
	ns, ok := i.h.knnNamespaces.get(k)
	if !ok {
		return 0, false
	}
	return ns.latency.Average(d)
}

// KNNMonitor returns the average registered knn monitor data for the time span
// ranging between time.Now() and time.Now().Sub(period).
func (i *info) KNNMonitor(period time.Duration) KNNMonItemAvg {
	return i.h.monitor.average(period)
}
