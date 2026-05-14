// storage_http_v2.go — Handlers HTTP del nuevo stack storage (Beta 8).
//
// Expone el StorageService vía REST en /api/storage/v2/...
// Convive con storage_http.go (rutas legacy de Beta 7 en /api/storage/...).
//
// Convenciones:
//   - Éxito:    HTTP 200, body { "data": ... }
//   - Error:    HTTP 4xx/5xx, body { "error": { "code": "...", "message": "..." } }
//
// Códigos HTTP por code semántico:
//   - pool_not_found, device_not_found             → 404
//   - bad_request, profile_invalid                 → 400
//   - pool_observed, capability_missing            → 403
//   - pool_name_taken, device_in_use,
//     operation_in_progress, min_disks_reached,
//     device_not_eligible, insufficient_disks      → 409
//   - btrfs_command_failed, mount_failed,
//     internal                                     → 500
//
// see docs/storage_http_api.md

package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// ─────────────────────────────────────────────────────────────────────────────
// HTTP response helpers
// ─────────────────────────────────────────────────────────────────────────────

// apiResponse es el wrapper estándar de toda respuesta exitosa.
type apiResponse struct {
	Data interface{} `json:"data"`
}

// apiErrorResponse es el wrapper estándar de toda respuesta de error.
type apiErrorResponse struct {
	Error apiError `json:"error"`
}

// apiError es el cuerpo del campo "error" en respuestas de fallo.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeJSON serializa data y la escribe como respuesta. status es el
// código HTTP. Si la serialización falla, escribe un error 500 sin body
// para no enviar JSON inválido.
func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	body, err := json.Marshal(payload)
	if err != nil {
		// No podemos garantizar JSON limpio si falló Marshal.
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// writeData es atajo para writeJSON con apiResponse.
func writeData(w http.ResponseWriter, status int, data interface{}) {
	writeJSON(w, status, apiResponse{Data: data})
}

// writeError serializa una respuesta de error con código y mensaje.
// El status HTTP se calcula desde el code semántico.
func writeError(w http.ResponseWriter, code, message string) {
	status := httpStatusForCode(code)
	writeJSON(w, status, apiErrorResponse{
		Error: apiError{Code: code, Message: message},
	})
}

// writeServiceError extrae el code de un ServiceError y delega en writeError.
// Si el error no es ServiceError, devuelve internal 500.
func writeServiceError(w http.ResponseWriter, err error) {
	if se, ok := err.(*ServiceError); ok {
		writeError(w, se.Code, se.Msg)
		return
	}
	logMsg("storage HTTP: unexpected error: %v", err)
	writeError(w, ErrCodeInternal, err.Error())
}

// httpStatusForCode mapea el code semántico a HTTP status.
func httpStatusForCode(code string) int {
	switch code {
	case ErrCodePoolNotFound, ErrCodeDeviceNotFound:
		return http.StatusNotFound
	case ErrCodeBadRequest, ErrCodeProfileInvalid:
		return http.StatusBadRequest
	case ErrCodePoolObserved, ErrCodeCapabilityMissing:
		return http.StatusForbidden
	case ErrCodePoolNameTaken,
		ErrCodeDeviceInUse,
		ErrCodeOperationInProgress,
		ErrCodeMinDisksReached,
		ErrCodeDeviceNotEligible,
		ErrCodeInsufficientDisks,
		ErrCodeTransitionNotPermitted:
		return http.StatusConflict
	case ErrCodeBtrfsCommandFailed, ErrCodeMountFailed, ErrCodeInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// methodNotAllowed responde 405 con el header Allow correcto.
func methodNotAllowed(w http.ResponseWriter, allowed ...string) {
	if len(allowed) > 0 {
		allowStr := allowed[0]
		for i := 1; i < len(allowed); i++ {
			allowStr += ", " + allowed[i]
		}
		w.Header().Set("Allow", allowStr)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusMethodNotAllowed)
	body, _ := json.Marshal(apiErrorResponse{
		Error: apiError{Code: ErrCodeBadRequest, Message: "method not allowed"},
	})
	_, _ = w.Write(body)
}

// ─────────────────────────────────────────────────────────────────────────────
// StorageHTTPHandler — agrupa los handlers
// ─────────────────────────────────────────────────────────────────────────────

// StorageHTTPHandler agrupa los handlers HTTP del módulo storage Beta 8.
// Inyectamos el service para que los tests puedan usar un service
// con DB temporal y mock executor.
type StorageHTTPHandler struct {
	service *StorageService
}

// NewStorageHTTPHandler crea el handler con el service inyectado.
func NewStorageHTTPHandler(service *StorageService) *StorageHTTPHandler {
	return &StorageHTTPHandler{service: service}
}

// Register registra todas las rutas en el mux dado.
// El path base es /api/storage/v2.
func (h *StorageHTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/storage/v2/pools", h.handlePools)
	mux.HandleFunc("/api/storage/v2/pools/", h.handlePoolByID)
	mux.HandleFunc("/api/storage/v2/devices", h.handleDevices)
	mux.HandleFunc("/api/storage/v2/operations", h.handleOperations)
	mux.HandleFunc("/api/storage/v2/generation", h.handleGeneration)
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/storage/v2/pools — lista todos los pools hidratados
// ─────────────────────────────────────────────────────────────────────────────

func (h *StorageHTTPHandler) handlePools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, "GET")
		return
	}

	pools, err := h.service.ListPools(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, pools)
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/storage/v2/pools/{id} — detalle de un pool
// ─────────────────────────────────────────────────────────────────────────────

func (h *StorageHTTPHandler) handlePoolByID(w http.ResponseWriter, r *http.Request) {
	id, rest := splitPoolIDPath(r.URL.Path)
	if id == "" {
		writeError(w, ErrCodeBadRequest, "missing pool id in path")
		return
	}
	if rest != "" {
		// Subrecursos (devices, rename, etc.) — gestionados en Bloque 4.
		writeError(w, ErrCodeBadRequest, "subresource not yet supported")
		return
	}

	if r.Method != http.MethodGet {
		methodNotAllowed(w, "GET")
		return
	}

	pool, err := h.service.GetPool(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, pool)
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/storage/v2/devices[?available=true]
// ─────────────────────────────────────────────────────────────────────────────

func (h *StorageHTTPHandler) handleDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, "GET")
		return
	}

	ctx := r.Context()
	availableOnly := r.URL.Query().Get("available") == "true"

	var (
		devices []*Device
		err     error
	)
	if availableOnly {
		devices, err = h.service.ListAvailableDevices(ctx)
	} else {
		devices, err = h.service.ListDevices(ctx)
	}
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, devices)
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/storage/v2/operations[?pool_id=X&status=Y&limit=N]
// ─────────────────────────────────────────────────────────────────────────────

func (h *StorageHTTPHandler) handleOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, "GET")
		return
	}

	q := r.URL.Query()
	filter := OperationFilter{}

	if poolID := q.Get("pool_id"); poolID != "" {
		filter.PoolID = &poolID
	}
	if statusStr := q.Get("status"); statusStr != "" {
		s := OperationStatus(statusStr)
		filter.Status = &s
	}
	if limitStr := q.Get("limit"); limitStr != "" {
		n, err := strconv.Atoi(limitStr)
		if err != nil || n < 0 {
			writeError(w, ErrCodeBadRequest,
				"invalid limit: must be a non-negative integer")
			return
		}
		filter.Limit = n
	}

	ops, err := h.service.ListOperations(r.Context(), filter)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, ops)
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/storage/v2/generation
// ─────────────────────────────────────────────────────────────────────────────

func (h *StorageHTTPHandler) handleGeneration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, "GET")
		return
	}

	gen, err := h.service.GetGeneration(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]int64{"generation": gen})
}

// ─────────────────────────────────────────────────────────────────────────────
// Path parsing
// ─────────────────────────────────────────────────────────────────────────────

// splitPoolIDPath extrae el pool ID de un path tipo
// "/api/storage/v2/pools/{id}" o "/api/storage/v2/pools/{id}/subresource/...".
// Devuelve (id, rest) donde rest puede ser vacío o "subresource/...".
// Si el path no comienza con el prefijo esperado, devuelve ("", "").
func splitPoolIDPath(urlPath string) (id, rest string) {
	const prefix = "/api/storage/v2/pools/"
	if !strings.HasPrefix(urlPath, prefix) {
		return "", ""
	}
	after := urlPath[len(prefix):]
	if after == "" {
		return "", ""
	}
	for i := 0; i < len(after); i++ {
		if after[i] == '/' {
			return after[:i], after[i+1:]
		}
	}
	return after, ""
}
