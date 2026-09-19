package items

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/session"
	"github.com/stretchr/testify/require"
)

// recordedSyncRequest is the part of a sync request body the pagination tests inspect.
type recordedSyncRequest struct {
	Items       EncryptedItems `json:"items"`
	Limit       int            `json:"limit"`
	SyncToken   string         `json:"sync_token"`
	CursorToken *string        `json:"cursor_token"`
}

type mockSyncResponse struct {
	status int
	data   syncResponseData
}

// mockSyncServer answers each sync request with the next response from the handler and
// records every request body.
type mockSyncServer struct {
	mu       sync.Mutex
	requests []recordedSyncRequest
	respond  func(n int, req recordedSyncRequest) mockSyncResponse
	server   *httptest.Server
}

func newMockSyncServer(t *testing.T, respond func(n int, req recordedSyncRequest) mockSyncResponse) *mockSyncServer {
	t.Helper()

	m := &mockSyncServer{respond: respond}
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var req recordedSyncRequest
		require.NoError(t, json.Unmarshal(body, &req))

		m.mu.Lock()
		n := len(m.requests)
		m.requests = append(m.requests, req)
		m.mu.Unlock()

		resp := m.respond(n, req)
		if resp.status != 0 && resp.status != http.StatusOK {
			w.WriteHeader(resp.status)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(syncResponse{Data: resp.data}))
	}))
	t.Cleanup(m.server.Close)

	return m
}

func (m *mockSyncServer) session() *session.Session {
	return &session.Session{
		Server:            m.server.URL,
		AccessToken:       "access",
		RefreshToken:      "refresh",
		MasterKey:         "master",
		AccessExpiration:  time.Now().Add(time.Hour).UnixMilli(),
		RefreshExpiration: time.Now().Add(24 * time.Hour).UnixMilli(),
		DefaultItemsKey:   session.SessionItemsKey{UUID: "ik", ItemsKey: "key"},
	}
}

// deletedNotes returns deleted notes, which need no encryption to pass validation.
func deletedNotes(prefix string, n int) EncryptedItems {
	var items EncryptedItems
	for i := range n {
		items = append(items, EncryptedItem{
			UUID:               fmt.Sprintf("%s-%d", prefix, i),
			ContentType:        common.SNItemTypeNote,
			Deleted:            true,
			UpdatedAtTimestamp: int64(1000 + i),
		})
	}

	return items
}

func TestSyncItemsViaAPISinglePushedItemSentOnce(t *testing.T) {
	m := newMockSyncServer(t, func(n int, req recordedSyncRequest) mockSyncResponse {
		data := syncResponseData{
			Items:     deletedNotes(fmt.Sprintf("page%d", n), 2),
			SyncToken: fmt.Sprintf("st%d", n),
		}
		if len(req.Items) > 0 {
			data.SavedItems = req.Items
		}
		if n < 2 {
			data.CursorToken = fmt.Sprintf("ct%d", n)
		}

		return mockSyncResponse{data: data}
	})

	out, err := syncItemsViaAPI(SyncInput{Session: m.session(), Items: deletedNotes("push", 1)})
	require.NoError(t, err)
	require.Len(t, m.requests, 3)
	require.Len(t, m.requests[0].Items, 1)
	require.Empty(t, m.requests[1].Items, "pushed item must not be re-sent on later pages")
	require.Empty(t, m.requests[2].Items, "pushed item must not be re-sent on later pages")
	require.Len(t, out.Data.SavedItems, 1)
	require.Len(t, out.Data.Items, 6)
	require.Equal(t, "st2", out.Data.SyncToken)
	require.Empty(t, out.Data.CursorToken)
}

func TestSyncItemsViaAPIPullOnlyPagesUseDefaultLimit(t *testing.T) {
	// large items shrink the push batch size; pages after the push must use the full limit
	push := deletedNotes("push", 1)
	push[0].Content = string(make([]byte, 20*1024))

	m := newMockSyncServer(t, func(n int, req recordedSyncRequest) mockSyncResponse {
		data := syncResponseData{SyncToken: fmt.Sprintf("st%d", n), SavedItems: req.Items}
		if n == 0 {
			data.CursorToken = "ct0"
		}

		return mockSyncResponse{data: data}
	})

	_, err := syncItemsViaAPI(SyncInput{Session: m.session(), Items: push})
	require.NoError(t, err)
	require.Len(t, m.requests, 2)
	require.Less(t, m.requests[0].Limit, common.PageSize)
	require.Equal(t, common.PageSize, m.requests[1].Limit)
}

func TestSyncItemsResumesAfterFailedPage(t *testing.T) {
	failed := false
	m := newMockSyncServer(t, func(n int, req recordedSyncRequest) mockSyncResponse {
		cursor := ""
		if req.CursorToken != nil {
			cursor = *req.CursorToken
		}

		switch cursor {
		case "":
			return mockSyncResponse{data: syncResponseData{Items: deletedNotes("page0", 3), SyncToken: "st0", CursorToken: "ct0"}}
		case "ct0":
			if !failed {
				failed = true

				return mockSyncResponse{status: http.StatusRequestEntityTooLarge}
			}

			return mockSyncResponse{data: syncResponseData{Items: deletedNotes("page1", 3), SyncToken: "st1"}}
		default:
			t.Fatalf("unexpected cursor %q", cursor)

			return mockSyncResponse{}
		}
	})

	so, err := syncItems(SyncInput{Session: m.session()})
	require.NoError(t, err)

	// first page, failed second page, retried second page: the first page is not fetched again
	require.Len(t, m.requests, 3)
	require.Equal(t, "st0", m.requests[2].SyncToken)
	require.NotNil(t, m.requests[2].CursorToken)
	require.Equal(t, "ct0", *m.requests[2].CursorToken)
	require.Less(t, m.requests[2].Limit, m.requests[1].Limit)
	require.Len(t, so.Items, 6)
	require.Equal(t, "st1", so.SyncToken)
}

func TestSyncConflictResyncUsesNewSyncToken(t *testing.T) {
	local := deletedNotes("note", 1)
	local[0].UpdatedAtTimestamp = 2000

	server := local[0]
	server.UpdatedAtTimestamp = 1000

	m := newMockSyncServer(t, func(n int, req recordedSyncRequest) mockSyncResponse {
		if n == 0 {
			return mockSyncResponse{data: syncResponseData{
				Items:     deletedNotes("existing", 3),
				SyncToken: "st0",
				Conflicts: ConflictedItems{{Type: ConflictTypeSync, ServerItem: server, UnsavedItem: local[0]}},
			}}
		}

		return mockSyncResponse{data: syncResponseData{SavedItems: req.Items, SyncToken: "st1"}}
	})

	so, err := Sync(SyncInput{Session: m.session(), Items: local})
	require.NoError(t, err)
	require.Len(t, m.requests, 2)
	require.Empty(t, m.requests[0].SyncToken)
	require.Equal(t, "st0", m.requests[1].SyncToken, "resync must continue from the first sync's token")
	require.Equal(t, "st1", so.SyncToken, "caller must receive the newest token")
}

func TestSyncServerWinsConflictIsNotPushedBack(t *testing.T) {
	local := deletedNotes("note", 1)
	server := local[0]
	server.Deleted = false
	server.Content = "server"

	m := newMockSyncServer(t, func(n int, req recordedSyncRequest) mockSyncResponse {
		return mockSyncResponse{data: syncResponseData{
			SyncToken: "st0",
			Conflicts: ConflictedItems{{Type: ConflictTypeReadOnly, ServerItem: server, UnsavedItem: local[0]}},
		}}
	})

	so, err := Sync(SyncInput{Session: m.session(), Items: local})
	require.NoError(t, err)
	require.Len(t, m.requests, 1, "server copy wins, so nothing should be re-sent")
	require.Empty(t, so.Conflicts)
	require.Len(t, so.Items, 1)
	require.Equal(t, "server", so.Items[0].Content)
}

func TestDeleteContentPushesWithSyncToken(t *testing.T) {
	notes := deletedNotes("note", 2)
	for i := range notes {
		notes[i].Deleted = false
		notes[i].ItemsKeyID = "" // ignored by validation, no decryption needed
	}

	m := newMockSyncServer(t, func(n int, req recordedSyncRequest) mockSyncResponse {
		if n == 0 {
			return mockSyncResponse{data: syncResponseData{Items: notes, SyncToken: "st0"}}
		}

		return mockSyncResponse{data: syncResponseData{SavedItems: req.Items, SyncToken: "st1"}}
	})

	deleted, err := DeleteContent(m.session(), false)
	require.NoError(t, err)
	require.Equal(t, 2, deleted)
	require.Len(t, m.requests, 2)
	require.Equal(t, "st0", m.requests[1].SyncToken, "deletions must not re-download the account")
}

func TestRateLimitDelay(t *testing.T) {
	require.Equal(t, time.Second, rateLimitDelay("", 0))
	require.Equal(t, 4*time.Second, rateLimitDelay("", 2))
	require.Equal(t, 8*time.Second, rateLimitDelay("", 4), "backoff is capped")
	require.Equal(t, 3*time.Second, rateLimitDelay("3", 4), "Retry-After is honoured")
	require.Equal(t, 8*time.Second, rateLimitDelay("120", 0), "Retry-After is capped")
}
