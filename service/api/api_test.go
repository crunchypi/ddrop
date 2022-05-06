package api

import (
	"context"
	"testing"
	"time"

	"github.com/crunchypi/ddrop/service/ops"
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
