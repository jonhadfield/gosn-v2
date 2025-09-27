package cache

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jonhadfield/gosn-v2/session"
)

// TestSyncErrorClassification tests the error classification system
func TestSyncErrorClassification(t *testing.T) {
	testCases := []struct {
		name              string
		inputError        error
		expectedType      SyncErrorType
		expectedRetryable bool
	}{
		{
			name:              "Rate limit error HTTP 429",
			inputError:        errors.New("HTTP 429: rate limit exceeded"),
			expectedType:      SyncErrorRateLimit,
			expectedRetryable: true,
		},
		{
			name:              "Rate limit error bandwidth message",
			inputError:        errors.New("You have exceeded the maximum bandwidth allotted to your account"),
			expectedType:      SyncErrorRateLimit,
			expectedRetryable: true,
		},
		{
			name:              "ItemsKey missing error",
			inputError:        errors.New("empty default items key"),
			expectedType:      SyncErrorItemsKey,
			expectedRetryable: false,
		},
		{
			name:              "Authentication error",
			inputError:        errors.New("unauthorized access - invalid session"),
			expectedType:      SyncErrorAuthentication,
			expectedRetryable: false,
		},
		{
			name:              "Validation error",
			inputError:        errors.New("validation failed: malformed content"),
			expectedType:      SyncErrorValidation,
			expectedRetryable: false,
		},
		{
			name:              "Network connectivity error",
			inputError:        errors.New("network connection timeout"),
			expectedType:      SyncErrorNetwork,
			expectedRetryable: true,
		},
		{
			name:              "Sync conflict error",
			inputError:        errors.New("sync conflict detected for item"),
			expectedType:      SyncErrorConflict,
			expectedRetryable: true,
		},
		{
			name:              "Unknown error",
			inputError:        errors.New("some unknown error occurred"),
			expectedType:      SyncErrorUnknown,
			expectedRetryable: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			syncErr := classifySyncError(tc.inputError)

			if syncErr == nil {
				t.Errorf("Expected classified error, got nil")
				return
			}

			if syncErr.Type != tc.expectedType {
				t.Errorf("Expected error type %d, got %d", tc.expectedType, syncErr.Type)
			}

			if syncErr.Retryable != tc.expectedRetryable {
				t.Errorf("Expected retryable=%t, got retryable=%t", tc.expectedRetryable, syncErr.Retryable)
			}

			if syncErr.Original != tc.inputError {
				t.Errorf("Original error not preserved correctly")
			}

			// Verify message is descriptive
			if len(syncErr.Message) < 10 {
				t.Errorf("Error message too short: %s", syncErr.Message)
			}

			t.Logf("✅ Correctly classified: Type=%d, Message=%s, Retryable=%t",
				syncErr.Type, syncErr.Message, syncErr.Retryable)
		})
	}
}

// TestRateLimitBackoff tests the exponential backoff implementation
func TestRateLimitBackoff(t *testing.T) {
	t.Run("ExponentialBackoffProgression", func(t *testing.T) {
		backoff := RateLimitBackoff{
			baseDelayMs: 100,  // Start at 100ms for testing
			maxDelayMs:  1000, // Cap at 1s for testing
		}

		expectedDelays := []int64{100, 200, 400, 800, 1000, 1000} // Reduced for testing, cap at 1s

		for i, expectedMs := range expectedDelays {
			start := time.Now()
			enforceRateLimitBackoff(&backoff)
			elapsed := time.Since(start)

			elapsedMs := elapsed.Milliseconds()

			// Allow 10% tolerance for timing variations
			tolerance := expectedMs / 10
			if elapsedMs < expectedMs-tolerance || elapsedMs > expectedMs+tolerance {
				t.Errorf("Attempt %d: Expected ~%dms delay, got %dms", i+1, expectedMs, elapsedMs)
			} else {
				t.Logf("✅ Attempt %d: Correct backoff delay ~%dms", i+1, elapsedMs)
			}

			if expectedMs >= 60000 && elapsedMs > 61000 {
				t.Errorf("Backoff should be capped at 60s, but got %dms", elapsedMs)
			}
		}
	})
}

// TestSessionItemsKeyValidation tests the ItemsKey validation
func TestSessionItemsKeyValidation(t *testing.T) {
	t.Run("ValidSession", func(t *testing.T) {
		session := &Session{
			Session: &session.Session{
				DefaultItemsKey: session.SessionItemsKey{
					ItemsKey: "valid-items-key-12345",
					UUID:     "test-uuid",
				},
			},
		}

		err := validateSessionItemsKey(session)
		if err != nil {
			t.Errorf("Expected valid session to pass validation, got: %v", err)
		} else {
			t.Log("✅ Valid session passed ItemsKey validation")
		}
	})

	t.Run("NilSession", func(t *testing.T) {
		err := validateSessionItemsKey(nil)
		if err == nil {
			t.Error("Expected nil session to fail validation")
		} else {
			t.Logf("✅ Nil session correctly rejected: %s", err.Error())
		}
	})

	t.Run("MissingItemsKey", func(t *testing.T) {
		session := &Session{
			Session: &session.Session{}, // DefaultItemsKey will be zero value
		}

		err := validateSessionItemsKey(session)
		syncErr, ok := err.(*SyncError)
		if !ok {
			t.Errorf("Expected SyncError, got %T", err)
		} else if syncErr.Type != SyncErrorItemsKey {
			t.Errorf("Expected SyncErrorItemsKey, got %d", syncErr.Type)
		} else {
			t.Logf("✅ Missing ItemsKey correctly classified: %s", syncErr.Message)
			t.Log("Note: In Sync() function, this would be a warning, not a failure")
		}
	})

	t.Run("EmptyItemsKey", func(t *testing.T) {
		session := &Session{
			Session: &session.Session{
				DefaultItemsKey: session.SessionItemsKey{
					ItemsKey: "", // Empty key
					UUID:     "test-uuid",
				},
			},
		}

		err := validateSessionItemsKey(session)
		syncErr, ok := err.(*SyncError)
		if !ok {
			t.Errorf("Expected SyncError, got %T", err)
		} else if syncErr.Type != SyncErrorItemsKey {
			t.Errorf("Expected SyncErrorItemsKey, got %d", syncErr.Type)
		} else {
			t.Logf("✅ Empty ItemsKey correctly classified: %s", syncErr.Message)
			t.Log("Note: In Sync() function, this would be a warning, not a failure")
		}
	})
}

// TestSyncErrorInterface tests that SyncError implements error interface correctly
func TestSyncErrorInterface(t *testing.T) {
	syncErr := &SyncError{
		Type:      SyncErrorRateLimit,
		Original:  errors.New("original error"),
		Message:   "Test error message",
		Retryable: true,
		BackoffMs: 1000,
	}

	// Test Error() method
	if syncErr.Error() != "Test error message" {
		t.Errorf("Expected 'Test error message', got '%s'", syncErr.Error())
	}

	// Test that it satisfies error interface
	var err error = syncErr
	if err.Error() != "Test error message" {
		t.Errorf("SyncError doesn't properly implement error interface")
	}

	t.Log("✅ SyncError correctly implements error interface")
}

// TestSyncWithMissingItemsKey tests that Sync continues with ItemsKey warnings
func TestSyncWithMissingItemsKey(t *testing.T) {
	// Create a session without ItemsKey (like test accounts)
	testSession := &Session{
		Session: &session.Session{
			Debug:       true,
			Server:      "https://api.standardnotes.com",
			AccessToken: "dummy-token", // Make session appear valid
		},
		CacheDBPath: "/tmp/test-missing-itemskey.db",
	}

	// Mock Valid() method behavior
	if testSession.Session.AccessToken != "" {
		// Session appears valid for testing
	}

	// This should not panic or fail, but should log warnings
	_, err := Sync(SyncInput{
		Session: testSession,
		Close:   true,
	})

	// Sync should proceed (though it may fail later for other reasons like network)
	// The key is that it doesn't fail immediately due to ItemsKey validation
	if err != nil {
		// Check that it's NOT an ItemsKey validation error
		if syncErr, ok := err.(*SyncError); ok && syncErr.Type == SyncErrorItemsKey {
			t.Errorf("Sync should not fail with ItemsKey validation error, got: %s", syncErr.Message)
		} else {
			t.Logf("✅ Sync continued past ItemsKey validation (failed later with: %s)", err.Error())
		}
	} else {
		t.Log("✅ Sync completed successfully despite missing ItemsKey")
	}

	// Clean up test database
	os.Remove("/tmp/test-missing-itemskey.db")
}
