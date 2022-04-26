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
// - Monitor  : true,
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
		Monitor:   true,
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

/*
--------------------------------------------------------------------------------
Utils for consistent setup of Server which is intended to be used for testing.
This is in the form of a test node and test network.
--------------------------------------------------------------------------------
*/

//testNode is a container of a Server and some associated values (like how the
// internal requestman.Handle is set up) and helper methods. It is also intended
// as a single node in the testNetwork T.
type testNode struct {
	addr     string
	server   *Server
	rManMeta *requestMananagerMeta
	stopFunc func() // stopFunc is used to shut down the server.
}

// newTestNode is a factory func for T testNode. Its internal requestman.Handle
// is set up using newRequestManagerMeta(), see docs for that for more info.
func newTestNode(addr string) (*testNode, error) {
	rManMeta := newRequestManagerMeta()
	handleArgs := rman.NewHandleArgs{
		NewSearchSpaceArgs:    rManMeta.newSearchSpaceArgs,
		NewLatencyTrackerArgs: rManMeta.newLatencyTrackerArgs,
		KNNQueueBuf:           rManMeta.knnQueueBuf,
		KNNQueueMaxConcurrent: rManMeta.knnQueueMaxConcurrent,
		Ctx:                   context.Background(),
		NewKNNMonitorArgs:     rManMeta.newKNNMonitorArgs,
	}

	s, ok := NewServer(addr, handleArgs)
	if !ok {
		s := "testNode setup failed, invalid requestman.Handle cfg"
		return nil, errors.New(s)
	}

	stopFunc, err := s.StartListen()
	if err != nil {
		s := "could not start server:"
		return nil, errors.New(fmt.Sprintln(s, err))
	}

	tn := testNode{
		addr:     addr,
		server:   s,
		rManMeta: rManMeta,
		stopFunc: stopFunc,
	}

	return &tn, nil
}

// fill is a convenience method for filling the internal requestmanager.Handle
// with "n" random data, using handle.AddData method. The added vectors are
// random, with dimension set as requestManagerHandleWrap.rManMeta.poolVecDim.
// Note, will simply panic if data could not be added, which can be caused
// by two cases:
// 1) the aforementioned poolVecDim is <= 0, so a rand vec is impossible.
// 2) handle.AddData returns false (see doc for that, could be over-capacity).
func (tn *testNode) fill(n int) {
	for i := 0; i < n; i++ {
		vec, ok := mathx.NewSafeVecRand(tn.rManMeta.poolVecDim)
		if !ok {
			panic("couldn't create a new random vec")
		}
		ns := tn.rManMeta.namespace
		dc := rman.DistancerContainer{D: vec}
		ok = tn.server.rManHandle.AddData(ns, dc, []byte{})
		if !ok {
			panic("could not add new data")
		}
	}
}

// makeLatency makes 'n' random KNN requests with the inner requestman.Handle instance,
// with 'interval' pauses in between.
func (tn *testNode) makeLatency(n int, interval time.Duration) error {
	for i := 0; i < n; i++ {
		time.Sleep(interval)
		args := tn.rManMeta.randKNNArgs()
		enqueueResult, ok := tn.server.rManHandle.KNN(args)
		if !ok {
			return errors.New("could not make a knn request")
		}
		<-enqueueResult.Pipe
	}
	return nil
}

// withTestNode is a convenience for setting up and cleaning resources, which
// is meant to reduce boilerplate in tests. It spins up a Server with the
// given address (returns err on fail).
func withTestNode(addr string, rcv func(*testNode)) error {
	tn, err := newTestNode(addr)
	if err != nil {
		return err
	}
	defer tn.stopFunc()

	rcv(tn)
	return nil
}

// testNetwork is used to simulate an rpc network as defined in this pkg.
type testNetwork struct {
	nodes map[string]*testNode // key is addr.
	addrs []string             // convenience collection of addrs.
}

// newTestNetwork sets up a new test network with the given addrs.
// Each node is set up using newRequestManagerWrap(), see docs for that
// for more info about all the different configurations.
func newTestNetwork(addrs []string) (*testNetwork, error) {
	tNetwork := testNetwork{
		nodes: make(map[string]*testNode),
		addrs: addrs,
	}
	for _, addr := range addrs {
		tn, err := newTestNode(addr)
		if err != nil {
			return nil, err
		}

		tNetwork.nodes[addr] = tn
	}

	return &tNetwork, nil
}

// stop calls the stopFunc on all internal testNode instances, i.e shuts down
// all the Server instances.
func (tn *testNetwork) stop() {
	for _, node := range tn.nodes {
		node.stopFunc()
	}
}

// withNetwork is similar as withServer(...), the maing difference being that
// this function lends an entire testNetwork, as opposed to a single server.
// This is meant to reduce setup- and shutdown boilerplate.
func withNetwork(t *testing.T, numNodes int, rcv func(n *testNetwork)) error {
	addrs := make([]string, numNodes)
	for i := 0; i < numNodes; i++ {
		addrs[i] = freeLocalNoFail(t)
	}

	tNetwork, err := newTestNetwork(addrs)
	if err != nil {
		return err
	}
	defer tNetwork.stop()

	rcv(tNetwork)
	return nil
}
