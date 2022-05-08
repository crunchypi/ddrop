package api

import (
	"net/http"
	"sync"

	"github.com/crunchypi/ddrop/service/ops"
	rman "github.com/crunchypi/ddrop/service/requestman"
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

func (h *handle) RPCPing(w http.ResponseWriter, r *http.Request) {
	// Payload type of return from deferred rpc call clientResult.
	type T = bool
	withNetIO(w, r, func(opts struct{}) []clientResult[T] {
		addrs := h.addrSet.addrsMaintanedLocked()
		ch := ops.NewClients(addrs).Ping()
		return newClientResults(ch, func(payload T) T { return payload })
	})
}

func (h *handle) RPCAddData(w http.ResponseWriter, r *http.Request) {
	// Payload type of return from deferred rpc call clientResult.
	type T = []bool
	withNetIO(w, r, func(opts []addDataArgs) []clientResult[T] {
		addrs := h.addrSet.addrsMaintanedLocked()

		optsExported := make([]ops.AddDataArgs, 0, len(opts))
		for _, opt := range opts {
			optsExported = append(optsExported, opt.export())
		}

		ch := ops.NewClients(addrs).AddData(optsExported)
		return newClientResults(ch, func(payload T) T { return payload })
	})
}

func (h *handle) RPCKNNEager(w http.ResponseWriter, r *http.Request) {
	withNetIO(w, r, func(opts knnArgs) []knnResp {
		addrs := h.addrSet.addrsMaintanedLocked()

		ch := make(chan knnResp)
		wg := sync.WaitGroup{}
		wg.Add(len(opts.QueryVecs))

		for i, knnArgs := range opts.export() {
			// Per query vec.
			go func(i int, knnArgs rman.KNNArgs) {
				defer wg.Done()

				// Gather results from remote rpc servers.
				knnResults := make([]clientResult[knnRespItem], 0, knnArgs.K)
				for _, cliResult := range ops.NewClients(addrs).KNNEagerx(knnArgs) {
					knnResult := newClientResult(
						*cliResult,
						func(payload ops.KNNRespItem) knnRespItem {
							return knnRespItem{
								Vec:   payload.Vec,
								Score: payload.Score,
							}
						})

					knnResults = append(knnResults, knnResult)
				}

				ch <- knnResp{
					QueryVec:      knnArgs.QueryVec,
					QueryVecIndex: i,
					Results:       knnResults,
				}
			}(i, knnArgs)
		}
		go func() { wg.Wait(); close(ch) }()

		// Unpack chan -> slice.
		resps := make([]knnResp, 0, len(addrs))
		for iKNNResp := range ch {
			resps = append(resps, iKNNResp)
		}
		return resps
	})
}

func (h *handle) RPCSSpaceNamespaces(w http.ResponseWriter, r *http.Request) {
	type T = []string
	withNetIO(w, r, func(_ struct{}) []clientResult[T] {
		addrs := h.addrSet.addrsMaintanedLocked()
		ch := ops.NewClients(addrs).Info().SSpaceNamespaces()
		return newClientResults(ch, func(payload T) T { return payload })
	})

}

func (h *handle) RPCSSpaceNamespace(w http.ResponseWriter, r *http.Request) {
	// Payload type of return from deferred rpc call clientResult.
	type T = bool
	withNetIO(w, r, func(opts string) []clientResult[T] {
		addrs := h.addrSet.addrsMaintanedLocked()
		ch := ops.NewClients(addrs).Info().SSpaceNamespace(opts)
		return newClientResults(ch, func(payload T) T { return payload })
	})
}

func (h *handle) RPCSSpaceDim(w http.ResponseWriter, r *http.Request) {
	// Payload type of return from deferred rpc call.
	type T = sSpaceDimResp
	withNetIO(w, r, func(opts string) []clientResult[T] {
		addrs := h.addrSet.addrsMaintanedLocked()
		ch := ops.NewClients(addrs).Info().SSpaceDim(opts)
		return newClientResults(ch, func(payload ops.SSpaceDimResp) T {
			return T{
				LookupOk: payload.LookupOk,
				Dim:      payload.Dim,
			}
		})
	})
}

func (h *handle) RPCSSpaceLen(w http.ResponseWriter, r *http.Request) {
	// Payload type of return from deferred rpc call clientResult.
	type T = sSpaceLenResp
	withNetIO(w, r, func(opts string) []clientResult[T] {
		addrs := h.addrSet.addrsMaintanedLocked()
		ch := ops.NewClients(addrs).Info().SSpaceLen(opts)

		return newClientResults(ch, func(payload ops.SSpaceLenResp) T {
			return T{
				LookupOk: payload.LookupOk,
				NSSpaces: payload.NSSpaces,
				NVecs:    payload.NVecs,
			}
		})
	})
}

func (h *handle) RPCSSpaceCap(w http.ResponseWriter, r *http.Request) {
	// Payload type of return from deferred rpc call clientResult.
	type T = sSpaceCapResp
	withNetIO(w, r, func(opts string) []clientResult[T] {
		addrs := h.addrSet.addrsMaintanedLocked()
		ch := ops.NewClients(addrs).Info().SSpaceCap(opts)

		return newClientResults(ch, func(payload ops.SSpaceCapResp) T {
			return T{
				LookupOk: payload.LookupOk,
				Cap:      payload.Cap,
			}
		})
	})
}

func (h *handle) RPCKNNLatency(w http.ResponseWriter, r *http.Request) {
	// Payload type of return from deferred rpc call clientResult.
	type T = knnLatencyResp
	withNetIO(w, r, func(opts knnLatencyArgs) []clientResult[T] {
		addrs := h.addrSet.addrsMaintanedLocked()

		conv := ops.KNNLatencyArgs{
			Key:    opts.Key,
			Period: opts.Period,
		}
		ch := ops.NewClients(addrs).Info().KNNLatency(conv)

		return newClientResults(ch, func(payload ops.KNNLatencyResp) T {
			return T{
				LookupOk: payload.LookupOk,
				Queue:    payload.Queue,
				Query:    payload.Query,
				BoundsOk: payload.BoundsOk,
			}
		})
	})
}

func (h *handle) RPCKNNMonitor(w http.ResponseWriter, r *http.Request) {
	// Payload type of return from deferred rpc call clientResult.
	type T = knnMonItemAvg
	withNetIO(w, r, func(opts knnMonArgs) []clientResult[T] {
		addrs := h.addrSet.addrsMaintanedLocked()

		conv := ops.KNNMonArgs{
			Start: opts.Start,
			End:   opts.End,
		}
		ch := ops.NewClients(addrs).Info().KNNMonitor(conv)

		return newClientResults(ch, func(payload rman.KNNMonItemAvg) T {
			return T{
				Created:         payload.Created,
				Span:            payload.Span,
				N:               payload.N,
				NFailed:         payload.NFailed,
				AvgLatency:      payload.AvgLatency,
				AvgScore:        payload.AvgScore,
				AvgScoreNoFails: payload.AvgScoreNoFails,
				AvgSatisfaction: payload.AvgSatisfaction,
			}
		})
	})
}
