package ops

import (
	"testing"
	"time"
)

/*
--------------------------------------------------------------------------------
File tests non-info methods for T Client and Server.

Tests are prefixed with TestSingleX because there is also composite operations.
--------------------------------------------------------------------------------
*/

func TestSinglePing(t *testing.T) {
	addr := freeLocalNoFail(t)

	err := withTestNode(addr, func(_ *testNode) {
		r := NewClient(addr).Ping()
		if r.NetErr != nil {
			t.Fatal(r)
		}
		if !r.Payload {
			t.Fatal("got unexpected not-ok")
		}
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestSingleAddData(t *testing.T) {
	addr := freeLocalNoFail(t)

	err := withTestNode(addr, func(testNode *testNode) {
		// Abbreviations for convenience.
		namespace := testNode.rManMeta.namespace
		dim := testNode.rManMeta.poolVecDim
		rm := testNode.server.rManHandle

		vec, _ := randFloat64Slice(dim)
		payload := []AddDataArgs{
			{Namespace: namespace, Vec: vec, Data: []byte{}},
		}

		r := NewClient(addr).AddData(payload)
		if r.NetErr != nil {
			t.Fatal(r)
		}
		if len(r.Payload) != 1 {
			t.Fatal("unexpected len of", len(r.Payload))
		}
		if !r.Payload[0] {
			t.Fatal("got unexpected not-ok")
		}
		_, l, _ := rm.Info().SSpaceLen(namespace)
		if l != 1 {
			t.Fatal("unexpected search space len after add:", l)
		}
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestSingleKNNEager(t *testing.T) {
	addr := freeLocalNoFail(t)

	err := withTestNode(addr, func(testNode *testNode) {
		// Need some data to query.
		testNode.fill(10_000)

		args := testNode.rManMeta.randKNNArgs()
		args.K++             // At least one.
		args.TTL = time.Hour // Mitigate timeout.

		r := NewClient(addr).KNNEager(args)
		if r.NetErr != nil {
			t.Fatal(r.NetErr)
		}
		if !r.Payload.Ok {
			t.Fatal("not-ok result from client")
		}
		// Accuracy is not the responsebility of this pkg, so only checking len.
		if len(r.Payload.KNN) == 0 {
			t.Fatal("unexpected 0 len of result")
		}
	})

	if err != nil {
		t.Fatal(err)
	}
}
