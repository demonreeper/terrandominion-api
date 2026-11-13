package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/game"
)

// located wraps a route that acts on a place (anti-spoofing G0): the fix it carries goes through
// GuardFix before the handler runs, so a signal lock, a stale fix, a fake-location flag or a teleport
// refuses it in one place. The body is read and put back, so the handler reads it as before. A route
// that names no place (a job take) is checked for the lock only.
func (s *Server) located(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, _ := playerFrom(r.Context())
		fix, err := readFix(r)
		if err != nil {
			fail(w, r, apperr.BadRequest.With("fix"))
			return
		}
		if err := s.svc.GuardFix(r.Context(), playerID, fix, r.URL.Path); err != nil {
			fail(w, r, err)
			return
		}
		next(w, r)
	}
}

// maxFixBody is plenty for any request that carries a fix.
const maxFixBody = 64 << 10

// fixFields are the fix keys every location request may carry. accuracy is the nearby scan's name.
type fixFields struct {
	Lat       float64    `json:"lat"`
	Lon       float64    `json:"lon"`
	AccuracyM float64    `json:"accuracyM"`
	Mock      bool       `json:"mock"`
	FixAt     *time.Time `json:"fixAt"`
}

func readFix(r *http.Request) (game.FixReport, error) {
	var f fixFields
	if r.Method == http.MethodGet {
		q := r.URL.Query()
		f.Lat, _ = strconv.ParseFloat(q.Get("lat"), 64)
		f.Lon, _ = strconv.ParseFloat(q.Get("lon"), 64)
		f.AccuracyM, _ = strconv.ParseFloat(q.Get("accuracyM"), 64)
		if f.AccuracyM == 0 {
			f.AccuracyM, _ = strconv.ParseFloat(q.Get("accuracy"), 64)
		}
		f.Mock = q.Get("mock") == "1" || q.Get("mock") == "true"
		if at, err := time.Parse(time.RFC3339, q.Get("fixAt")); err == nil {
			f.FixAt = &at
		}
	} else if r.Body != nil {
		raw, err := io.ReadAll(io.LimitReader(r.Body, maxFixBody))
		if err != nil {
			return game.FixReport{}, err
		}
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(raw))
		if len(bytes.TrimSpace(raw)) > 0 {
			// A body that isn't a fix (or has none) simply carries no position; the handler judges the rest.
			_ = json.Unmarshal(raw, &f)
		}
	}
	// From G1 the app sends the fake flag and the fix's time as headers on every request.
	if !f.Mock {
		f.Mock = r.Header.Get("X-Fix-Mock") == "1"
	}
	if f.FixAt == nil {
		if at, err := time.Parse(time.RFC3339Nano, r.Header.Get("X-Fix-At")); err == nil {
			f.FixAt = &at
		}
	}
	return game.FixReport{Lat: f.Lat, Lon: f.Lon, AccuracyM: f.AccuracyM, Mock: f.Mock, FixAt: f.FixAt}, nil
}

// grounded refuses a war action while the phone reports itself well above the ground (game.WarUpHigh: a balcony or
// a tower flat). The app sends X-Fix-Height (metres above ground) and X-Fix-Height-Acc with every request when it
// knows them; without them nothing is refused. Wrap it inside located, on attacks, drones, hacks, contests and
// defending; never on harvesting, collecting or building.
func (s *Server) grounded(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		height, accuracy := readHeight(r)
		st := s.svc.Settings(r.Context())
		if game.WarUpHigh(height, accuracy, st) {
			fail(w, r, apperr.WarUpHigh.WithData(map[string]any{"heightM": math.Round(height), "maxM": st.WarMaxHeightM}))
			return
		}
		next(w, r)
	}
}

// readHeight is the phone's height above ground and its accuracy from the headers; NaN and 0 when not sent.
func readHeight(r *http.Request) (float64, float64) {
	height, err := strconv.ParseFloat(r.Header.Get("X-Fix-Height"), 64)
	if err != nil {
		height = math.NaN()
	}
	accuracy, _ := strconv.ParseFloat(r.Header.Get("X-Fix-Height-Acc"), 64)
	return height, accuracy
}
