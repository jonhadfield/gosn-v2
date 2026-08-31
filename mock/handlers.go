package mock

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// accessTokenPrefix marks the tokens the mock issues. It deliberately avoids
// the "2:" prefix that means cookie-based authentication, so that clients
// authenticate with the Authorization header alone.
const accessTokenPrefix = "mock-access-"

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, tag, message string) {
	writeJSON(w, status, map[string]any{
		"meta": map[string]any{},
		"data": map[string]any{
			"error": map[string]any{"tag": tag, "message": message},
		},
	})
}

// emailMatches compares an email from a request with the account's, allowing
// for the path escaping clients apply to it.
func (s *Server) emailMatches(in string) bool {
	if in == s.Email {
		return true
	}

	unescaped, err := url.PathUnescape(in)

	return err == nil && unescaped == s.Email
}

func (s *Server) handleAuthParams(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid-request", err.Error())
		return
	}

	nonce := s.keyParams.PwNonce
	if !s.emailMatches(req.Email) {
		// Real servers answer for unknown accounts too, rather than disclose
		// which addresses are registered.
		nonce = "0000000000000000000000000000000000000000000000000000000000000000"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"identifier": req.Email,
			"pw_nonce":   nonce,
			"version":    s.keyParams.Version,
		},
	})
}

func (s *Server) handleSignIn(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid-request", err.Error())
		return
	}

	if !s.emailMatches(req.Email) || req.Password != s.serverPassword {
		writeError(w, http.StatusUnauthorized, "invalid-auth", "Invalid email or password")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"meta": map[string]any{},
		"data": map[string]any{
			"session":    s.newSessionPayload(),
			"key_params": s.keyParams,
			"user": map[string]any{
				"uuid":            "00000000-0000-0000-0000-00000000user",
				"email":           s.Email,
				"protocolVersion": s.keyParams.Version,
			},
		},
	})
}

func (s *Server) handleRefresh(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"meta": map[string]any{},
		"data": map[string]any{"session": s.newSessionPayload()},
	})
}

// handleRegister accepts a registration for the account the server was built
// for, and rejects anything else: the master key and items key are derived at
// construction and cannot be re-derived for a different password.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid-request", err.Error())
		return
	}

	if !s.emailMatches(req.Email) {
		writeError(w, http.StatusBadRequest, "invalid-registration",
			fmt.Sprintf("mock server only serves %s", s.Email))

		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"meta": map[string]any{},
		// token sits at the top level, which is where a registering client
		// reads it from.
		"token": "mock-registration-token",
		"data": map[string]any{
			"session":    s.newSessionPayload(),
			"key_params": s.keyParams,
			"user":       map[string]any{"uuid": "00000000-0000-0000-0000-00000000user", "email": s.Email},
		},
	})
}

// newSessionPayload issues a fresh pair of tokens. Each call rotates them, so
// a client that refreshes ends up with a token the previous one does not match.
func (s *Server) newSessionPayload() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tokenSeq++
	now := time.Now().UTC()

	return map[string]any{
		"access_token":       fmt.Sprintf("%s%d", accessTokenPrefix, s.tokenSeq),
		"refresh_token":      fmt.Sprintf("mock-refresh-%d", s.tokenSeq),
		"access_expiration":  now.Add(24 * time.Hour).UnixMilli(),
		"refresh_expiration": now.Add(30 * 24 * time.Hour).UnixMilli(),
		"readonly_access":    false,
	}
}

type syncRequest struct {
	Items       []Item `json:"items"`
	SyncToken   string `json:"sync_token"`
	CursorToken string `json:"cursor_token"`
	Limit       int    `json:"limit"`
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer "+accessTokenPrefix) {
		writeError(w, http.StatusUnauthorized, "invalid-auth", "Invalid login credentials")
		return
	}

	var req syncRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid-request", err.Error())
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.syncs++

	if s.expired {
		// One rejection, then the refreshed token is accepted.
		s.expired = false

		writeError(w, http.StatusUnauthorized, "expired-token", "Access token has expired")

		return
	}

	if len(s.failSync) > 0 {
		status := s.failSync[0]
		s.failSync = s.failSync[1:]

		writeError(w, status, "server-error", http.StatusText(status))

		return
	}

	saved, conflicts, err := s.applyWrites(req.Items)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid-request", err.Error())
		return
	}

	pushed := make(map[string]bool, len(req.Items))
	for _, in := range req.Items {
		pushed[in.UUID] = true
	}

	retrieved, cursor := s.itemsSince(readToken(req.CursorToken, req.SyncToken), pushed, req.Limit)

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"retrieved_items": retrieved,
			"saved_items":     saved,
			"unsaved":         []Item{},
			"conflicts":       conflicts,
			"sync_token":      strconv.FormatInt(s.revision, 10),
			"cursor_token":    cursor,
		},
	})
}

// applyWrites stores the items a client pushed, holding back any the caller has
// marked as conflicting.
func (s *Server) applyWrites(in []Item) (saved, conflicts []any, err error) {
	saved = make([]any, 0, len(in))
	conflicts = make([]any, 0)

	for _, item := range in {
		if item.UUID == "" {
			return nil, nil, fmt.Errorf("item of type %q has no uuid", item.ContentType)
		}

		if conflictType, ok := s.conflicts[item.UUID]; ok {
			existing := item
			if stored, found := s.stored[item.UUID]; found {
				existing = stored.item
			}

			conflicts = append(conflicts, map[string]any{
				"type":         conflictType,
				"server_item":  existing,
				"unsaved_item": item,
			})

			continue
		}

		if item.Deleted {
			// A deleted item keeps its identity and loses its payload, which is
			// what the API returns.
			item.Content = ""
			item.EncItemKey = ""
			item.ItemsKeyID = ""
		}

		saved = append(saved, s.put(item))
	}

	return saved, conflicts, nil
}

// itemsSince returns the items written after the given revision, excluding the
// ones the client just pushed, along with a cursor token when the response had
// to be capped.
func (s *Server) itemsSince(since int64, pushed map[string]bool, limit int) ([]Item, string) {
	var out []Item

	for _, si := range s.stored {
		if si.revision > since && !pushed[si.item.UUID] {
			out = append(out, si.item)
		}
	}

	// Sending them in revision order lets the cursor pick up where it left off.
	sort.Slice(out, func(i, j int) bool {
		return s.stored[out[i].UUID].revision < s.stored[out[j].UUID].revision
	})

	max := s.pageSize
	if limit > 0 && (max == 0 || limit < max) {
		max = limit
	}

	if max > 0 && len(out) > max {
		out = out[:max]

		// The cursor is the revision of the last item sent, so the next request
		// resumes from there.
		return out, strconv.FormatInt(s.stored[out[len(out)-1].UUID].revision, 10)
	}

	return out, ""
}

// readToken prefers the cursor token, which a client sends while paging
// through a response, and falls back to the sync token.
func readToken(cursorToken, syncToken string) int64 {
	for _, t := range []string{cursorToken, syncToken} {
		if t == "" || t == "null" {
			continue
		}

		if v, err := strconv.ParseInt(t, 10, 64); err == nil {
			return v
		}
	}

	return 0
}
