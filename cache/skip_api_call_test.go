package cache

import (
	"testing"
	"time"

	"github.com/jonhadfield/gosn-v2/common"
	"github.com/stretchr/testify/require"
)

func TestSkipAPICall(t *testing.T) {
	now := time.Now()

	testCases := []struct {
		name       string
		syncTokens []SyncToken
		wantSkip   bool
		wantAge    time.Duration
	}{
		{
			name:     "no sync token",
			wantSkip: false,
		},
		{
			name:       "recent sync token",
			syncTokens: []SyncToken{{SyncToken: "a", CreatedAt: now.Add(-time.Minute)}},
			wantSkip:   true,
			wantAge:    time.Minute,
		},
		{
			name:       "sync token at interval",
			syncTokens: []SyncToken{{SyncToken: "a", CreatedAt: now.Add(-common.MinSyncInterval)}},
			wantSkip:   false,
			wantAge:    common.MinSyncInterval,
		},
		{
			name:       "old sync token",
			syncTokens: []SyncToken{{SyncToken: "a", CreatedAt: now.Add(-time.Hour)}},
			wantSkip:   false,
			wantAge:    time.Hour,
		},
		{
			name: "multiple sync tokens",
			syncTokens: []SyncToken{
				{SyncToken: "a", CreatedAt: now.Add(-time.Minute)},
				{SyncToken: "b", CreatedAt: now.Add(-time.Minute)},
			},
			wantSkip: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			skip, age := skipAPICall(tc.syncTokens, now)
			require.Equal(t, tc.wantSkip, skip)
			require.Equal(t, tc.wantAge, age)
		})
	}
}
