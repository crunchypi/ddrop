package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/crunchypi/ddrop/service/ops"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

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

// post is a convenience func on top of http.Post, which provides simple-to-use
// generic syntax. It simply tries to encode "data" into a json, post it to the
// url, then attempts to unpack the response from a json into an instance of T
// which is to be returned. The error is not nil on these conditions:
// - "data" cannot be encoded into a json.
// - http.Post(...) returns an error.
// - T cannot be decoded from a json.
func post[T any](url string, data any) (T, error) {
	var r T

	// Encode send data.
	b, err := json.Marshal(data)
	if err != nil {
		return r, err
	}
	// Post and get reply.
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(b))
	if err != nil {
		return r, err
	}
	defer resp.Body.Close()

	// Decode end return data.
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return r, err
	}
	return r, json.Unmarshal(b, &r)
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

// testNode represents a virtual node which uses both the http server in this
// pkg and the rpc server in the ops pkg. Set up with newTestNode(...).
type testNode struct {
	addrAPI string  // Addr for the api server of this pkg.
	addrRPC string  // Addr for the rpc server of the ops pkg.
	handle  *handle // Ptr to server handle of this pkg.
	stopF   func()  // Ctx stop func for the handle in this struct.
}

// newTestNode attempts to set up a testNode, filling the fields.
// It uses func freeLocalNoFail to set both the addrAPI and addrRPC fields,
// starts a new http server using StartServer, then steals the handle.
func newTestNode(t *testing.T) testNode {
	tNode := testNode{
		addrAPI: freeLocalNoFail(t),
		addrRPC: freeLocalNoFail(t),
	}

	wg := sync.WaitGroup{}
	wg.Add(1)

	// Used to steal the handle and wait until http server is up before exit.
	onRunning := func(h *handle) {
		defer wg.Done()
		tNode.handle = h
	}

	ctx, ctxCancel := context.WithCancel(context.Background())
	tNode.stopF = ctxCancel

	go func() {
		StartServer(StartServerArgs{
			Addr:                   tNode.addrAPI,
			Ctx:                    ctx,
			ReadTimeout:            time.Minute,
			WriteTimeout:           time.Minute,
			UpdateFrequencyAddrSet: time.Minute,
			onRunning:              onRunning,
		})
	}()

	wg.Wait()
	return tNode
}

// startRPC tries to set up (and start) a new ops.Server and put it into
// testNode.handle. The rpc addr is taken from testNode.addrRPC, while
// the args (requestman.NewHandleArgs) are defined as follows:
//
// - Ctx                                    : testNode.handle.ctx,
// - knnQueueBuf                            : 100,
// - knnQueueMaxConcurrent                  : 100,
//
// - newSearchSpaceArgs.SearchSpaceMaxCap   : 10k.
// - newSearchSpaceArgs.MaxN                : 100,
// - newSearchSpaceMaitenanceTaskInterval   : 1s,
//
// - newLatencyTrackerArgs.MaxChainLinkN    : 10,
// - newLatencyTrackerArgs.MinChainLinkSize : 1s,
// - newLatencyTrackerArgs.StandardPeriod   : 1s,
//
// - newKNNMonitor.MaxChainLinkN            : 10,
// - newKNNMonitor.MinChainLinkSize         : 1s,
//
func (tn *testNode) startRPC() error {
	if tn.handle == nil {
		return errors.New("internal handle not set")
	}

	// Try setup and start new rpc server.
	args := newRequestManagerHandleArgs{
		NewSearchSpacesArgs: newSearchSpacesArgs{
			SearchSpacesMaxCap:      10_000,
			SearchSpacesMaxN:        100,
			MaintenanceTaskInterval: time.Second,
		},
		NewLatencyTrackerArgs: newLatencyTrackerArgs{
			MaxChainLinkN:    10,
			MinChainLinkSize: time.Second,
			StandardPeriod:   time.Second,
		},
		KNNQueueBuf:           100,
		KNNQueueMaxConcurrent: 100,
		NewKNNMonitorArgs: newLatencyTrackerArgs{
			MaxChainLinkN:    10,
			MinChainLinkSize: time.Second,
			StandardPeriod:   time.Second,
		},
	}
	s, ok := ops.NewServer(tn.addrRPC, args.export(tn.handle.ctx))
	if !ok {
		return errors.New("could not set up a new ops.Server")
	}
	stop, err := s.StartListen()
	if err != nil {
		return errors.New("could not start a new ops.Server")
	}

	// Set state of handle.
	tn.handle.rpcServerWrap.mx.Lock()
	defer tn.handle.rpcServerWrap.mx.Unlock()
	tn.handle.rpcServerWrap.inner.server = s
	tn.handle.rpcServerWrap.inner.serverStopF = stop
	tn.handle.rpcServerWrap.state = rpcServerStateStarted
	tn.handle.addrSet.addrsMaintanedLocked(tn.addrRPC)
	return nil
}

// testNode is a slice of T testNode, containing a few convenience methods.
type testNetwork struct {
	nodes []testNode
}

// newTestNetwork tries to set up n testNode instances using newTestNode(...).
// Additionally, all the nodes are made to "be aware of eachother", i.e all
// their testNode.handle.addrSet are filled with all testNode.addrRPC.
func newTestNetwork(n int, t *testing.T) testNetwork {
	tn := testNetwork{nodes: make([]testNode, 0, n)}
	for i := 0; i < n; i++ {
		tn.nodes = append(tn.nodes, newTestNode(t))
	}
	// Register all rpc addrs in all nodes.
	addrs := make([]string, 0, n)
	for _, node := range tn.nodes {
		addrs = append(addrs, node.addrRPC)
	}
	for _, node := range tn.nodes {
		node.handle.addrSet.addrsMaintanedLocked(addrs...)
	}

	return tn
}

// stop calls the testNode.stopF on all internal nodes.
func (tn *testNetwork) stop() {
	for _, node := range tn.nodes {
		node.stopF()
	}
}

// startRPC calls testNode.startRPC on all internal nodes.
func (tn *testNetwork) startRPC() error {
	for _, node := range tn.nodes {
		if err := node.startRPC(); err != nil {
			return err
		}
	}
	return nil
}

// fill tries to fill all nodes with random vector data, using
// testNode.handle.rpcServerWrap.inner.server.AddData(...).
// This will put "n" data (vecs) with "dim" as dimension into the "ns" namespace.
// Note, will panic if:
// - n <= 0.
// - The AddData method, as described above, returns nil.
// - The ops.SResp (used as response args ptr of the AddData method) contains any
//   bools indicating that data could not be added.
func (tn *testNetwork) fill(ns string, n, dim int) {
	for _, node := range tn.nodes {
		node.handle.rpcServerWrap.mx.Lock()
		defer node.handle.rpcServerWrap.mx.Unlock()

		addDataArgs := make([]ops.AddDataArgs, n)
		for i := 0; i < n; i++ {
			v, ok := randFloat64Slice(dim)
			if !ok {
				panic("could not create a new vector")
			}
			addDataArgs[i] = ops.AddDataArgs{
				Namespace: ns,
				Vec:       v,
				Data:      []byte{},
			}
		}

		sArgs := ops.SArgs[[]ops.AddDataArgs]{Payload: addDataArgs}
		sResp := ops.SResp[[]bool]{Payload: make([]bool, 0, 1)}

		err := node.handle.rpcServerWrap.inner.server.AddData(sArgs, &sResp)
		if err != nil {
			panic(err)
		}
		for _, b := range sResp.Payload {
			if !b {
				panic("got false bool resp from ops.Server.AddData")
			}
		}
	}
}

// withNetwork is a convenience func for setting up a testNetwork instance,
// then tearing it down.
func withNetwork(t *testing.T, numNodes int, rcv func(tn *testNetwork)) {
	tNetwork := newTestNetwork(numNodes, t)
	defer tNetwork.stop()

	if err := tNetwork.startRPC(); err != nil {
		t.Fatal("could not start rpc servers:", err)
	}
	rcv(&tNetwork)
}
