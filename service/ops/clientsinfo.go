package ops

import (
	rman "github.com/crunchypi/ddrop/service/requestman"
)

// CSInfo is for namespacing info methods of type Clients.
type CSInfo Clients

func (csi *CSInfo) clients() *Clients {
	cs := Clients(*csi)
	return &cs
}

// SSpaceNamespaces does a composite call to Client.Info().SSpaceNamespaces(),
// using all internal addrs. See docs for that method for more details.
func (csi *CSInfo) SSpaceNamespaces() ClientResults[[]string] {
	// Nested return type.
	type T = []string

	// Request/task func per client/address.
	rf := func(c *Client) *ClientResult[T] {
		return c.Info().SSpaceNamespaces()
	}

	// Concurrent requests.
	return fanInRequests(fanInRequestsArgs[T]{
		addrs:       csi.RemoteAddrs,
		ttl:         csi.Timeout,
		requestFunc: rf,
	})
}

// SSpaceNamespace does a composite call to Client.Info().SSpaceNamespace(),
// using all internal addrs. See docs for that method for more details.
func (csi *CSInfo) SSpaceNamespace(key string) ClientResults[bool] {
	// Nested return type.
	type T = bool

	// Request/task func per client/address.
	rf := func(c *Client) *ClientResult[T] {
		return c.Info().SSpaceNamespace(key)
	}

	// Concurrent requests.
	return fanInRequests(fanInRequestsArgs[T]{
		addrs:       csi.RemoteAddrs,
		ttl:         csi.Timeout,
		requestFunc: rf,
	})
}

// SSpaceDim does a composite call to Client.Info().SSpaceDim(),
// using all internal addrs. See docs for that method for more details.
func (csi *CSInfo) SSpaceDim(key string) ClientResults[SSpaceDimResp] {
	// Nested return type.
	type T = SSpaceDimResp

	// Request/task func per client/address.
	rf := func(c *Client) *ClientResult[T] {
		return c.Info().SSpaceDim(key)
	}

	// Concurrent requests.
	return fanInRequests(fanInRequestsArgs[T]{
		addrs:       csi.RemoteAddrs,
		ttl:         csi.Timeout,
		requestFunc: rf,
	})
}

// SSpaceLen does a composite call to Client.Info().SSpaceLen(),
// using all internal addrs. See docs for that method for more details.
func (csi *CSInfo) SSpaceLen(key string) ClientResults[SSpaceLenResp] {
	// Nested return type.
	type T = SSpaceLenResp

	// Request/task func per client/address.
	rf := func(c *Client) *ClientResult[T] {
		return c.Info().SSpaceLen(key)
	}

	// Concurrent requests.
	return fanInRequests(fanInRequestsArgs[T]{
		addrs:       csi.RemoteAddrs,
		ttl:         csi.Timeout,
		requestFunc: rf,
	})
}

// SSpaceCap does a composite call to Client.Info().SSpaceCap(),
// using all internal addrs. See docs for that method for more details.
func (csi *CSInfo) SSpaceCap(key string) ClientResults[SSpaceCapResp] {
	// Nested return type.
	type T = SSpaceCapResp

	// Request/task func per client/address.
	rf := func(c *Client) *ClientResult[T] {
		return c.Info().SSpaceCap(key)
	}

	// Concurrent requests.
	return fanInRequests(fanInRequestsArgs[T]{
		addrs:       csi.RemoteAddrs,
		ttl:         csi.Timeout,
		requestFunc: rf,
	})
}

// KNNLatency does a composite call to Client.Info().KNNLatency(),
// using all internal addrs. See docs for that method for more details.
func (csi *CSInfo) KNNLatency(args KNNLatencyArgs) ClientResults[KNNLatencyResp] {
	// Nested return type.
	type T = KNNLatencyResp

	// Request/task func per client/address.
	rf := func(c *Client) *ClientResult[T] {
		return c.Info().KNNLatency(args)
	}

	// Concurrent requests.
	return fanInRequests(fanInRequestsArgs[T]{
		addrs:       csi.RemoteAddrs,
		ttl:         csi.Timeout,
		requestFunc: rf,
	})
}

// KNNMonitor does a composite call to Client.Info().KNNMonitor(),
// using all internal addrs. See docs for that method for more details.
func (csi *CSInfo) KNNMonitor(args KNNMonArgs) ClientResults[rman.KNNMonItemAvg] {
	// Nested return type.
	type T = rman.KNNMonItemAvg

	// Request/task func per client/address.
	rf := func(c *Client) *ClientResult[T] {
		return c.Info().KNNMonitor(args)
	}

	// Concurrent requests.
	return fanInRequests(fanInRequestsArgs[T]{
		addrs:       csi.RemoteAddrs,
		ttl:         csi.Timeout,
		requestFunc: rf,
	})
}
