package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Error codes returned in {"error": {"code", "message"}}.
const (
	codeInvalidJSON      = "invalid_json"
	codeValidation       = "validation_error"
	codeUnauthenticated  = "unauthenticated"
	codeForbidden        = "forbidden"
	codeNotFound         = "not_found"
	codeInviteNotFound   = "invite_not_found"
	codeLessonNotFound   = "lesson_not_found"
	codeTaskNotFound     = "task_not_found"
	codeTooManyTasks     = "too_many_tasks"
	codeFamilyFull       = "family_full"
	codeMethodNotAllowed = "method_not_allowed"
	codeInternal         = "internal"
)

const maxBodyBytes = 16 << 10

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json response", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}

// writeInternal logs the real error and returns a generic 500.
func writeInternal(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("request failed", "method", r.Method, "path", routePattern(r), "err", err)
	writeError(w, http.StatusInternalServerError, codeInternal, "Something went wrong on our side. Please try again.")
}

// decodeJSON reads a JSON object from the request body into dst.
// On failure it writes a 400 and returns false.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidJSON, "The request body must be a JSON object.")
		return false
	}
	return true
}
