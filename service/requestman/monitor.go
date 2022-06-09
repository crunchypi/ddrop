package requestman

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/crunchypi/ddrop/pkg/knnc"
	"github.com/crunchypi/ddrop/pkg/timex"
)

// knnMonItem captures stats per individual KNN request.
type knnMonItem struct {
	Latency      time.Duration
	AvgScore     float64
	Satisfaction float64
}

// KNNMonItemAvg captures stats for a group of KNN requests over a period.
type KNNMonItemAvg struct {
	isSet   bool
	Created time.Time     // Start of recording.
	Span    time.Duration // Recording duration.

	N               int           // Number of recorded requests (including fails).
	NFailed         int           // Number of (completely) failed requests.
	AvgLatency      time.Duration // Average latency of all requests.
	AvgScore        float64       // Average score for all requests.
	AvgScoreNoFails float64       // Same as AvgScore but without fails.
	AvgSatisfaction float64       // Success ratio (got n / want n).
}

// mergeKNNMonItem merges a knnMonItem in such a way that averages are maintained.
// Note that KNNMonItemAvg.AvgScoreNoFails will have some imprecision and should
// only be used for estimation.
//
// Internals: ia.isSet will be set to true.
func (ia *KNNMonItemAvg) mergeKNNMonItem(i knnMonItem) {
	if !ia.isSet {
		ia.isSet = true
		ia.Created = time.Now()
	}

	// Expand to old total.
	n := float64(ia.N)
	totalLatency := ia.AvgLatency * time.Duration(n)
	totalScore := ia.AvgScore * n
	totalSatisfaction := ia.AvgSatisfaction * n

	// Add and contract to new average. Note const to prevent zero div.
	c := 0.00000001
	n++
	ia.N = int(n)
	ia.NFailed += int(math.Floor(1 - i.Satisfaction))
	ia.AvgLatency = (time.Duration(totalLatency) + i.Latency) / time.Duration(n)
	ia.AvgScore = (totalScore + i.AvgScore) / n
	ia.AvgScoreNoFails = (totalScore + i.AvgScore) / (n - float64(ia.NFailed) + c)
	ia.AvgSatisfaction = (totalSatisfaction + i.Satisfaction) / n
}

// mergeKNNMonItemAvg merges another KNNMonItemAvg instance with this instance,
// only 'this' is changed. The merging is done as follows:
// - this.Created is set to be the oldest.
// - other.N is added to this.N.
// - All other field pairs are simply added, divided by 2, then set to this.
func (ia *KNNMonItemAvg) mergeKNNMonItemAvg(other *KNNMonItemAvg) {
	if !ia.isSet {
		*ia = *other
		return
	}
	if !other.isSet {
		return
	}

	// Set to earliest timestamp.
	if other.Created.Before(ia.Created) {
		ia.Created = other.Created
	}

	ia.Span = (ia.Span + other.Span) / 2
	ia.N = ia.N + other.N
	ia.NFailed = (ia.NFailed + other.NFailed)
	ia.AvgLatency = (ia.AvgLatency + other.AvgLatency) / 2
	ia.AvgScore = (ia.AvgScore + other.AvgScore) / 2
	ia.AvgScoreNoFails = (ia.AvgScoreNoFails + other.AvgScoreNoFails) / 2
	ia.AvgSatisfaction = (ia.AvgSatisfaction + other.AvgSatisfaction) / 2
}

// knnMonitor is intended for monitoring KNN requests in this pkg. It operates
// on the principle that current entries are added at the head of a linked list,
// and over time the entries are pushed towards the tail. This gives averages
// of monitoring items (T KNNMonItemAvg) in particular time frames.. The entries
// themselves are registered with:
//  knnMonitor.register(...)
// The method accepts a KNNEnqueueResults, which is intended to be what a KNN
// requester gets after making a query. That T contains a chan with results;
// the idea here is to put a 'man in the middle' listener between the user
// and the sender end of that chan, then capture the results that way.
// Intended setup:
// - User makes a knn request.
// - A instance of KNNEnqueueResult is made, by default it goes to both the
//   requester and the internal request processing (latter sends results to
//   the former).
// - That KNNEnqueueResult (A) is put into knnMonitor.register(...), which
//   returns another KNNEnqueueResult (B).
// - Internal request processing gets A, requester gets B.
// - Read with knnMonitor.average(...)
//
// Note; thread safe.
type knnMonitor struct {
	mx sync.Mutex
	et timex.EventTracker[KNNMonItemAvg]
}

// registerMonItem merges a knnMonItem into the head of the internal linked list.
func (m *knnMonitor) registerMonItem(item knnMonItem) {
	m.mx.Lock()
	defer m.mx.Unlock()

	m.et.Register(func(created time.Time, n int, monItem KNNMonItemAvg) KNNMonItemAvg {
		// Handle unset.
		if !monItem.isSet {
			monItem.isSet = true
			monItem.Created = created
			monItem.Span = m.et.Cfg().MinStep
		}

		monItem.mergeKNNMonItem(item)
		return monItem
	})
}

// average merges together all KNNMonItemAvg instances that were created between
// time.Now() and time.Now().Sub(period). For example, if period is 1 minute,
// then the returned items will be the average for the last minute.
//
// Thread safe.
func (m *knnMonitor) average(period time.Duration) KNNMonItemAvg {
	stamp := time.Now()
	m.mx.Lock()
	defer m.mx.Unlock()

	kmItem := KNNMonItemAvg{}
	m.et.Iter(func(i int, etItem *timex.EventTrackerItem[KNNMonItemAvg]) bool {
		include := stamp.Sub(etItem.Created) <= period
		// Set first.
		if include && !kmItem.isSet {
			kmItem = etItem.Payload
			return include
		}
		// Merge into first.
		if include {
			kmItem.mergeKNNMonItemAvg(&etItem.Payload)
		}
		return include
	})

	return kmItem
}

// knnMonitorRegisterArgs is intended as args for knnMonitor.register(...).
type knnMonitorRegisterArgs struct {
	knnEnqueueResult KNNEnqueueResult // What to listen for.
	k                int              // Number of excepted KNN request results.
	ttl              time.Duration    // Listen deadline (mitigate leaks).
}

// register puts a monitoring listener on items sent through
// args.knnEnqueueResult.Pipe and the returned KNNEnqueueResult.Pipe.
// Intended setup:
// - User makes a knn request.
// - A instance of KNNEnqueueResult is made, by default it goes to both the
//   requester and the internal request processing (latter sends results to
//   the former).
// - Put that KNNEnqueueResult (A) here, another (B) is returned.
// - Internal request processing gets A, requester gets B.
//
// Note; thread safe.
func (m *knnMonitor) register(args knnMonitorRegisterArgs) KNNEnqueueResult {
	out := KNNEnqueueResult{
		Pipe:   make(chan knnc.ScoreItems, cap(args.knnEnqueueResult.Pipe)),
		Cancel: args.knnEnqueueResult.Cancel,
	}

	// Leak prevention.
	ctx, ctxCancel := context.WithDeadline(
		context.Background(),
		time.Now().Add(args.ttl*10),
	)

	go func() {
		defer close(out.Pipe)
		defer ctxCancel()

		stamp := time.Now()
		// Rcv safely with timeout.
		safeChanIter(safeChanIterArgs[knnc.ScoreItems]{
			ch:  args.knnEnqueueResult.Pipe,
			ctx: ctx,
			rcv: func(scoreItems knnc.ScoreItems) bool {
				// Send (safely with timeout) result on all exit paths.
				defer safeChanSend(safeChanSendArgs[knnc.ScoreItems]{
					ch:  out.Pipe,
					ctx: ctx,
					elm: scoreItems,
				})
				// Update stamp on all exit paths.
				defer func() {
					stamp = time.Now()
				}()

				delta := time.Now().Sub(stamp)
				scoreItems = scoreItems.Trim()

				// Guard zero div.
				if len(scoreItems) == 0 {
					m.registerMonItem(knnMonItem{Latency: delta})
					return true
				}

				// Total -> average.
				totalScore := 0.
				for _, scoreItem := range scoreItems {
					totalScore += scoreItem.Score
				}

				m.registerMonItem(knnMonItem{
					Latency:      delta,
					AvgScore:     totalScore / float64(len(scoreItems)),
					Satisfaction: float64(len(scoreItems)) / float64(args.k),
				})

				return true
			},
		})
	}()

	return out
}
