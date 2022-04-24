package ops

import (
	"time"

	rman "github.com/crunchypi/ddrop/service/requestman"
)

// CInfo is a namespace on top of Client, which contains methods related to
// metadata, similar to requestman.Handle.Info().
type CInfo Client

// client converts CInfo
func (ci *CInfo) client() *Client {
	c := Client(*ci)
	return &c
}

// SSpaceNamespaces tries to get namespaces from the remote server.
//
// The remote server forwards the call to the method with the same name on top
// of its internal requestmanager.Handle.Info(). See the docs for that path
// for more details about args, returns, etc.
func (ci *CInfo) SSpaceNamespaces() *ClientResult[[]string] {
	// Nested return type.
	type T = []string

	// Request.
	send := NewSArgs(false)
	resp := SResp[T]{}
	nErr := ci.client().call(callArgs{"SInfo.SSpaceNamespaces", send, &resp})

	return &ClientResult[T]{
		RemoteAddr:     ci.RemoteAddr,
		NetErr:         nErr,
		Payload:        resp.Payload,
		NetworkLatency: resp.RecvTime.Sub(send.SendTime),
	}
}

// SSpaceNamespace checks if the given namespace/key exists on the remote server.
//
// The remote server forwards the call to the method with the same name on top
// of its internal requestmanager.Handle.Info(). See the docs for that path
// for more details about args, returns, etc.
func (ci *CInfo) SSpaceNamespace(key string) *ClientResult[bool] {
	// Nested return type.
	type T = bool

	// Request.
	send := NewSArgs(key)
	resp := SResp[T]{}
	nErr := ci.client().call(callArgs{"SInfo.SSpaceNamespace", send, &resp})

	return &ClientResult[T]{
		RemoteAddr:     ci.RemoteAddr,
		NetErr:         nErr,
		Payload:        resp.Payload,
		NetworkLatency: resp.RecvTime.Sub(send.SendTime),
	}
}

// SSpaceDimResp is intended as a response from CInfo.SSpaceDim.
type SSpaceDimResp struct {
	LookupOk bool // LookupOk indicates if the namespace/key was valid.
	Dim      int  // Uniform vector dimension.
}

// SSpaceDim tries to get the uniform dimension for vectors on the search space
// with the given key/namespace from the remote server.
//
// The remote server forwards the call to the method with the same name on top
// of its internal requestmanager.Handle.Info(). See the docs for that path
// for more details about args, returns, etc.
func (ci *CInfo) SSpaceDim(key string) *ClientResult[SSpaceDimResp] {
	// Nested return type.
	type T = SSpaceDimResp

	// Request.
	send := NewSArgs(key)
	resp := SResp[T]{}
	nErr := ci.client().call(callArgs{"SInfo.SSpaceDim", send, &resp})

	return &ClientResult[T]{
		RemoteAddr:     ci.RemoteAddr,
		NetErr:         nErr,
		Payload:        resp.Payload,
		NetworkLatency: resp.RecvTime.Sub(send.SendTime),
	}
}

// SSPaceLenResp is intended as a response from CInfo.SSpaceLen.
type SSpaceLenResp struct {
	LookupOk bool // LookupOk indicates if the namespace/key was valid.
	NSSpaces int  // NSSpaces specifies a number of search spaces.
	NVecs    int  // NVecs specifies the total number of vectors.
}

// SSpaceLen tries to get the amount of search spaces (and the sum of all their
// vectors) for a given key/namespace from the remote server.
//
// The remote server forwards the call to the method with the same name on top
// of its internal requestmanager.Handle.Info(). See the docs for that path
// for more details about args, returns, etc.
func (ci *CInfo) SSpaceLen(key string) *ClientResult[SSpaceLenResp] {
	// Nested return type.
	type T = SSpaceLenResp

	// Request.
	send := NewSArgs(key)
	resp := SResp[T]{}
	nErr := ci.client().call(callArgs{"SInfo.SSpaceLen", send, &resp})

	return &ClientResult[T]{
		RemoteAddr:     ci.RemoteAddr,
		NetErr:         nErr,
		Payload:        resp.Payload,
		NetworkLatency: resp.RecvTime.Sub(send.SendTime),
	}
}

// SSpaceCapresp is intended as a response from CInfo.SSpaceCap.
type SSpaceCapResp struct {
	LookupOk bool // LookupOk indicates if the namespace/key was valid.
	Cap      int  // Cap specifies how many search spaces can exist.
}

// SSpaceCap tries to get the search space (not total vector) capacity for a
// given key/namespace from the remote server.
//
// The remote server forwards the call to the method with the same name on top
// of its internal requestmanager.Handle.Info(). See the docs for that path
// for more details about args, returns, etc.
func (ci *CInfo) SSpaceCap(key string) *ClientResult[SSpaceCapResp] {
	// Nested return type.
	type T = SSpaceCapResp

	// Request.
	send := NewSArgs(key)
	resp := SResp[T]{}
	nErr := ci.client().call(callArgs{"SInfo.SSpaceCap", send, &resp})

	return &ClientResult[T]{
		RemoteAddr:     ci.RemoteAddr,
		NetErr:         nErr,
		Payload:        resp.Payload,
		NetworkLatency: resp.RecvTime.Sub(send.SendTime),
	}
}

// KNNLatencyArgs is intended for CInfo.KNNLatency.
type KNNLatencyArgs struct {
	Key    string        // Key specifies the namespace to use.
	Period time.Duration // Period specifies what period (since now) to check.
}

type KNNLatencyResp struct {
	LookupOk bool          // LookupOk indicates if the namespace/key was valid.
	Queue    time.Duration // Queue is for knn queue latency.
	Query    time.Duration // Query is for knn query latency.
	BoundsOk bool          // BoundsOk is false if the period used was too large.
}

// KNNLatency tries to get some metadata related to KNN latency on the remote
// server, specifically queue and query durations.
//
// The remote server forwards the call to these methods:
// - requestman.Handle.Info().KNNQueueLatency(...)
// - requestman.Handle.Info().KNNQueryLatency(...)
// See docs for those methods for more details about args, returns, etc.
func (ci *CInfo) KNNLatency(args KNNLatencyArgs) *ClientResult[KNNLatencyResp] {
	// Nested return type.
	type T = KNNLatencyResp

	// Request.
	send := NewSArgs(args)
	resp := SResp[T]{}
	nErr := ci.client().call(callArgs{"SInfo.KNNLatency", send, &resp})

	return &ClientResult[T]{
		RemoteAddr:     ci.RemoteAddr,
		NetErr:         nErr,
		Payload:        resp.Payload,
		NetworkLatency: resp.RecvTime.Sub(send.SendTime),
	}
}

// KNNMonArgs is intended for CInfo.KNNMonitor
//
// Start (and End) times are used as bounds for what monitoring data to
// include. Do note that "end" represents how far back in time to go, while
// "start" represents the offset. So to get records for the last minute:
// - Start: time.Now()
// - End  : time.Now().Add(-time.Minute)
type KNNMonArgs struct {
	// Start of record. 
	Start time.Time
	// End of record, how far to go back in time relative to "Start".
	End time.Time
}

// KNNMonitor tries to get monitoring data related to KNN queries from the remote
// server.
//
// The remote server forwards the call to the method with the same name on top
// of its internal requestmanager.Handle.Info(). See the docs for that path
// for more details about args, returns, etc.
func (ci *CInfo) KNNMonitor(args KNNMonArgs) *ClientResult[rman.KNNMonItemAvg] {
	// Nested return type.
	type T = rman.KNNMonItemAvg

	// Request.
	send := NewSArgs(args)
	resp := SResp[T]{}
	nErr := ci.client().call(callArgs{"SInfo.KNNMonitor", send, &resp})

	return &ClientResult[T]{
		RemoteAddr:     ci.RemoteAddr,
		NetErr:         nErr,
		Payload:        resp.Payload,
		NetworkLatency: resp.RecvTime.Sub(send.SendTime),
	}
}
