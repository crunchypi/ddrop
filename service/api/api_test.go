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
