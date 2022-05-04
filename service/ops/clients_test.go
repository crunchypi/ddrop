package ops

import (
	"testing"
	"time"

	rman "github.com/crunchypi/ddrop/service/requestman"
)

/*
--------------------------------------------------------------------------------
File tests non-info methods for T Clients (on top of Client and Server)

Tests are prefixed with TestCompositeX because there is also 'single' operations,
as in client_test.go and clientinfo_test.go
--------------------------------------------------------------------------------
*/

func TestCompositePing(t *testing.T) {
	n := 3

	err := withNetwork(t, n, func(tn *testNetwork) {
		ch := NewClients(tn.addrs, time.Second).Ping()

		ch, nResps := countChan(ch)
		if nResps != n {
			t.Fatal("unexpected amt of responses:", nResps)
		}

		// Make sure that all were ok.
		for clientResult := range ch {
			if err := clientResult.NetErr; err != nil {
				t.Fatal("one node got a network err:", err)
			}
			if !clientResult.Payload {
				t.Fatal("one node got a not-ok result")
			}
		}
	})

	if err != nil {
		t.Fatal("could not setup a test network:", err)
	}
}

func TestCompositeAddData(t *testing.T) {
	n := 3

	err := withNetwork(t, n, func(tn *testNetwork) {
		// Use any node to get a valid namespace and dim.
		node := tn.nodes[tn.addrs[0]]
		ns := node.rManMeta.namespace
		dim := node.rManMeta.poolVecDim

		vec, _ := randFloat64Slice(dim)
		payload := []AddDataArgs{
			{Namespace: ns, Vec: vec, Data: []byte{}},
		}
		ch := NewClients(tn.addrs, time.Minute).AddData(payload)

		// The Clients.AddData method should place data at _one_ server.
		ch, nResps := countChan(ch)
		if nResps != 1 {
			t.Fatal("unexpected amt of responses:", nResps)
		}

		// This is to get that address in order to check the relevant
		// requestman.Handle instance for correct state (new data).
		recieveNodeAddr := ""
		for clientResult := range ch {
			if err := clientResult.NetErr; err != nil {
				t.Fatal("one node got a network err:", err)
			}
			if len(clientResult.Payload) == 0 {
				t.Fatal("unexpected empty result")
			}
			if !clientResult.Payload[0] {
				t.Fatal("one node got a not-ok result")
			}
			// The x.AddData method for the composite type 'Clients'
			// adds to only one node, picked at random. So this chan
			// should have only one result.
			if recieveNodeAddr != "" {
				t.Fatal("got more than one result")
			}
			recieveNodeAddr = clientResult.RemoteAddr
		}

		// Validate that the new data was actually placed.
		node, ok := tn.nodes[recieveNodeAddr]
		if !ok {
			s := "could not access nodes in test network with key %v."
			t.Fatalf(s, recieveNodeAddr)
		}
		_, l, _ := node.server.rManHandle.Info().SSpaceLen(ns)
		if l != 1 {
			t.Fatalf("unexpected vecpool len after data add. want 1, have %v", l)
		}
	})

	if err != nil {
		t.Fatal("could not setup a test network:", err)
	}
}

func TestCompositeKNNEagerx(t *testing.T) {
	err := withNetwork(t, 5, func(tn *testNetwork) {
		for _, node := range tn.nodes {
			node.fill(1000)
		}
		// Use any node to get a valid namespace and dim.
		node := tn.nodes[tn.addrs[0]]
		ns := node.rManMeta.namespace
		dim := node.rManMeta.poolVecDim

		// Easy/fast spec knn args.
		v, _ := randFloat64Slice(dim)
		k := 3
		args := rman.KNNArgs{
			Namespace: ns,
			Priority:  1,
			QueryVec:  v,
			KNNMethod: rman.KNNMethodCosineSimilarity,
			Ascending: false,
			K:         k,
			Extent:    0.5,
			Accept:    0.1,
			Reject:    0,
			TTL:       time.Minute,
		}

		r := NewClients(tn.addrs, args.TTL).KNNEagerx(args)
		if len(r) != k {
			t.Fatal("unexpected result len:", len(r))
		}
	})

	if err != nil {
		t.Fatal("could not setup a test network:", err)
	}
}
