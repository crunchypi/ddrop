package api

import (
	"net/http"

	"github.com/crunchypi/ddrop/service/ops"
)

func (h *handle) Ping(w http.ResponseWriter, r *http.Request) {
	withNetIO(w, r, func(_ struct{}) bool {
		return true
	})
}

func (h *handle) RPCAddrsPut(w http.ResponseWriter, r *http.Request) {
	withNetIO(w, r, func(addrs []string) []string {
		return h.addrSet.addrsMaintanedLocked(addrs...)
	})
}

func (h *handle) RPCAddrsGet(w http.ResponseWriter, r *http.Request) {
	withNetIO(w, r, func(_ struct{}) []string {
		return h.addrSet.addrsMaintanedLocked()
	})
}

func (h *handle) RPCServerStop(w http.ResponseWriter, r *http.Request) {
	withNetIO(w, r, func(_ struct{}) status {
		h.rpcServerWrap.mx.Lock()
		// Not deferring unlock because of double locking mechanism.

		// Only valid state for stopping is "...Running".
		if h.rpcServerWrap.state != rpcServerStateStarted {
			state := h.rpcServerWrap.state
			h.rpcServerWrap.mx.Unlock()
			w.WriteHeader(http.StatusConflict)
			return state.toStatus()
		}

		// Outer update and unlock.
		h.rpcServerWrap.state = rpcServerStateStopping
		h.rpcServerWrap.mx.Unlock()

		// Inner handling.
		h.rpcServerWrap.inner.mx.Lock()
		h.rpcServerWrap.inner.serverStopF()
		h.rpcServerWrap.inner.serverStopF = nil
		h.rpcServerWrap.inner.server = nil
		h.rpcServerWrap.inner.mx.Unlock()

		// Outer update since now the state should be "...Stopped".
		h.rpcServerWrap.mx.Lock()
		defer h.rpcServerWrap.mx.Unlock()
		h.rpcServerWrap.state = rpcServerStateStopped
		return h.rpcServerWrap.state.toStatus()
	})
}

func (h *handle) RPCServerStart(w http.ResponseWriter, r *http.Request) {
	withNetIO(w, r, func(opts rpcServerStartArgs) status {

		// Validate.
		conv := opts.Cfg.export(h.ctx)
		if !conv.Ok() {
			w.WriteHeader(http.StatusBadRequest)
			return status{}
		}

		// Set up new potential server. Doing this here to reduce mutex
		// locking (and unlocking) complexity further down.
		newServer, ok := ops.NewServer(opts.Addr, conv)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return status{}
		}

		newServerStopF, err := newServer.StartListen()
		if err != nil {
			//w.Write([]byte(err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
			return status{}
		}

		// Add the new addr.
		h.addrSet.addrsMaintanedLocked(opts.Addr)

		// Try starting below.
		// Not deferring unlock because of double locking mechanism.
		h.rpcServerWrap.mx.Lock()

		// Only valid state for stopping is "...Default/Stopped".
		ok = false
		ok = ok || h.rpcServerWrap.state == rpcServerStateDefault
		ok = ok || h.rpcServerWrap.state == rpcServerStateStopped
		if !ok {
			state := h.rpcServerWrap.state
			h.rpcServerWrap.mx.Unlock()
			newServerStopF() // Don't need it anymore.
			w.WriteHeader(http.StatusConflict)
			return state.toStatus()
		}

		// Outer update and unlock.
		h.rpcServerWrap.state = rpcServerStateStarting
		h.rpcServerWrap.mx.Unlock()

		// Inner handling. Again, intentionally not deferring unlock.
		h.rpcServerWrap.inner.mx.Lock()
		h.rpcServerWrap.inner.server = newServer
		h.rpcServerWrap.inner.serverStopF = newServerStopF
		h.rpcServerWrap.inner.mx.Unlock()

		// Outer update since now the state should be "...Started".
		h.rpcServerWrap.mx.Lock()
		defer h.rpcServerWrap.mx.Unlock()
		h.rpcServerWrap.state = rpcServerStateStarted
		return h.rpcServerWrap.state.toStatus()
	})
}
