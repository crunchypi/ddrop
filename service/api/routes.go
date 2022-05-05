package api

import (
	"net/http"
)

func (h *handle) Ping(w http.ResponseWriter, r *http.Request) {
	withNetIO(w, r, func(_ struct{}) bool {
		return true
	})
}
