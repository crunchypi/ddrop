package ops

import (
	"sync"
	"testing"
	"time"
)

func TestCompositeSSpaceNamespaces(t *testing.T) {
	n := 3
	err := withNetwork(t, n, func(tn *testNetwork) {
		// Create some data so there is a namespace.
		for _, node := range tn.nodes {
			node.fill(10)
		}

		ns := tn.nodes[tn.addrs[0]].rManMeta.namespace
		ch := NewClients(tn.addrs).Info().SSpaceNamespaces()

		ch, nResults := countChan(ch)
		if nResults != n {
			t.Fatal("got an unexpected amt of results:", nResults)
		}

		for clientResult := range ch {
			if err := clientResult.NetErr; err != nil {
				t.Fatal("one node got a network err:", err)
			}

			if l := len(clientResult.Payload); l != 1 {
				t.Fatal("one node got an unexpected amt of namespaces:", l)
			}

			if nsRes := clientResult.Payload[0]; nsRes != ns {
				t.Fatal("one node got an unexpected namespace:", nsRes)
			}
		}
	})

	if err != nil {
		t.Fatal("could not setup a test network:", err)
	}
}

func TestCompositeSSpaceNamespace(t *testing.T) {
	n := 3
	err := withNetwork(t, n, func(tn *testNetwork) {
		// Create some data so there is a namespace.
		for _, node := range tn.nodes {
			node.fill(10)
		}

		ns := tn.nodes[tn.addrs[0]].rManMeta.namespace
		ch := NewClients(tn.addrs).Info().SSpaceNamespace(ns)

		nResults := 0
		for clientResult := range ch {
			nResults++

			if err := clientResult.NetErr; err != nil {
				t.Fatal("one node got a network err:", err)
			}

			if !clientResult.Payload {
				t.Fatal("one node got a not-ok namespace lookup")
			}
		}

		if nResults != n {
			t.Fatal("got an unexpected amt of results, nResults")
		}
	})

	if err != nil {
		t.Fatal("could not setup a test network:", err)
	}
}

func TestCompositeSSpaceDim(t *testing.T) {
	n := 3
	err := withNetwork(t, n, func(tn *testNetwork) {
		// Create some data so there is a namespace.
		for _, node := range tn.nodes {
			node.fill(10)
		}

		// Any node to get namespace and dim.
		node := tn.nodes[tn.addrs[0]]
		dim := node.rManMeta.poolVecDim
		ns := node.rManMeta.namespace
		ch := NewClients(tn.addrs).Info().SSpaceDim(ns)

		nResults := 0
		for clientResult := range ch {
			nResults++

			if err := clientResult.NetErr; err != nil {
				t.Fatal("one node got a network err:", err)
			}

			if !clientResult.Payload.LookupOk {
				t.Fatal("one node got a not-ok namespace lookup")
			}

			if dimRes := clientResult.Payload.Dim; dimRes != dim {
				t.Fatal("got unexpected dim result:", dimRes)
			}
		}

		if nResults != n {
			t.Fatal("got an unexpected amt of results, nResults")
		}
	})

	if err != nil {
		t.Fatal("could not setup a test network:", err)
	}
}

func TestCompositeSSpaceLen(t *testing.T) {
	n := 3
	err := withNetwork(t, n, func(tn *testNetwork) {
		// Create some data so there is a namespace and len.
		l := 10
		for _, node := range tn.nodes {
			node.fill(l)
		}

		// Any node to get namespace.
		ns := tn.nodes[tn.addrs[0]].rManMeta.namespace
		ch := NewClients(tn.addrs).Info().SSpaceLen(ns)

		nResults := 0
		for clientResult := range ch {
			nResults++

			if err := clientResult.NetErr; err != nil {
				t.Fatal("one node got a network err:", err)
			}

			if !clientResult.Payload.LookupOk {
				t.Fatal("one node got a not-ok namespace lookup")
			}

			if lenRes := clientResult.Payload.NVecs; lenRes != l {
				t.Fatal("got unexpected len result:", lenRes)
			}
		}

		if nResults != n {
			t.Fatal("got an unexpected amt of results, nResults")
		}
	})

	if err != nil {
		t.Fatal("could not setup a test network:", err)
	}
}

func TestCompositeSSpaceCap(t *testing.T) {
	n := 3
	err := withNetwork(t, n, func(tn *testNetwork) {
		for _, node := range tn.nodes {
			node.fill(1)
		}

		// Any node to get namespace and cap.
		node := tn.nodes[tn.addrs[0]]
		ns := node.rManMeta.namespace
		expectCap := node.rManMeta.newSearchSpaceArgs.SearchSpacesMaxCap
		ch := NewClients(tn.addrs).Info().SSpaceCap(ns)

		nResults := 0
		for clientResult := range ch {
			nResults++

			if err := clientResult.NetErr; err != nil {
				t.Fatal("one node got a network err:", err)
			}

			if !clientResult.Payload.LookupOk {
				t.Fatal("one node got a not-ok namespace lookup")
			}

			if capRes := clientResult.Payload.Cap; capRes != expectCap {
				t.Fatal("got unexpected len result:", capRes)
			}
		}

		if nResults != n {
			t.Fatal("got an unexpected amt of results, nResults")
		}
	})

	if err != nil {
		t.Fatal("could not setup a test network:", err)
	}
}

func TestCompositeKNNLatency(t *testing.T) {
	n := 3
	err := withNetwork(t, n, func(tn *testNetwork) {

		// Add data and make some latency.
		wg := sync.WaitGroup{}
		wg.Add(n)
		for _, node := range tn.nodes {
			go func(node *testNode) {
				defer wg.Done()
				node.fill(1000)
				node.makeLatency(10, time.Millisecond*10)
			}(node)
		}
		wg.Wait()

		// Any node to get namespace and cap.
		node := tn.nodes[tn.addrs[0]]
		ns := node.rManMeta.namespace
		ch := NewClients(tn.addrs).Info().KNNLatency(KNNLatencyArgs{
			Key:    ns,
			Period: time.Minute,
		})

		// Check amt. for results.
		ch, nResults := countChan(ch)
		if nResults != n {
			t.Fatal("unexpected amt. for results:", nResults)
		}

		// Check details.
		for clientResult := range ch {
			if err := clientResult.NetErr; err != nil {
				t.Fatal("one node got a network err:", err)
			}

			if !clientResult.Payload.LookupOk {
				t.Fatal("one node got a not-ok namespace lookup")
			}

			if clientResult.Payload.Queue == 0 || clientResult.Payload.Query == 0 {
				t.Fatal("one node had 0 latency for either queue or query")
			}
		}
	})

	if err != nil {
		t.Fatal("could not setup a test network:", err)
	}

}

func TestCompositeKNNMonitor(t *testing.T) {
	n := 3
	err := withNetwork(t, n, func(tn *testNetwork) {

		// Add data and make some latency.
		wg := sync.WaitGroup{}
		wg.Add(n)
		for _, node := range tn.nodes {
			go func(node *testNode) {
				defer wg.Done()
				node.fill(1000)
				node.makeLatency(10, time.Millisecond*10)
			}(node)
		}
		wg.Wait()

		ch := NewClients(tn.addrs).Info().KNNMonitor(KNNMonArgs{
			Start: time.Now(),
			End:   time.Now().Add(-time.Minute),
		})

		// Check amt. for results.
		ch, nResults := countChan(ch)
		if nResults != n {
			t.Fatal("unexpected amt. for results:", nResults)
		}

		// Check details.
		for clientResult := range ch {
			if err := clientResult.NetErr; err != nil {
				t.Fatal("one node got a network err:", err)
			}

			if clientResult.Payload.N == 0 {
				t.Fatal("knn monitor had n=0")
			}
		}
	})

	if err != nil {
		t.Fatal("could not setup a test network:", err)
	}

}
