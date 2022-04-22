package requestman

import (
	"context"
	"math/rand"
	"runtime"
	"testing"
	"time"

	"github.com/crunchypi/ddrop/pkg/knnc"
	"github.com/crunchypi/ddrop/pkg/mathx"
	"github.com/crunchypi/ddrop/pkg/timex"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Convenience, see func def for field vals. Panics if setup fails.
// args:
// - sSpaceMaxN is used for SearchSpacesMaxCap and SearchSpaceMaxN.
// - knnQueueN is used for knnQueue.queue buf and MaxConcurrent.
// - ctx is used as context for the handle. Accepts nil.
func newTestHandle(sSpaceMaxN, knnQueueN int, ctx context.Context) *Handle {
	if ctx == nil {
		ctx = context.Background()
	}
	h, ok := NewHandle(NewHandleArgs{
		NewSearchSpaceArgs: knnc.NewSearchSpacesArgs{
			SearchSpacesMaxCap:      sSpaceMaxN,
			SearchSpacesMaxN:        sSpaceMaxN,
			MaintenanceTaskInterval: time.Millisecond * 100,
		},
		NewLatencyTrackerArgs: timex.NewLatencyTrackerArgs{
			MaxChainLinkN:    10,
			MinChainLinkSize: time.Millisecond * 100,
			StandardPeriod:   time.Second,
		},
		KNNQueueBuf:           knnQueueN,
		KNNQueueMaxConcurrent: knnQueueN,
		Ctx:                   ctx,
		NewKNNMonitorArgs: timex.NewLatencyTrackerArgs{
			MaxChainLinkN:    1,
			MinChainLinkSize: time.Second,
		},
	})

	if !ok {
		panic("impl err: expected functioning *Handle meant for testing")
	}

	return h
}

// Convenience, makes a random (valid) KNNRequest with TTL=time.Minute.
// Panics if vec creation fails or returned KNNargs.OK() == false.
func newTestKNNArgs(dim int, ns string) KNNArgs {
	v, ok := randFloat64Slice(dim)
	if !ok {
		panic("could not create a new vec")
	}

	args := KNNArgs{
		Namespace: ns,
		Priority:  rand.Intn(4) + 1,
		QueryVec:  v,
		KNNMethod: KNNMethodCosineSimilarity,
		Ascending: false,
		K:         rand.Intn(3) + 7,
		Extent:    rand.Float64()*(0.2-0.1) + 0.1,
		Accept:    rand.Float64()*(1.0-0.9) + 0.9,
		Reject:    rand.Float64()*(0.9-0.8) + 0.8,
		TTL:       time.Minute,
	}

	if !args.Ok() {
		panic("new rand KNNArgs returned false on args.Ok()")
	}

	return args
}

func TestHandleAddData(t *testing.T) {
	ns := "test"
	dc := DistancerContainer{D: mathx.NewSafeVec(9)}
	h := newTestHandle(100, 100, nil)

	if ok := h.AddData(ns, dc, []byte{}); !ok {
		t.Fatal("got not-ok when adding data")
	}

	nsItem, ok := h.knnNamespaces.get(ns)
	if !ok {
		t.Fatal("got not-ok when trying to retrieve namespaced SearchSpaces")
	}

	if nsItem.searchSpaces == nil {
		t.Fatal("retrieved namespaced SearchSpaces is nil")
	}

	_, n := nsItem.searchSpaces.Len()
	if n != 1 {
		t.Fatalf("Unexpected SearchSpaces.Len: %v", n)
	}
}

// NOTE: Weak test, it only checks that multiple concurrent KNN requests
// go through (KNNArgs.TTL=Hour so everything passes), and don't return empty.
func TestHandleKNN(t *testing.T) {
	vecDim := 10
	namespace := "test"

	nData := 100_000
	maxConcurrent := 100

	ctx, ctxCancel := context.WithCancel(context.Background())
	h := newTestHandle(nData, maxConcurrent, ctx)

	nGoroutines := runtime.NumGoroutine()

	// Add some data.
	for i := 0; i < nData; i++ {
		v, ok := mathx.NewSafeVecRand(vecDim)
		if !ok {
			t.Fatal("impl error; could not create a vec")
		}
		if ok := h.AddData(namespace, DistancerContainer{D: v}, []byte{}); !ok {
			t.Fatal("unexpected not-ok when adding data")
		}
	}

	// Make requests.
	enqueueResults := make(chan KNNEnqueueResult)
	go func() {
		defer close(enqueueResults)
		for i := 0; i < maxConcurrent*2; i++ {
			args := newTestKNNArgs(vecDim, namespace)
			args.TTL = time.Hour
			// ok (not nil) checked in loop over enqueueResult below.
			r, _ := h.KNN(args)
			enqueueResults <- r
		}
	}()

	// Check results.
	for enqRes := range enqueueResults {
		// Will be nil is h.KNN(...) above returned false.
		if enqRes.Pipe == nil {
			t.Fatal("one EnqueueResult.Pipe is unexpectedly nil")
		}

		r, ok := <-enqRes.Pipe
		if !ok {
			t.Fatal("pipe closed; one KNN cancelled unexpectedly")
		}

		if len(r.Trim()) == 0 {
			t.Fatal("one KNN request got 0 result items.")
		}
	}

	ctxCancel()
	// Check leaks.
	runtime.GC()
	if nGoroutines != runtime.NumGoroutine() {
		s := "number of goroutines at the end of this test is not the"
		s += " same as at the start; possible leak. Want %v, have %v."
		t.Fatalf(s, nGoroutines, runtime.NumGoroutine())
	}
}
