package httpapi

import (
	"net/http"

	"github.com/demonreeper/terrandominion/api/internal/store"
)

// Friends: GET /v1/friends, GET /v1/friends/search?q=, POST /v1/friends/{code} (request, or accept
// theirs), POST /v1/friends/{code}/accept, POST /v1/friends/{code}/decline, DELETE /v1/friends/{code},
// PUT /v1/me/friends (the name friends see, findable by email, accept requests).

func (s *Server) friends(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	view, err := s.svc.Friends(r.Context(), playerID)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, view)
}

func (s *Server) findFriends(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	found, err := s.svc.FindPlayers(r.Context(), playerID, r.URL.Query().Get("q"))
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, found)
}

func (s *Server) requestFriend(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	status, err := s.svc.RequestFriend(r.Context(), playerID, r.PathValue("code"))
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]string{"status": status})
}

func (s *Server) answerFriend(accept bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, _ := playerFrom(r.Context())
		if err := s.svc.AnswerFriend(r.Context(), playerID, r.PathValue("code"), accept); err != nil {
			fail(w, r, err)
			return
		}
		ok(w, map[string]bool{"ok": true})
	}
}

func (s *Server) removeFriend(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	if err := s.svc.RemoveFriend(r.Context(), playerID, r.PathValue("code")); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]bool{"ok": true})
}

func (s *Server) setFriendSettings(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body store.FriendSettings
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	saved, err := s.svc.SetFriendSettings(r.Context(), playerID, body)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, saved)
}

func (s *Server) setPrivacy(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		EmailSearchable bool `json:"emailSearchable"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	saved, err := s.svc.SetEmailSearchable(r.Context(), playerID, body.EmailSearchable)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, saved)
}
