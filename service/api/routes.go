package api

import (
	"net/http"
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
