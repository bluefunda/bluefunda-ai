package server

import (
	"errors"
	"net/http"

	"github.com/bluefunda/bluefunda-ai/abaper/internal/adt"
)

// transportRequestHandler serves GET /api/v1/transports/{trkey}: the
// transport request with its tasks and objects, read with the per-request
// X-SAP-* client. Read-only.
//
//	400 trkey is not <SID>K<6 digits>
//	404 SAP has no such request
func (rs *RestServer) transportRequestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		rs.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	trkey, err := adt.NormalizeTransportNumber(r.PathValue("trkey"))
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}
	c, err := rs.clientForRequest(r)
	if err != nil {
		rs.sendClientError(w, err)
		return
	}
	result, err := c.GetTransportRequest(r.Context(), trkey)
	switch {
	case errors.Is(err, adt.ErrNotFound):
		rs.sendError(w, "transport request "+trkey+" not found", http.StatusNotFound)
		return
	case errors.Is(err, adt.ErrInvalidTransportNumber):
		rs.sendError(w, err.Error(), http.StatusBadRequest)
		return
	case err != nil:
		rs.sendError(w, err.Error(), http.StatusBadGateway)
		return
	}
	rs.sendSuccess(w, result)
}
