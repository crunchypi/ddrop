package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/crunchypi/ddrop/service/ops"
)

// addrSet is a set of addrs that is used with rpc operations (pkg /service/ops).
// It will be used to add/rm addrs but also to check if any are stale, see the
// addrSet.maintain method.
type addrSet struct {
	mx     sync.Mutex
	_addrs map[string]bool

	// updateFrequency is how often the "maintain" method actually maintains
	// the addrs in the "addrs" set.
	updateFrequency time.Duration
	updateTimeStamp time.Time
}

// addrs adds the slice of newAddrs into the internal set, then returns all the
// addrs currently in the set. So it is used both as a putter and getter.
// Note that this is not mutex protected.
func (s *addrSet) addrs(newAddrs ...string) []string {
	for _, addr := range newAddrs {
		s._addrs[addr] = true
	}

	r := make([]string, 0, len(s._addrs))
	for addr := range s._addrs {
		r = append(r, addr)
	}

	return r
}

// maintain tries to maintain the internal set of addrs by pinging them with
// ops.Clients(addrSet.addrs()).Ping() -- those nodes that yield a negative
// resppnse are removed from the internal set of addrs. This action does not
// occur more often than addrSet.updateFrequency.
// Note that this method is not mutex protected.
func (s *addrSet) maintain() {
	if time.Now().Sub(s.updateTimeStamp) < s.updateFrequency {
		return
	}
	s.updateTimeStamp = time.Now()

	for clientResp := range ops.NewClients(s.addrs()).Ping() {
		if !clientResp.Payload {
			delete(s._addrs, clientResp.RemoteAddr)
			continue
		}
		s._addrs[clientResp.RemoteAddr] = true
	}
}

// addrsMaintanedLocked does addrSet.addrs(newAddrs...) and addrSet.maintain()
// in a mutex protected way.
func (s *addrSet) addrsMaintanedLocked(newAddrs ...string) []string {
	s.mx.Lock()
	defer s.mx.Unlock()
	s.maintain()
	return s.addrs(newAddrs...)
}

// status is a way of making a http response with a body containing some status.
type status struct {
	Code int    `json:"statusCode"`
	Msg  string `json:"statusMsg"`
}

// rpcServerState indicates the state of an rpc server (pkg ops). Intended to
// be used with T rpcServerWrap below.
type rpcServerState int

const (
	// rpcServerStateDefault = not set / never initialized.
	rpcServerStateDefault rpcServerState = iota
	// rpcServerStateStarting = in the process of being started.
	rpcServerStateStarting
	// rpcServerStateStarted = successfully started / running
	rpcServerStateStarted
	// rpcServerStateStopping = in the process of being stopped.
	rpcServerStateStopping
	// rpcServerStateStopped = successfully stopped.
	rpcServerStateStopped
)

// toStatus converts a rpcServerState into a "status" instance where the
// Msg field has human friendly text.
func (s *rpcServerState) toStatus() status {
	code := int(*s)
	prefix := "rpc server state: "
	switch *s {
	case rpcServerStateDefault:
		return status{code, prefix + "not set"}
	case rpcServerStateStarting:
		return status{code, prefix + "starting"}
	case rpcServerStateStarted:
		return status{code, prefix + "started"}
	case rpcServerStateStopping:
		return status{code, prefix + "stopping"}
	case rpcServerStateStopped:
		return status{code, prefix + "stopped"}
	default:
		return status{code, prefix + "not handled (internal error)"}
	}
}

// rpcServerWrap is a wrap around ops.Server. It has a double lock pattern where
// the outer protects the state and the inner protects the ops.Server instance
// itself (+stop func). This is as such because starting and in particular
// stopping the server can take time, and waiting for those processes to complete
// just to get a status is unnecessary.
type rpcServerWrap struct {
	mx    sync.Mutex
	state rpcServerState
	inner struct {
		mx          sync.Mutex
		server      *ops.Server
		serverStopF func()
	}
}

// handle with be the server handle, the thing that holds state.
type handle struct {
	ctx           context.Context
	addrSet       addrSet
	rpcServerWrap rpcServerWrap
}

// registerRoutes registers all endpoints for this server handle.
func (h *handle) registerRoutes(mux *http.ServeMux) {
	// Key: endpoint url, Val: rcv method.
	routes := map[string]func(http.ResponseWriter, *http.Request){
		"/ping":                 h.Ping,
		"/ops/rpc/addrs/put":    h.RPCAddrsPut,
		"/ops/rpc/addrs/get":    h.RPCAddrsGet,
		"/ops/rpc/server/stop":  h.RPCServerStop,
		"/ops/rpc/server/start": h.RPCServerStart,
	}

	for k, v := range routes {
		mux.Handle(k, http.HandlerFunc(v))
	}
}
