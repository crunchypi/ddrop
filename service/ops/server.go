package ops

import (
	"context"
	"net"
	"net/rpc"
	"time"

	"github.com/crunchypi/ddrop/pkg/mathx"
	rman "github.com/crunchypi/ddrop/service/requestman"
)

// Server is an rpc server on top of requestman.Handle.
type Server struct {
	LocalAddr      string
	rManHandle     *rman.Handle
	rManHandleStop func()
}

// NewServer is a factory function. Will return (nil, false) is a new
// requestman.Handle could not be created with the given rManHandleArgs.
func NewServer(localAddr string, rManHandleArgs rman.NewHandleArgs) (*Server, bool) {
	// Guard for chaining ctx.
	if rManHandleArgs.Ctx == nil {
		return nil, false
	}
	ctx, ctxStop := context.WithCancel(rManHandleArgs.Ctx)
	rManHandleArgs.Ctx = ctx

	rManHandle, ok := rman.NewHandle(rManHandleArgs)
	if !ok {
        ctxStop()
		return nil, false
	}

	s := Server{
		LocalAddr:      localAddr,
		rManHandle:     rManHandle,
		rManHandleStop: ctxStop,
	}

	return &s, true
}

// StartListen spins up the server and makes it active. The returned func
// is used for stopping (this also stops the internal requestman.Handle),
// while the error indicates the following (These are for the setup):
// - rpc.NewServer().Register(this) returns an err.
// - net.Listen("tcp", this.LocalAddr) returns an err.
//
// Note, while accepting requests with net.Listener.Accept(), if an err
// is returned, then the listening event-loop simply fails.
func (s *Server) StartListen() (stop func(), err error) {
	handler := rpc.NewServer()
	if err := handler.Register(s); err != nil {
		return nil, err
	}

	// Also register namespaced server info.
	sinfo := SInfo(*s)
	if err := handler.Register(&sinfo); err != nil {
		return nil, err
	}

	ln, err := net.Listen("tcp", s.LocalAddr)
	if err != nil {
		return nil, err
	}

	var conn net.Conn
	stop = func() {
		ln.Close()
		if conn != nil {
			conn.Close()
		}
		if s.rManHandleStop != nil {
			s.rManHandleStop()
		}
	}

	go func() {
		for {
			cxn, err := ln.Accept()
			conn = cxn
			if err != nil {
				break
			}
			go handler.ServeConn(cxn)
		}
	}()
	return stop, nil
}

// SArgs is used as a Server argument wrapper with metadata.
// Go rpc methods are required to have the following signature format:
//  x.Method(args any, resp *any) error
// This is used as the 'args'.
type SArgs[T any] struct {
	SendTime time.Time
	Payload  T
}

// SResp is used as a Server argument wrapper with metadata.
// Go rpc methods are required to have the following signature format:
//  x.Method(args any, resp *any) error
// This is used as the 'resp'.
type SResp[T any] struct {
	RecvTime time.Time
	Payload  T
}

// NewSArgs is a convenience func for setting up a new SArgs[T] with instance
// with the SendTime field set to time.Now().
func NewSArgs[T any](payload T) SArgs[T] {
	return SArgs[T]{
		SendTime: time.Now(),
		Payload:  payload,
	}
}

// Ping simply sets resp.RecvTime to now and resp.Payload to true (on success).
func (s *Server) Ping(args SArgs[bool], resp *SResp[bool]) error {
	resp.RecvTime = time.Now()
	resp.Payload = true
	return nil
}

// AddData attempts to add the given data to the internal requestman.Handle with
// the AddData() method. The returns of those AddData() calls are stored index
// for index in the response.
func (s *Server) AddData(args SArgs[[]AddDataArgs], resp *SResp[[]bool]) error {
	resp.RecvTime = time.Now()

	if args.Payload == nil {
		return nil
	}

	// Make sure the resp.Payload slice is of matching length as args.Payload
	// because of the way bools are stored (by index) in the loop below.
	if resp.Payload == nil || len(resp.Payload) <= len(args.Payload) {
		resp.Payload = make([]bool, len(args.Payload))
	}

	// Try add.
	for i, addDataArgs := range args.Payload {
		resp.Payload[i] = s.rManHandle.AddData(
			addDataArgs.Namespace,
			rman.DistancerContainer{
				D:       mathx.NewSafeVec(addDataArgs.Vec...),
				Expires: addDataArgs.Expires,
			},
			addDataArgs.Data,
		)
	}

	return nil
}

// KNNEager attempts to do a KNN request using the KNN method of the internal
// requestmanager.Handle. It does so eagerly, so will wait until the KNN request
// is complete.
//
// Note that network latency is factored in with args.Payload.TTL
func (s *Server) KNNEager(args SArgs[rman.KNNArgs], resp *SResp[KNNResp]) error {
	resp.RecvTime = time.Now()

	// Factor network latency into TTL.
	args.Payload.TTL -= resp.RecvTime.Sub(args.SendTime)
	if args.Payload.TTL <= 0 {
		return nil
	}

	// Do request.
	enqueueResult, ok := s.rManHandle.KNN(args.Payload)
	if !ok {
		return nil
	}

	// Await result.
	select {
	case <-time.After(args.Payload.TTL + time.Microsecond):
		enqueueResult.Cancel.Cancel()
	case result := <-enqueueResult.Pipe:
		(*resp).Payload.KNN = KNNRespItemsFromScoreItems(result)
		(*resp).Payload.Ok = true
	}

	return nil
}
