package ops

import (
	"net"
	"net/rpc"
	"time"

	rman "github.com/crunchypi/ddrop/service/requestman"
)

// Client is a client for the rpc network server defined in this pkg.
// It is used to interface remote requestman.Handle.
type Client struct {
	RemoteAddr string
	// Timeout specifies connection timeout.
	Timeout time.Duration
}

// NewClient sets up a new client. If a timeout isn't specified, or has a
// time.Duration <= 0, then the timeout will be set to a second as default.
func NewClient(remoteAddr string, timeout ...time.Duration) *Client {
	if len(timeout) == 0 || timeout[0] <= time.Duration(0) {
		return &Client{RemoteAddr: remoteAddr, Timeout: time.Second}
	}
	return &Client{RemoteAddr: remoteAddr, Timeout: timeout[0]}
}

// ClientResult is a wrapper around any result returned from a client -> server
// call. It contains additional meta info, specifically the address used, err,
// as well as network latency.
type ClientResult[T any] struct {
	RemoteAddr string
	NetErr     error
	Payload    T

	NetworkLatency time.Duration
}

// callArgs is used as arguments for Client.call. It mirrors rpc.Client.Call(...).
type callArgs struct {
	rpcServiceMethod string
	rpcArgs          any
	rpcResp          any
}

// call is a convenience remote-call method. It handles rpc.Client setup,
// timeout, and resource release.
func (c *Client) call(args callArgs) error {
	conn, err := net.DialTimeout("tcp", c.RemoteAddr, c.Timeout)
	if err != nil {
		return err
	}

	defer conn.Close()

	client := rpc.NewClient(conn)
	defer client.Close()
	return client.Call(args.rpcServiceMethod, args.rpcArgs, args.rpcResp)
}

// Ping pings the remote server. The returned ClientResult.Payload will be true
// if the server is alive.
func (c *Client) Ping() *ClientResult[bool] {
	// Nested return type.
	type T = bool

	// Request.
	send := NewSArgs(false)
	resp := SResp[T]{}
	nErr := c.call(callArgs{"Server.Ping", send, &resp})

	return &ClientResult[T]{
		RemoteAddr:     c.RemoteAddr,
		NetErr:         nErr,
		Payload:        resp.Payload,
		NetworkLatency: resp.RecvTime.Sub(send.SendTime),
	}
}

// AddDataArgs is intended as args for Client.AddData.
type AddDataArgs struct {
	Namespace string
	Vec       []float64
	Data      []byte
	Expires   time.Time
}

// AddData tries to add data to the remote server.
// The remote server uses requestmanager.Handle.AddData(...), see
// the docs for more details about args, returns, etc.
func (c *Client) AddData(args []AddDataArgs) *ClientResult[[]bool] {
	// Nested return type.
	type T = []bool

	// Request.
	send := NewSArgs[[]AddDataArgs](args)
	resp := SResp[T]{}
	nErr := c.call(callArgs{"Server.AddData", send, &resp})

	return &ClientResult[T]{
		RemoteAddr:     c.RemoteAddr,
		NetErr:         nErr,
		Payload:        resp.Payload,
		NetworkLatency: resp.RecvTime.Sub(send.SendTime),
	}
}

// KNNRespItem is intended as a single item in KNNResp.
type KNNRespItem struct {
	Vec   []float64
	Score float64
}

// KNNResp is intended as the response of Client.KNNEager.
type KNNResp struct {
	KNN []KNNRespItem
	// Ok is generally matched with the bool returned from
	// requestman.Handle.KNN. But it is also false if the
	// requestman.KNNArgs.TTL is less than network latency.
	Ok bool
}

// KNNEager tries to (eagerly) do a KNN lookup on a remote server.
// The remote server uses requestmanager.Handle.KNN(...), see
// the docs for more details about args, returns, etc.
//
// Note; network latency is factored in with args.TTL.
//
// Note; eagers means that it calls the server, which waits for the entire
// knn request before returning any results.
func (c *Client) KNNEager(args rman.KNNArgs) *ClientResult[KNNResp] {
	// Nested return type.
	type T = KNNResp

	// Request.
	send := NewSArgs(args)
	resp := SResp[T]{}
	nErr := c.call(callArgs{"Server.KNNEager", send, &resp})

	return &ClientResult[T]{
		RemoteAddr:     c.RemoteAddr,
		NetErr:         nErr,
		Payload:        resp.Payload,
		NetworkLatency: resp.RecvTime.Sub(send.SendTime),
	}
}

// Info returns a method namespace. Similar to requestman.Handle.Info()
func (c *Client) Info() *CInfo {
	ci := CInfo(*c)
	return &ci
}
