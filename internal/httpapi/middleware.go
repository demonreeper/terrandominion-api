package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
)

type ctxKey int

const (
	keyRequestID ctxKey = iota
	keyPlayer
	keySession
	keyAdmin
)

func requestID(ctx context.Context) string {
	id, _ := ctx.Value(keyRequestID).(string)
	return id
}

func playerFrom(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(keyPlayer).(int64)
	return id, ok
}

func sessionFrom(ctx context.Context) int64 {
	id, _ := ctx.Value(keySession).(int64)
	return id
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// checkAppBuild refuses app builds below MinAppBuild / MinAppBuildIOS (E1006). The app sends
// X-App-Build (its build number) and X-App-Platform on every request; requests without a build
// (scripts, curl, health checks) pass.
func (s *Server) checkAppBuild(r *http.Request) error {
	if !strings.HasPrefix(r.URL.Path, "/v1/") {
		return nil
	}
	build, _ := strconv.Atoi(r.Header.Get("X-App-Build"))
	return s.svc.CheckAppBuild(r.Context(), r.Header.Get("X-App-Platform"), build)
}

// base wraps every request: request id, panic recovery, access log, IP rate limit and the app build lock.
func (s *Server) base(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		id := newRequestID()
		ctx := context.WithValue(r.Context(), keyRequestID, id)
		r = r.WithContext(ctx)
		w.Header().Set("X-Request-Id", id)
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		defer func() {
			if p := recover(); p != nil {
				slog.Error("E1900 panic", "request", id, "path", r.URL.Path, "panic", p, "stack", string(debug.Stack()))
				fail(rec, r, apperr.Internal)
			}
			slog.Debug("request", "request", id, "method", r.Method, "path", r.URL.Path,
				"status", rec.status, "ms", time.Since(started).Milliseconds())
		}()

		if !s.ipLimiter.allow(s.clientIP(r), time.Now()) {
			fail(rec, r, apperr.RateLimited)
			return
		}
		if err := s.checkAppBuild(r); err != nil {
			fail(rec, r, err)
			return
		}
		next.ServeHTTP(rec, r)
	})
}

// authed requires a valid access token and rate-limits per player.
func (s *Server) authed(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		token, found := strings.CutPrefix(header, "Bearer ")
		if !found || token == "" {
			fail(w, r, apperr.Unauthorized)
			return
		}
		claims, err := s.svc.Tokens().ParseAccess(token)
		if err != nil {
			fail(w, r, apperr.Unauthorized.With("%v", err))
			return
		}
		playerID, err := claims.PlayerID()
		if err != nil {
			fail(w, r, apperr.Unauthorized.With("subject: %v", err))
			return
		}
		// A token from before player ids were moved names an id that may now be someone else's.
		if after := s.svc.Settings(r.Context()).TokensValidAfter; after > 0 &&
			(claims.IssuedAt == nil || float64(claims.IssuedAt.Unix()) < after) {
			fail(w, r, apperr.Unauthorized.With("token issued before TokensValidAfter"))
			return
		}
		if !s.playerLimiter.allow("p"+itoa(int(playerID)), time.Now()) {
			fail(w, r, apperr.RateLimited)
			return
		}
		ctx := context.WithValue(r.Context(), keyPlayer, playerID)
		ctx = context.WithValue(ctx, keySession, claims.SessionID)
		ctx = context.WithValue(ctx, keyAdmin, claims.Admin)
		next(w, r.WithContext(ctx))
	}
}

// admin requires an admin token, and re-checks the database so a revoked admin loses access at
// once rather than when their token expires.
func (s *Server) admin(next http.HandlerFunc) http.HandlerFunc {
	return s.authed(func(w http.ResponseWriter, r *http.Request) {
		playerID, _ := playerFrom(r.Context())
		isAdmin, _ := r.Context().Value(keyAdmin).(bool)
		if !isAdmin {
			fail(w, r, apperr.Forbidden)
			return
		}
		me, err := s.svc.Me(r.Context(), playerID)
		if err != nil {
			fail(w, r, err)
			return
		}
		if !me.IsAdmin {
			fail(w, r, apperr.Forbidden.With("admin flag removed"))
			return
		}
		next(w, r)
	})
}

// mod lets moderators and admins through. The role is read from the database on every call, so a
// removed role stops working at once.
func (s *Server) mod(next http.HandlerFunc) http.HandlerFunc {
	return s.authed(func(w http.ResponseWriter, r *http.Request) {
		playerID, _ := playerFrom(r.Context())
		staff, err := s.svc.IsStaff(r.Context(), playerID)
		if err != nil {
			fail(w, r, err)
			return
		}
		if !staff {
			fail(w, r, apperr.Forbidden)
			return
		}
		next(w, r)
	})
}

func (s *Server) clientIP(r *http.Request) string {
	if s.cfg.TrustProxy {
		if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// limiter is a small in-memory token bucket per key. One API process serves the game for now;
// when there are several, this moves to Redis or nginx.
type limiter struct {
	mu      sync.Mutex
	rate    float64 // tokens per second
	burst   float64
	buckets map[string]*bucket
}

type bucket struct {
	tokens float64
	seen   time.Time
}

func newLimiter(perSecond, burst float64) *limiter {
	return &limiter{rate: perSecond, burst: burst, buckets: map[string]*bucket{}}
}

func (l *limiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, found := l.buckets[key]
	if !found {
		if len(l.buckets) > 100_000 {
			l.sweep(now)
		}
		l.buckets[key] = &bucket{tokens: l.burst - 1, seen: now}
		return true
	}
	b.tokens = min(l.burst, b.tokens+now.Sub(b.seen).Seconds()*l.rate)
	b.seen = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (l *limiter) sweep(now time.Time) {
	for key, b := range l.buckets {
		if now.Sub(b.seen) > 10*time.Minute {
			delete(l.buckets, key)
		}
	}
}
