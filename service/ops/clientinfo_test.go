package ops

import (
	"testing"
	"time"
)

/*
--------------------------------------------------------------------------------
File tests info methods for T Client and Server, i.e CInfo and SInfo.

Tests are prefixed with TestSingleX because there is also composite operations.
--------------------------------------------------------------------------------
*/

func TestSingleInfoSSpaceNamespaces(t *testing.T) {
	addr := freeLocalNoFail(t)

	err := withTestNode(addr, func(testNode *testNode) {
		// Before fill.
		r := NewClient(addr).Info().SSpaceNamespaces()
		if r.NetErr != nil {
			t.Fatal(r.NetErr)
		}
		if len(r.Payload) != 0 {
			t.Fatal("unexpected namespace count (want 0):", len(r.Payload))
		}

		// After fill.
		testNode.fill(10)
		r = NewClient(addr).Info().SSpaceNamespaces()
		if r.NetErr != nil {
			t.Fatal(r.NetErr)
		}
		if len(r.Payload) != 1 {
			t.Fatal("unexpected namespace count (want 1):", len(r.Payload))
		}
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestSingleInfoSSpaceNamespace(t *testing.T) {
	addr := freeLocalNoFail(t)

	err := withTestNode(addr, func(testNode *testNode) {
		ns := testNode.rManMeta.namespace
		// Before fill.
		r := NewClient(addr).Info().SSpaceNamespace(ns)
		if r.NetErr != nil {
			t.Fatal(r.NetErr)
		}
		if r.Payload {
			t.Fatal("unexpected namespace found:")
		}

		// After fill.
		testNode.fill(10)
		r = NewClient(addr).Info().SSpaceNamespace(ns)
		if r.NetErr != nil {
			t.Fatal(r.NetErr)
		}
		if !r.Payload {
			t.Fatal("unexpected namespace not-found")
		}
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestSingleInfoSSpaceDim(t *testing.T) {
	addr := freeLocalNoFail(t)

	err := withTestNode(addr, func(testNode *testNode) {
		ns := testNode.rManMeta.namespace
		dim := testNode.rManMeta.poolVecDim

		testNode.fill(10)

		r := NewClient(addr).Info().SSpaceDim(ns)
		if r.NetErr != nil {
			t.Fatal(r.NetErr)
		}
		if !r.Payload.LookupOk {
			t.Fatal("unexpected namespace not-found")
		}
		if r.Payload.Dim != dim {
			t.Fatalf("unexpected neq dim. want %v, got %v", dim, r.Payload.Dim)
		}
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestSingleInfoSSpaceLen(t *testing.T) {
	addr := freeLocalNoFail(t)

	err := withTestNode(addr, func(testNode *testNode) {
		ns := testNode.rManMeta.namespace

		n := 9
		testNode.fill(n)

		r := NewClient(addr).Info().SSpaceLen(ns)
		if r.NetErr != nil {
			t.Fatal(r.NetErr)
		}
		if !r.Payload.LookupOk {
			t.Fatal("unexpected namespace not-found")
		}
		if r.Payload.NVecs != n {
			t.Fatalf("unexpected neq len. want %v, got %v", n, r.Payload.NVecs)
		}
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestSingleInfoSSpaceCap(t *testing.T) {
	addr := freeLocalNoFail(t)

	err := withTestNode(addr, func(testNode *testNode) {
		ns := testNode.rManMeta.namespace
		rManHandle := testNode.server.rManHandle

		testNode.fill(9)
		capacity, _ := rManHandle.Info().SSpaceCap(ns)

		r := NewClient(addr).Info().SSpaceCap(ns)
		if r.NetErr != nil {
			t.Fatal(r.NetErr)
		}
		if !r.Payload.LookupOk {
			t.Fatal("unexpected namespace not-found")
		}
		if r.Payload.Cap != capacity {
			s := "unexpected neq cap. want %v, got %v"
			t.Fatalf(s, capacity, r.Payload.Cap)
		}
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestSingleInfoKNNLatency(t *testing.T) {
	addr := freeLocalNoFail(t)

	err := withTestNode(addr, func(testNode *testNode) {
		ns := testNode.rManMeta.namespace
		rManHandle := testNode.server.rManHandle

		testNode.fill(10_000)
		testNode.makeLatency(10, time.Millisecond*10)

		lQueue, _ := rManHandle.Info().KNNQueueLatency(time.Minute)
		lQuery, _ := rManHandle.Info().KNNQueryLatency(ns, time.Minute)
		expectTotal := lQueue + lQuery

		// Full minute go get the complete range.
		args := KNNLatencyArgs{Key: ns, Period: time.Minute}

		r := NewClient(addr).Info().KNNLatency(args)
		if r.NetErr != nil {
			t.Fatal(r.NetErr)
		}
		if !r.Payload.LookupOk {
			t.Fatal("unexpected namespace not-found")
		}

		gotTotal := r.Payload.Queue + r.Payload.Query
		if expectTotal != gotTotal {
			s := "unexpected neq cap. want %v, got %v"
			t.Fatalf(s, expectTotal, gotTotal)
		}
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestSingleInfoKNNMonitor(t *testing.T) {
	addr := freeLocalNoFail(t)

	err := withTestNode(addr, func(testNode *testNode) {

		testNode.fill(10_000)
		testNode.makeLatency(100, time.Millisecond*10)

		r := NewClient(addr).Info().KNNMonitor(KNNMonArgs{Period: time.Minute})
		if r.NetErr != nil {
			t.Fatal(r.NetErr)
		}
		if r.Payload.N == 0 {
			t.Fatal("unexpected 0 entries monitored")
		}
	})

	if err != nil {
		t.Fatal(err)
	}
}
