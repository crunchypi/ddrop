package requestman

import (
	"sync"

	"github.com/crunchypi/ddrop/pkg/knnc"
	"github.com/crunchypi/ddrop/pkg/timex"
)

/*
File contains a hashmap wrapper for knnc.SearchSpaces, as a way of namespacing.
*/

// knnNamespacesItem is intended to be used in namedSSPace.items (as values).
// Keeps a knnc.SearchSpaces instance and a timex.LatencyTracker to track
// how long KNN lookups take.
type knnNamespacesItem struct {
	latency      *timex.LatencyTracker
	searchSpaces *knnc.SearchSpaces
}

// knnNamespaces is a namespacing mutex-protected wrapper around knnc.SearchSpaces.
// See more info at T namedSSPaceItem.
type knnNamespaces struct {
	sync.RWMutex
	items map[string]knnNamespacesItem

	// newSearchSpaceArgs keeps instructions for how to create new search spaces
	// that go into new namedSSPaceItem (for knnNamespaces.items).
	newSearchSpaceArgs knnc.NewSearchSpacesArgs
	// newLatencyTrackerArgs keeps instructions for how to create new latency
	// trackers that go into new namedSSPaceItems (for knnNamespaces.items)
	newLatencyTrackerArgs timex.NewLatencyTrackerArgs
}

// key returns true if a key/namespace exists.
func (ns *knnNamespaces) key(key string) bool {
	ns.RLock()
	defer ns.RUnlock()

	_, ok := ns.items[key]
	return ok
}

// keys retrieves all keys/namespaces.
func (ns *knnNamespaces) keys() []string {
	ns.RLock()
	defer ns.RUnlock()

	keys := make([]string, 0, len(ns.items))
	for k := range ns.items {
		keys = append(keys, k)
	}
	return keys
}

// get retrieves a knnNamespaceItem using a key/namespace. Returns false if the
// namespace does not exist.
func (ns *knnNamespaces) get(key string) (knnNamespacesItem, bool) {
	ns.Lock()
	defer ns.Unlock()
	// Roundabout since 'return map[key]' gives 1 item...
	ss, ok := ns.items[key]
	return ss, ok
}

// put adds a DistancerContainer to a namespace. If the namespace does not exist
// then a new one will be automatically created. Returns false if
// - DistancerContainer.D == nil.
// - An attempt to create a new namespace failed. This happens if a new
//   knnc.NewSearchSpaces(knnNamespaces.newSearchSpaceArgs) returns false.
// - knnc.SearchSpaces.AddSearchable(DistancerContainer) returns false.
func (ns *knnNamespaces) put(key string, d DistancerContainer) bool {
	if d.D == nil {
		return false
	}

	ns.Lock()
	defer ns.Unlock()

	nsItem, ok := ns.items[key]
	if !ok {
		newSearchSpaces, ok := knnc.NewSearchSpaces(ns.newSearchSpaceArgs)
		if !ok {
			return false
		}
		newSearchSpaces.StartMaintenance()

		lt, _ := timex.NewLatencyTracker(ns.newLatencyTrackerArgs)
		nsItem.latency = lt
		nsItem.searchSpaces = newSearchSpaces
		ns.items[key] = nsItem
	}

	return nsItem.searchSpaces.AddSearchable(&d)
}

// del deletes all namespaces with the specified keys. If no keys are used, then
// everything is deleted -- same as calling ns.del(ns.keys()...).
func (ns *knnNamespaces) del(keys ...string) {
	if len(keys) == 0 {
		keys = ns.keys()
	}
	for _, k := range keys {
		delete(ns.items, k)
	}
}
