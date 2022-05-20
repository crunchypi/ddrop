package api

import (
	"context"
	"testing"
	"time"

	"github.com/crunchypi/ddrop/service/ops"
	rman "github.com/crunchypi/ddrop/service/requestman"
)

func TestPing(t *testing.T) {
	addr := freeLocalNoFail(t)
	url := "http://localhost" + addr + "/ping"

	ctx, ctxStop := context.WithCancel(context.Background())
	ok, err := StartServer(StartServerArgs{
		Addr:                   addr,
		Ctx:                    ctx,
		ReadTimeout:            time.Minute,
		WriteTimeout:           time.Minute,
		UpdateFrequencyAddrSet: time.Second,
		onRunning: func(h *handle) {
			defer ctxStop()

			r, err := post[bool](url, true)
			if err != nil {
				t.Fatal(err)
			}
			if !r {
				t.Fatal("unexpected false return from server")
			}
		},
	})

	if !ok || err != nil {
		t.Fatalf("issue with server, returned bool=%v, err=%v", ok, err)
	}
}

func TestRPCAddrsPut(t *testing.T) {
	addr := freeLocalNoFail(t)
	url := "http://localhost" + addr + "/ops/rpc/addrs/put"

	ctx, ctxStop := context.WithCancel(context.Background())
	ok, err := StartServer(StartServerArgs{
		Addr:                   addr,
		Ctx:                    ctx,
		ReadTimeout:            time.Minute,
		WriteTimeout:           time.Minute,
		UpdateFrequencyAddrSet: time.Second,
		onRunning: func(h *handle) {
			defer ctxStop()
			newAddr := "test"

			r, err := post[[]string](url, []string{newAddr})
			if err != nil {
				t.Fatal(err)
			}
			if r == nil || len(r) == 0 {
				t.Fatal("got nil / zero len result")
			}
			if r[0] != newAddr {
				t.Fatal("got unexpected addr result:", r)
			}
			if len(h.addrSet._addrs) != 1 {
				t.Fatal("internal addr set of handle is not len 1")
			}
		},
	})

	if !ok || err != nil {
		t.Fatalf("issue with server, returned bool=%v, err=%v", ok, err)
	}
}

func TestRPCAddrsGet(t *testing.T) {
	addr := freeLocalNoFail(t)
	url := "http://localhost" + addr + "/ops/rpc/addrs/get"

	ctx, ctxStop := context.WithCancel(context.Background())
	ok, err := StartServer(StartServerArgs{
		Addr:                   addr,
		Ctx:                    ctx,
		ReadTimeout:            time.Minute,
		WriteTimeout:           time.Minute,
		UpdateFrequencyAddrSet: time.Second,
		onRunning: func(h *handle) {
			defer ctxStop()
			newAddr := "test"
			h.addrSet.addrsMaintanedLocked(newAddr)

			r, err := post[[]string](url, struct{}{})
			if err != nil {
				t.Fatal(err)
			}
			if r == nil || len(r) == 0 {
				t.Fatal("got nil / zero len result")
			}
			if r[0] != newAddr {
				t.Fatal("got unexpected addr result:", r)
			}
			if len(h.addrSet._addrs) != 1 {
				t.Fatal("internal addr set of handle is not len 1")
			}
		},
	})

	if !ok || err != nil {
		t.Fatalf("issue with server, returned bool=%v, err=%v", ok, err)
	}
}

func TestRPCServerStop(t *testing.T) {
	addrAPI := freeLocalNoFail(t)
	addrRPC := freeLocalNoFail(t)
	url := "http://localhost" + addrAPI + "/ops/rpc/server/stop"

	ctx, ctxStop := context.WithCancel(context.Background())
	ok, err := StartServer(StartServerArgs{
		Addr:                   addrAPI,
		Ctx:                    ctx,
		ReadTimeout:            time.Minute,
		WriteTimeout:           time.Minute,
		UpdateFrequencyAddrSet: time.Second,
		onRunning: func(h *handle) {
			defer ctxStop()

			args := newRequestManagerHandleArgs{
				NewSearchSpacesArgs: newSearchSpacesArgs{
					SearchSpacesMaxCap:      100,
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

			newServer, ok := ops.NewServer(addrRPC, args.export(ctx))
			if !ok {
				t.Fatal("could not set up a new rpc server instance")
			}
			newServerStopF, err := newServer.StartListen()
			if err != nil {
				t.Fatal("could not start listening with the new rpc server")
			}

			newServerStopped := false
			newServerStopFChained := func() {
				newServerStopF()
				newServerStopped = true
			}

			h.rpcServerWrap.mx.Lock()
			h.rpcServerWrap.state = rpcServerStateStarted
			h.rpcServerWrap.inner.server = newServer
			h.rpcServerWrap.inner.serverStopF = newServerStopFChained
			h.rpcServerWrap.mx.Unlock()

			r, err := post[status](url, struct{}{})
			if err != nil {
				t.Fatal(err)
			}
			if r.Code != int(rpcServerStateStopped) {
				t.Fatal("got unexpected state:", r.Msg)
			}
			if !newServerStopped {
				t.Fatal("internal rpc server did not get stop signal")
			}
		},
	})

	if !ok || err != nil {
		t.Fatalf("issue with server, returned bool=%v, err=%v", ok, err)
	}
}

func TestRPCServerStart(t *testing.T) {
	addrAPI := freeLocalNoFail(t)
	addrRPC := freeLocalNoFail(t)
	url := "http://localhost" + addrAPI + "/ops/rpc/server/start"

	ctx, ctxStop := context.WithCancel(context.Background())
	ok, err := StartServer(StartServerArgs{
		Addr:                   addrAPI,
		Ctx:                    ctx,
		ReadTimeout:            time.Minute,
		WriteTimeout:           time.Minute,
		UpdateFrequencyAddrSet: time.Second,
		onRunning: func(h *handle) {
			defer ctxStop()

			args := newRequestManagerHandleArgs{
				NewSearchSpacesArgs: newSearchSpacesArgs{
					SearchSpacesMaxCap:      100,
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

			r, err := post[status](url, rpcServerStartArgs{addrRPC, args})
			if err != nil {
				t.Fatal(err)
			}
			if r.Code != int(rpcServerStateStarted) {
				t.Fatal("got unexpected state:", r.Msg)
			}
			h.rpcServerWrap.mx.Lock()
			defer h.rpcServerWrap.mx.Unlock()
			if h.rpcServerWrap.inner.server == nil {
				t.Fatal("internal rpc server is nil")
			}
		},
	})

	if !ok || err != nil {
		t.Fatalf("issue with server, returned bool=%v, err=%v", ok, err)
	}
}

func TestRPCPing(t *testing.T) {
	nNodes := 3
	url := func(addr string) string {
		return "http://localhost" + addr + "/cmd/ping"
	}
	withNetwork(t, nNodes, func(tn *testNetwork) {
		url := url(tn.nodes[0].addrAPI)
		r, err := post[[]clientResult[bool]](url, struct{}{})
		if err != nil {
			t.Fatal(err)
		}
		if len(r) != nNodes {
			t.Fatal("unexpected resp len:", len(r))
		}
		for _, cliResp := range r {
			if !cliResp.Payload {
				t.Fatal("got one not-ok from addr:", cliResp.RemoteAddr)
			}
		}
	})
}

func TestRPCAddData(t *testing.T) {
	nNodes := 3
	url := func(addr string) string {
		return "http://localhost" + addr + "/cmd/add"
	}
	withNetwork(t, nNodes, func(tn *testNetwork) {
		url := url(tn.nodes[0].addrAPI)

		opts := []addDataArgs{
			{Namespace: "", Vec: []float64{1}, Data: []byte{}},
		}

		r, err := post[[]clientResult[[]bool]](url, opts)
		if err != nil {
			t.Fatal("issue sending/receiving:", err)
		}

		if r == nil || len(r) == 0 {
			t.Fatal("empty response")
		}
		// opts.Clients.AddData adds all data to a single node.
		if r[0].Payload == nil {
			t.Fatal("empty payload")
		}
		if len(r[0].Payload) != 1 {
			t.Fatal("unexpected amt. for responses:", len(r[0].Payload))
		}
		for _, cliResp := range r {
			for _, okBool := range cliResp.Payload {
				if !okBool {
					t.Fatal("unexpected not-ok")
				}
			}
		}
	})
}

func TestRPCKNN(t *testing.T) {
	nNodes := 3
	url := func(addr string) string {
		return "http://localhost" + addr + "/cmd/knn"
	}
	withNetwork(t, nNodes, func(tn *testNetwork) {
		url := url(tn.nodes[0].addrAPI)

		// Fill search spaces / vec pools with data.
		namespace := "test"
		dim := 3
		tn.fill(namespace, 1000, dim)

		// A couple query vecs.
		v1, ok := randFloat64Slice(dim)
		if !ok {
			t.Fatal("could not make query vec no. 1")
		}
		v2, ok := randFloat64Slice(dim)
		if !ok {
			t.Fatal("could not make query vec no. 2")
		}

		opts := knnArgs{
			QueryVecs: [][]float64{v1, v2},
			Args: knnArgsPartial{
				Namespace: namespace,
				Priority:  1,
				KNNMethod: rman.KNNMethodCosineSimilarity,
				Ascending: false,
				K:         5,
				Extent:    1,
				Accept:    0.5,
				Reject:    0.4,
				TTL:       time.Hour,
				Monitor:   false,
			},
		}

		r, err := post[[]knnResp](url, opts)
		if err != nil {
			t.Fatal("issue sending/receiving:", err)
		}

		if len(r) != len(opts.QueryVecs) {
			s := "unexpected amt of resutls (with regards to no. of vecs):"
			t.Fatal(s, len(r))
		}

		for _, rItem := range r {
			if len(rItem.Results) != opts.Args.K {
				t.Fatal("unexpected amt of results (knn items per vec)")
			}
		}
	})
}

func TestSSpaceNamespaces(t *testing.T) {
	nNodes := 2
	url := func(addr string) string {
		return "http://localhost" + addr + "/info/namespaces"
	}
	withNetwork(t, nNodes, func(tn *testNetwork) {
		url := url(tn.nodes[0].addrAPI)

		namespace := "test"
		tn.fill(namespace, 1, 1)

		r, err := post[[]clientResult[[]string]](url, struct{}{})
		if err != nil {
			t.Fatal("issue sending/receiving:", err)
		}

		for _, rItem := range r {
			if rItem.Payload == nil || len(rItem.Payload) == 0 {
				t.Fatal("empty result")
			}
			if rItem.Payload[0] != namespace {
				t.Fatal("unexpected result:", rItem.Payload[0])
			}
		}
	})
}

func TestSSpaceNamespace(t *testing.T) {
	nNodes := 2
	url := func(addr string) string {
		return "http://localhost" + addr + "/info/namespace"
	}
	withNetwork(t, nNodes, func(tn *testNetwork) {
		url := url(tn.nodes[0].addrAPI)

		namespace := "test"
		tn.fill(namespace, 1, 1)

		r, err := post[[]clientResult[bool]](url, namespace)
		if err != nil {
			t.Fatal("issue sending/receiving:", err)
		}

		for _, rItem := range r {
			if !rItem.Payload {
				t.Fatal("unexpected false response")
			}
		}
	})
}

func TestSSpaceDim(t *testing.T) {
	nNodes := 2
	url := func(addr string) string {
		return "http://localhost" + addr + "/info/dim"
	}
	withNetwork(t, nNodes, func(tn *testNetwork) {
		url := url(tn.nodes[0].addrAPI)

		namespace := "test"
		dim := 3
		tn.fill(namespace, 1, dim)

		r, err := post[[]clientResult[sSpaceDimResp]](url, namespace)
		if err != nil {
			t.Fatal("issue sending/receiving:", err)
		}

		for _, rItem := range r {
			if rItem.Payload.Dim != dim {
				t.Fatal("unexpected dim response:", rItem.Payload.Dim)
			}
		}
	})
}

func TestSSpaceLen(t *testing.T) {
	nNodes := 2
	url := func(addr string) string {
		return "http://localhost" + addr + "/info/len"
	}
	withNetwork(t, nNodes, func(tn *testNetwork) {
		url := url(tn.nodes[0].addrAPI)

		namespace := "test"
		nVecs := 10
		tn.fill(namespace, nVecs, 1)

		r, err := post[[]clientResult[sSpaceLenResp]](url, namespace)
		if err != nil {
			t.Fatal("issue sending/receiving:", err)
		}

		for _, rItem := range r {
			if rItem.Payload.NVecs != nVecs {
				t.Fatal("unexpected dim response:", rItem.Payload.NVecs)
			}
		}
	})
}

func TestSSpaceCap(t *testing.T) {
	nNodes := 2
	url := func(addr string) string {
		return "http://localhost" + addr + "/info/cap"
	}
	withNetwork(t, nNodes, func(tn *testNetwork) {
		url := url(tn.nodes[0].addrAPI)

		namespace := "test"
		nVecs := 10
		tn.fill(namespace, nVecs, 1)

		r, err := post[[]clientResult[sSpaceCapResp]](url, namespace)
		if err != nil {
			t.Fatal("issue sending/receiving:", err)
		}

		for _, rItem := range r {
			// Weak check, just makes sure it's not a default 0.
			if rItem.Payload.Cap == 0 {
				t.Fatal("unexpected cap response:", rItem.Payload.Cap)
			}
		}
	})
}

func TestKNNLatency(t *testing.T) {
	nNodes := 2
	url := func(addr string) string {
		return "http://localhost" + addr + "/info/knnLatency"
	}
	withNetwork(t, nNodes, func(tn *testNetwork) {
		url := url(tn.nodes[0].addrAPI)

		namespace := "test"
		nVecs := 10
		dim := 10
		tn.fill(namespace, nVecs, dim)
		tn.knnFuzz(namespace, 3, dim, time.Millisecond*50)

		opts := knnLatencyArgs{
			Key:    namespace,
			Period: time.Hour,
		}

		r, err := post[[]clientResult[knnLatencyResp]](url, opts)
		if err != nil {
			t.Fatal("issue sending/receiving:", err)
		}

		for _, rItem := range r {
			// Weak check, just makes sure it's not a default 0.
			if rItem.Payload.Query == 0 {
				t.Fatal("unexpected default latency resp")
			}
		}
	})
}

func TestKNNMonitor(t *testing.T) {
	nNodes := 2
	url := func(addr string) string {
		return "http://localhost" + addr + "/info/knnMonitor"
	}
	withNetwork(t, nNodes, func(tn *testNetwork) {
		url := url(tn.nodes[0].addrAPI)

		namespace := "test"
		nVecs := 10
		dim := 10
		tn.fill(namespace, nVecs, dim)
		tn.knnFuzz(namespace, 3, dim, time.Millisecond*50)

		opts := knnMonArgs{
			Start: time.Now(),
			End:   time.Now().Add(-time.Hour),
		}

		r, err := post[[]clientResult[knnMonItemAvg]](url, opts)
		if err != nil {
			t.Fatal("issue sending/receiving:", err)
		}

		for _, rItem := range r {
			// Weak check, just makes sure it's not a default 0.
			if rItem.Payload.N == 0 {
				t.Fatal("unexpected default knnMonItemAvg resp (N field)")
			}
		}
	})
}
