package requestman

import (
	"context"
	"time"

	"github.com/crunchypi/ddrop/pkg/knnc"
	"github.com/crunchypi/ddrop/pkg/timex"
)

/*
File contains a queue that does controlled processing of knn requests. Top-level
logic is to accept instances of T knnRequest, process them with top n parent
goroutines using knnRequest.consume, which handles the interaction with pkg/knnc
and feeding the results to knnRequest.enqueueResult
*/

// knnQueueItem is intended as a single item in the knnQueue.queue chan. It
// contains a knnRequest- and knnNamespacesItem, such that the former can
// access data in the latter. knnQueueItem also completely handles a knnRequest
// with the knnQueueItem.process method.
type knnQueueItem struct {
	nsItem  knnNamespacesItem
	request knnRequest
}

// process uses the internal knn searchspace as data in order to consume the
// internal knnRequest. Specifically:
//  knnQueueItem.request.consume(knnQueueItem.nsItem.searchSpaces).
//
// This method also registers the time spent on a KNN search into
// nsItem.latency.
//
// There are a few cases where a knn request is dropped:
// 1) nsItem is not initialized properly (contains nil values).
// 2) knn request is cancelled, using knnRequest.enqueueResult.Cancel.
// 2) knnRequest.args.TTL is too short for the estimated time extense.
//    This is calculated based on delta time since knnQueueItem.request
//    was created (.created field) _and_ the average latency of
//    knnQueueItem.nsItem.latency.AverageSTD().
func (qi *knnQueueItem) process() {
	// Note, not doing 'defer close(qi.request.enqueueResult.Pipe)' because
	// that is done in qi.request.consume. Doing it again might lead to a
	// double close and panic.

	// This shouldn't really happend but adding for safety.
	if qi.nsItem.searchSpaces == nil || qi.nsItem.latency == nil {
		close(qi.request.enqueueResult.Pipe)
		return
	}

	// Might have been cancelled while in queue.
	if qi.request.enqueueResult.Cancel.Cancelled() {
		close(qi.request.enqueueResult.Pipe)
		return
	}

	// Check that time waited in queue + estimated query time does not exceed
	// the acceptable latency / deadline.
	queueWait := time.Now().Sub(qi.request.created)
	queryWaitEstimation, _ := qi.nsItem.latency.AverageSTD()
	if queueWait+queryWaitEstimation > qi.request.args.TTL {
		close(qi.request.enqueueResult.Pipe)
		return
	}

	defer qi.nsItem.latency.RegisterCallback()()
	// This closes the qi.request.enqueueResult.Pipe channel.
	qi.request.consume(qi.nsItem.searchSpaces) /* TODO: handle fail? */
}

// knnQueue does controlled processing of knn requests with a defined max amount
// of _parent_ goroutines. It has an 'eventloop' which goes through items in a
// chan of knnQueueItem, and calls their (knnQueueItem).process() method. See
// knnQueueItem and knnQueueItem.process docs for more detailed info.
type knnQueue struct {
	// latency tracks the average queue time.
	latency *timex.LatencyTracker
	queue   chan knnQueueItem
	// maxConcurrent specifies the highest amount of _parent_ goroutines that can
	// be used for a knn request (which in itself can use multiple goroutines).
	maxConcurrent int

	// ctx is used for stopping the processing loop in startProcessing.
	// Will wait until all requests are done before quitting.
	ctx context.Context
}

// startProcessing starts the queue processing / event loop. It iterates over the
// internal queued knnQueueItems, of which the .process() method is called. The
// loop blocks if the number of concurrent knnQueueItems.process() routines exceeds
// knnQueue.maxConcurrent, for the purpose of controlling load. Additionally, iters
// update the internal latency tracker. Method itself will block.
func (q *knnQueue) startProcessing() {
	ticker := knnc.ActiveGoroutinesTicker{}
	for qItem := range q.queue {
		ticker.BlockUntilBelowN(q.maxConcurrent)

		go func(qItem knnQueueItem) {
			done := ticker.AddAwait()
			defer done()

			queueWait := time.Now().Sub(qItem.request.created)
			q.latency.Register(queueWait)
			if queueWait > qItem.request.args.TTL {
				return
			}

			qItem.process()
		}(qItem)

		// Check graceful shutdown signal.
		select {
		case <-q.ctx.Done():
			ticker.BlockUntilBelowN(1)
			return
		default:
			continue
		}
	}
}
