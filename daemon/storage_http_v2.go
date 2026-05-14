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
	"fmt"
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

// storageHTTPHandler es la instancia global. Inicializada por initStorageModule()
// y consumida por startHTTPServer() para registrar las rutas v2.
var storageHTTPHandler *StorageHTTPHandler

// NewStorageHTTPHandler crea el handler con el service inyectado.
func NewStorageHTTPHandler(service *StorageService) *StorageHTTPHandler {
	return &StorageHTTPHandler{service: service}
}

// Register registra todas las rutas en el mux dado.
// El path base es /api/storage/v2.
//
// Todas las rutas pasan por requireAdmin (mismo patrón que Beta 7):
// si la sesión no es admin → 401 Unauthorized sin tocar el service.
func (h *StorageHTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/storage/v2/pools", h.requireAdmin(h.handlePools))
	mux.HandleFunc("/api/storage/v2/pools/", h.requireAdmin(h.handlePoolByID))
	mux.HandleFunc("/api/storage/v2/devices", h.requireAdmin(h.handleDevices))
	mux.HandleFunc("/api/storage/v2/operations", h.requireAdmin(h.handleOperations))
	mux.HandleFunc("/api/storage/v2/generation", h.requireAdmin(h.handleGeneration))
	mux.HandleFunc("/api/storage/v2/scan", h.requireAdmin(h.handleScan))
}

// requireAdmin envuelve un handler con verificación de sesión admin.
// Si requireAdmin (definida en sessions.go) devuelve nil, ya ha escrito
// el 401 en w, así que solo retornamos.
func (h *StorageHTTPHandler) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if requireAdmin(w, r) == nil {
			return
		}
		next(w, r)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// /api/storage/v2/pools — GET (list) | POST (create)
// ─────────────────────────────────────────────────────────────────────────────

func (h *StorageHTTPHandler) handlePools(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listPools(w, r)
	case http.MethodPost:
		h.createPool(w, r)
	default:
		methodNotAllowed(w, "GET", "POST")
	}
}

// listPools — GET /api/storage/v2/pools
func (h *StorageHTTPHandler) listPools(w http.ResponseWriter, r *http.Request) {
	pools, err := h.service.ListPools(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, pools)
}

// createPool — POST /api/storage/v2/pools
//
// Body:
//   {"name": "data", "profile": "raid1",
//    "device_ids": ["d1", "d2"],
//    "compression": "zstd", "wipe_first": false}
func (h *StorageHTTPHandler) createPool(w http.ResponseWriter, r *http.Request) {
	var req CreatePoolRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, ErrCodeBadRequest, err.Error())
		return
	}

	op, err := h.service.CreatePool(r.Context(), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	// 200 OK + Operation. El frontend mira op.Status y op.PoolID
	// para decidir cómo seguir.
	writeData(w, http.StatusOK, op)
}

// ─────────────────────────────────────────────────────────────────────────────
// /api/storage/v2/pools/{id}[/subresource...]
//
// Multiplexa según path y método:
//   GET    /pools/{id}                            → detalle
//   DELETE /pools/{id}                            → destroy
//   POST   /pools/{id}/rename                     → rename
//   POST   /pools/{id}/set-compression            → set compression
//   POST   /pools/{id}/convert-profile            → convert profile
//   POST   /pools/{id}/devices                    → add device
//   DELETE /pools/{id}/devices/{deviceID}         → remove device
//   POST   /pools/{id}/devices/{deviceID}/replace → replace device
// ─────────────────────────────────────────────────────────────────────────────

func (h *StorageHTTPHandler) handlePoolByID(w http.ResponseWriter, r *http.Request) {
	id, rest := splitPoolIDPath(r.URL.Path)
	if id == "" {
		writeError(w, ErrCodeBadRequest, "missing pool id in path")
		return
	}

	// Caso 1: /pools/{id}     (sin subrecurso)
	if rest == "" {
		switch r.Method {
		case http.MethodGet:
			h.getPool(w, r, id)
		case http.MethodDelete:
			h.destroyPool(w, r, id)
		default:
			methodNotAllowed(w, "GET", "DELETE")
		}
		return
	}

	// Caso 2: /pools/{id}/{subresource...}
	switch {
	case rest == "rename":
		if r.Method != http.MethodPost {
			methodNotAllowed(w, "POST")
			return
		}
		h.renamePool(w, r, id)

	case rest == "set-compression":
		if r.Method != http.MethodPost {
			methodNotAllowed(w, "POST")
			return
		}
		h.setCompression(w, r, id)

	case rest == "convert-profile":
		if r.Method != http.MethodPost {
			methodNotAllowed(w, "POST")
			return
		}
		h.convertProfile(w, r, id)

	case rest == "devices":
		if r.Method != http.MethodPost {
			methodNotAllowed(w, "POST")
			return
		}
		h.addDevice(w, r, id)

	case strings.HasPrefix(rest, "devices/"):
		// /pools/{id}/devices/{deviceID}[/replace]
		deviceTail := rest[len("devices/"):]
		deviceID, deviceRest := splitFirstSegment(deviceTail)
		if deviceID == "" {
			writeError(w, ErrCodeBadRequest, "missing device id in path")
			return
		}
		switch {
		case deviceRest == "":
			if r.Method != http.MethodDelete {
				methodNotAllowed(w, "DELETE")
				return
			}
			h.removeDevice(w, r, id, deviceID)
		case deviceRest == "replace":
			if r.Method != http.MethodPost {
				methodNotAllowed(w, "POST")
				return
			}
			h.replaceDevice(w, r, id, deviceID)
		default:
			writeError(w, ErrCodeBadRequest, "unknown device subresource")
		}

	default:
		writeError(w, ErrCodeBadRequest, "unknown pool subresource")
	}
}

// ─── /pools/{id} handlers ─────────────────────────────────────────────────────

func (h *StorageHTTPHandler) getPool(w http.ResponseWriter, r *http.Request, id string) {
	pool, err := h.service.GetPool(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, pool)
}

func (h *StorageHTTPHandler) destroyPool(w http.ResponseWriter, r *http.Request, id string) {
	op, err := h.service.DestroyPool(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, op)
}

// ─── /pools/{id}/rename ───────────────────────────────────────────────────────

type renamePoolRequest struct {
	Name string `json:"name"`
}

func (h *StorageHTTPHandler) renamePool(w http.ResponseWriter, r *http.Request, id string) {
	var req renamePoolRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, ErrCodeBadRequest, err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, ErrCodeBadRequest, "name is required")
		return
	}
	op, err := h.service.RenamePool(r.Context(), id, req.Name)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, op)
}

// ─── /pools/{id}/set-compression ──────────────────────────────────────────────

type setCompressionRequest struct {
	Algorithm string `json:"algorithm"`
}

func (h *StorageHTTPHandler) setCompression(w http.ResponseWriter, r *http.Request, id string) {
	var req setCompressionRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, ErrCodeBadRequest, err.Error())
		return
	}
	if req.Algorithm == "" {
		writeError(w, ErrCodeBadRequest, "algorithm is required")
		return
	}
	op, err := h.service.SetPoolCompression(r.Context(), id, req.Algorithm)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, op)
}

// ─── /pools/{id}/convert-profile ──────────────────────────────────────────────

type convertProfileBody struct {
	NewProfile Profile `json:"new_profile"`
}

func (h *StorageHTTPHandler) convertProfile(w http.ResponseWriter, r *http.Request, id string) {
	var body convertProfileBody
	if err := decodeJSONBody(r, &body); err != nil {
		writeError(w, ErrCodeBadRequest, err.Error())
		return
	}
	op, err := h.service.ConvertProfile(r.Context(), ConvertProfileRequest{
		PoolID:     id,
		NewProfile: body.NewProfile,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, op)
}

// ─── /pools/{id}/devices ──────────────────────────────────────────────────────

type addDeviceBody struct {
	DeviceID  string `json:"device_id"`
	WipeFirst bool   `json:"wipe_first,omitempty"`
}

func (h *StorageHTTPHandler) addDevice(w http.ResponseWriter, r *http.Request, poolID string) {
	var body addDeviceBody
	if err := decodeJSONBody(r, &body); err != nil {
		writeError(w, ErrCodeBadRequest, err.Error())
		return
	}
	if body.DeviceID == "" {
		writeError(w, ErrCodeBadRequest, "device_id is required")
		return
	}
	op, err := h.service.AddDevice(r.Context(), AddDeviceRequest{
		PoolID:    poolID,
		DeviceID:  body.DeviceID,
		WipeFirst: body.WipeFirst,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, op)
}

// ─── /pools/{id}/devices/{deviceID} ───────────────────────────────────────────

func (h *StorageHTTPHandler) removeDevice(w http.ResponseWriter, r *http.Request, poolID, deviceID string) {
	op, err := h.service.RemoveDevice(r.Context(), RemoveDeviceRequest{
		PoolID:   poolID,
		DeviceID: deviceID,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, op)
}

// ─── /pools/{id}/devices/{deviceID}/replace ───────────────────────────────────

type replaceDeviceBody struct {
	NewDeviceID string `json:"new_device_id"`
}

func (h *StorageHTTPHandler) replaceDevice(w http.ResponseWriter, r *http.Request, poolID, oldDeviceID string) {
	var body replaceDeviceBody
	if err := decodeJSONBody(r, &body); err != nil {
		writeError(w, ErrCodeBadRequest, err.Error())
		return
	}
	if body.NewDeviceID == "" {
		writeError(w, ErrCodeBadRequest, "new_device_id is required")
		return
	}
	op, err := h.service.ReplaceDevice(r.Context(), ReplaceDeviceRequest{
		PoolID:      poolID,
		OldDeviceID: oldDeviceID,
		NewDeviceID: body.NewDeviceID,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, op)
}

// ─────────────────────────────────────────────────────────────────────────────
// /api/storage/v2/scan — POST → ScanDevices
// ─────────────────────────────────────────────────────────────────────────────

func (h *StorageHTTPHandler) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, "POST")
		return
	}
	result, err := h.service.ScanDevices(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, result)
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
	return splitFirstSegment(after)
}

// splitFirstSegment toma una cadena tipo "abc/def/ghi" y la divide en
// el primer segmento y el resto: ("abc", "def/ghi"). Si no hay "/",
// devuelve (cadena, "").
func splitFirstSegment(s string) (first, rest string) {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return s[:i], s[i+1:]
		}
	}
	return s, ""
}

// decodeJSONBody decodifica el body de la petición en dest. Limita
// el tamaño del body a 64 KB para evitar abuso. Rechaza JSON malformado.
//
// El caller debe validar los CAMPOS de dest (longitudes, no-vacíos, etc.).
// Esta función solo valida que el JSON parsea.
func decodeJSONBody(r *http.Request, dest interface{}) error {
	if r.Body == nil {
		return fmt.Errorf("empty request body")
	}
	// 64 KB es de sobra para nuestros payloads (pool name + 4 device IDs)
	r.Body = http.MaxBytesReader(nil, r.Body, 64*1024)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // rechaza campos extra → ayuda a detectar typos del cliente
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("invalid JSON body: %v", err)
	}
	return nil
}
