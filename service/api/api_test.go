package api

import (
	"context"
	"testing"
	"time"
)

func TestPing(t *testing.T) {
	addr := freeLocalNoFail(t)
	ctx, ctxStop := context.WithCancel(context.Background())

	ok, err := StartServer(StartServerArgs{
		Addr:         addr,
		Ctx:          ctx,
		ReadTimeout:  time.Minute,
		WriteTimeout: time.Minute,
		onRunning: func(h *handle) {
			defer ctxStop()

			url := "http://localhost" + addr + "/ping"
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
	ctx, ctxStop := context.WithCancel(context.Background())

	ok, err := StartServer(StartServerArgs{
		Addr:         addr,
		Ctx:          ctx,
		ReadTimeout:  time.Minute,
		WriteTimeout: time.Minute,
		onRunning: func(h *handle) {
			defer ctxStop()
			newAddr := "test"

			url := "http://localhost" + addr + "/ops/addrs/put"
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
	ctx, ctxStop := context.WithCancel(context.Background())

	ok, err := StartServer(StartServerArgs{
		Addr:         addr,
		Ctx:          ctx,
		ReadTimeout:  time.Minute,
		WriteTimeout: time.Minute,
		onRunning: func(h *handle) {
			defer ctxStop()
			newAddr := "test"
			h.addrSet.addrsMaintanedLocked(newAddr)

			url := "http://localhost" + addr + "/ops/addrs/get"
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
