package ops

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/crunchypi/ddrop/pkg/knnc"
	"github.com/crunchypi/ddrop/pkg/mathx"
	"github.com/crunchypi/ddrop/pkg/timex"
	rman "github.com/crunchypi/ddrop/service/requestman"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

/*
--------------------------------------------------------------------------------
Misc utils.
--------------------------------------------------------------------------------
*/

// freePort gets a free port. Courtesy of
// https://gist.github.com/sevkin/96bdae9274465b2d09191384f86ef39d
func freePort() (port int, err error) {
	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			return l.Addr().(*net.TCPAddr).Port, nil
		}
	}
	return
}

// freeLocalNoFail is a wrapper around freePort(), where the returner err
// raises a t.Fatal if not nil, using the given testing.T. Otherwise, the
// port will be returned as a str in the format ":x".
func freeLocalNoFail(t *testing.T) string {
	port, err := freePort()
	if err != nil {
		t.Fatal("could not get a free port;", err)
	}

	return fmt.Sprintf(":%d", port)
}

// randFloat64Slice returns a random float slice with the given dimension.
// Will return (nil, false) if the dimension is not a positive integer.
func randFloat64Slice(dim int) ([]float64, bool) {
	if dim <= 0 {
		return nil, false
	}

	s := make([]float64, dim)
	for i := 0; i < dim; i++ {
		s[i] = rand.Float64()
	}

	return s, true
}

// randKNNArgs returns (requestmanager.) KNNArgs with some random fields.
// The fiels are set as follows:
// - Namespace: namespace (given to this func).
// - Priority : rand range [1, 4].
// - QueryVec : random vec with given dim (given to this func).
// - KNNMethod: requestman.KNNMethodCosineSimilarity,
// - Ascend   : false,
// - K        : rand range [7, 10].
// - Extent   : rand range [0.4, 0.6].
// - Accept   : rand range [0.9, 1.0].
// - Reject   : rand range [0.5, 0.9].
// - TTL      : rand range [10ms, 100ms].
//
// NOTE: will panic if the returned KNNArgs.Ok() == false. This will happen if
// the given dim <= 0, or if this func is implemented incorrectly.
func randKNNArgs(namespace string, dim int) rman.KNNArgs {
	v, _ := randFloat64Slice(dim)
	ttlMin := time.Millisecond * 10
	ttlMax := time.Millisecond * 100
	ttl := rand.Int63n(int64(ttlMax)-int64(ttlMin)) + int64(ttlMin)

	knnArgs := rman.KNNArgs{
		Namespace: namespace,
		Priority:  rand.Intn(4) + 1,
		QueryVec:  v,
		KNNMethod: rman.KNNMethodCosineSimilarity,
		Ascending: false,
		K:         rand.Intn(3) + 7,
		Extent:    rand.Float64()*(0.6-0.4) + 0.4,
		Accept:    rand.Float64()*(1.0-0.9) + 0.9,
		Reject:    rand.Float64()*(0.9-0.5) + 0.5,
		TTL:       time.Duration(ttl),
	}

	if !knnArgs.Ok() {
		panic("could not create a new valid requestman.KNNArgs: impl err")
	}

	return knnArgs
}

/*
--------------------------------------------------------------------------------
Utils for consistent setup of requestmanager.Handle
--------------------------------------------------------------------------------
*/

// requestManagerMeta stores information about how a requestmanager.Handle was
// set up. This is convenient for testing purposes (in addition to a Handle)
// because it gives quick access to some useful information.
type requestMananagerMeta struct {
	namespace  string
	poolVecDim int

	knnQueueBuf           int
	knnQueueMaxConcurrent int

	newSearchSpaceArgs    knnc.NewSearchSpacesArgs
	newLatencyTrackerArgs timex.NewLatencyTrackerArgs
	newKNNMonitorArgs     timex.NewLatencyTrackerArgs
}

// randKNNArgs defers the call to randKNNArgs(...) func in this pkg, using
// the internal 'naemspace' and 'poolVecDim' as arguments.
func (m *requestMananagerMeta) randKNNArgs() rman.KNNArgs {
	return randKNNArgs(m.namespace, m.poolVecDim)
}

// newRequestManagerMeta is a factory func.
// It sets fiels as follows:
// - namespace                              : "test",
// - poolVecDim                             : 50,
//
// - knnQueueBuf                            : 100,
// - knnQueueMaxConcurrent                  : 100,
//
// - newSearchSpaceArgs.SearchSpaceMaxCap   : 10k.
// - newSearchSpaceArgs.MaxN                : 10k,
// - newSearchSpaceMaitenanceTaskInterval   : 100ms,
//
// - newLatencyTrackerArgs.MaxChainLinkN    : 10,
// - newLatencyTrackerArgs.MinChainLinkSize : 100ms,
// - newLatencyTrackerArgs.StandardPeriod   : 1s,
//
// - newKNNMonitor.MaxChainLinkN            : 10,
// - newKNNMonitor.MinChainLinkSize         : 1s,
//
func newRequestManagerMeta() *requestMananagerMeta {
	newSearchSpaceArgs := knnc.NewSearchSpacesArgs{
		SearchSpacesMaxCap:      10_000,
		SearchSpacesMaxN:        10_000,
		MaintenanceTaskInterval: time.Millisecond * 100,
	}

	newLatencyTrackerArgs := timex.NewLatencyTrackerArgs{
		MaxChainLinkN:    10,
		MinChainLinkSize: time.Millisecond * 100,
		StandardPeriod:   time.Second,
	}

	return &requestMananagerMeta{
		namespace:  "test",
		poolVecDim: 50,

		knnQueueBuf:           100,
		knnQueueMaxConcurrent: 100,

		newSearchSpaceArgs:    newSearchSpaceArgs,
		newLatencyTrackerArgs: newLatencyTrackerArgs,
		newKNNMonitorArgs:     newLatencyTrackerArgs,
	}
}

// requestManagerHandleWrap is a concenience type which wraps around a
// requestmanager.Handle and the way it was set up (meta data). It is
// also used for some convenience methods.
type requestManagerHandleWrap struct {
	handle   *rman.Handle
	rManMeta *requestMananagerMeta
}

// newRequestManagerWrap is a factory func. It uses newRequestManagerMeta()
// (in this pkg/file) as instructions. See docs for that func for more info.
// Note, requestManager.NewHandleArgs.Ctx is set as context.Background().
// Additionally, will panic if a new handle could not be set up, as that is
// purely an implementation error here.
func newRequestManagerWrap() *requestManagerHandleWrap {
	rManMeta := newRequestManagerMeta()
	handle, ok := rman.NewHandle(rman.NewHandleArgs{
		NewSearchSpaceArgs:    rManMeta.newSearchSpaceArgs,
		NewLatencyTrackerArgs: rManMeta.newLatencyTrackerArgs,
		KNNQueueBuf:           rManMeta.knnQueueBuf,
		KNNQueueMaxConcurrent: rManMeta.knnQueueMaxConcurrent,
		Ctx:                   context.Background(),
		NewKNNMonitorArgs:     rManMeta.newKNNMonitorArgs,
	})

	if !ok {
		panic("test setup failed")
	}
	return &requestManagerHandleWrap{
		handle:   handle,
		rManMeta: rManMeta,
	}
}

// fill is a convenience method for filling the internal requestmanager.Handle
// with "n" random data, using handle.AddData method. The added vectors are
// random, with dimension set as requestManagerHandleWrap.rManMeta.poolVecDim.
// Note, will simply panic if data could not be added, which can be caused
// by two cases:
// 1) the aforementioned poolVecDim is <= 0, so a rand vec is impossible.
// 2) handle.AddData returns false (see doc for that, could be over-capacity).
func (w *requestManagerHandleWrap) fill(n int) {
	for i := 0; i < n; i++ {
		vec, ok := mathx.NewSafeVecRand(w.rManMeta.poolVecDim)
		if !ok {
			panic("couldn't create a new random vec")
		}
		dc := rman.DistancerContainer{D: vec}
		ok = w.handle.AddData(w.rManMeta.namespace, dc, []byte{})
		if !ok {
			panic("could not add new data")
		}
	}
}

/*
--------------------------------------------------------------------------------
Utils for consistent setup of Server which is intended to be used for testing.
--------------------------------------------------------------------------------
*/

// serverWrap is a convenience wrapper around a Server instance and the
// requestManagerHandleWrap which it was set up with.
type serverWrap struct {
	server   *Server
	rManWrap *requestManagerHandleWrap
}

// withServer is a convenience for setting up and cleaning resources, which
// is meant to reduce boilerplate in tests. It spins up a Server with the
// given address (returns err on fail), then passes the
// requestManagerHandleWrap that was used to create the aforementioned server
// to the rcv func.
func withServer(addr string, rcv func(w *requestManagerHandleWrap)) error {
	w := newRequestManagerWrap()
	s, ok := NewServer(addr, w.handle)
	if !ok {
		return errors.New("could not setup server")
	}
	stop, err := s.StartListen()
	if err != nil {
		return errors.New(fmt.Sprintf("could not setup server wrap: %v", err))
	}
	defer stop()
	rcv(w)

	return nil
}