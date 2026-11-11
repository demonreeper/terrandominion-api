package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
)

// Every response is {"data": ...} or {"error": {"code": 1200, "message": "...", "data": {...}}}.
// Clients switch on the numeric code; the message is a sentence for display, and the error's data
// (left out when empty) holds the numbers behind it.
type envelope struct {
	Data  any           `json:"data,omitempty"`
	Error *apperr.Error `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Warn("response write failed", "err", err)
	}
}

func ok(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, envelope{Data: data})
}

// fail writes the numbered error and logs it with its code and request id, so a code a player
// reports can be found in the log. Client mistakes log at info, server faults at error.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	e := apperr.As(err)
	attrs := []any{"code", e.Code, "path", r.URL.Path, "request", requestID(r.Context())}
	if pid, ok := playerFrom(r.Context()); ok {
		attrs = append(attrs, "player", pid)
	}
	if e.Detail != "" {
		attrs = append(attrs, "detail", e.Detail)
	}
	if e.Status >= 500 {
		slog.Error("E"+itoa(e.Code)+" "+e.Message, attrs...)
	} else {
		slog.Info("E"+itoa(e.Code)+" "+e.Message, attrs...)
	}
	if pid, ok := playerFrom(r.Context()); ok && onFail != nil {
		onFail(r, pid, e)
	}
	writeJSON(w, e.Status, envelope{Error: e})
}

// onFail is told about every refused request of a signed-in player; the server uses it to store
// failed location checks (td.infringements). Set once when the server is built.
var onFail func(r *http.Request, playerID int64, e *apperr.Error)

const maxBody = 64 << 10

// decode reads a JSON body strictly: unknown fields and trailing data are rejected, so client
// typos surface during development instead of being silently ignored.
func decode(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return apperr.TooLarge
		}
		return apperr.BadRequest.With("json: %v", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperr.BadRequest.With("trailing data after JSON body")
	}
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
