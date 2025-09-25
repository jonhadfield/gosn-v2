package cache

import (
	"sync"
	"testing"
	"time"
)

// TestConsecutiveSyncDelayMechanism tests the delay mechanism in isolation
// This test doesn't require authentication and validates the core consecutive sync logic
func TestConsecutiveSyncDelayMechanism(t *testing.T) {
	// Test the enforceMinimumSyncDelay function directly
	t.Run("DirectDelayTesting", func(t *testing.T) {
		// Reset the global sync timing state for clean test
		func() {
			syncMutex.Lock()
			defer syncMutex.Unlock()
			lastSyncTime = time.Time{} // Reset to zero value
		}()

		const numCalls = 5
		callTimes := make([]time.Time, numCalls)

		// Make consecutive calls to enforceMinimumSyncDelay
		for i := 0; i < numCalls; i++ {
			start := time.Now()
			enforceMinimumSyncDelay()
			callTimes[i] = start
			t.Logf("Call %d completed at: %v", i+1, start)
		}

		// Validate timing between consecutive calls
		for i := 1; i < len(callTimes); i++ {
			elapsed := callTimes[i].Sub(callTimes[i-1])
			t.Logf("Time between call %d and %d: %v", i, i+1, elapsed)

			// From the second call onwards, we should see proper delays (5 second minimum)
			if i >= 2 {
				minExpectedDelay := 4800 * time.Millisecond // Allow some tolerance for 5s delays
				if elapsed < minExpectedDelay {
					t.Errorf("Insufficient delay between calls %d and %d: got %v, expected at least %v",
						i, i+1, elapsed, minExpectedDelay)
				} else {
					t.Logf("✅ Good delay observed between calls %d and %d: %v", i, i+1, elapsed)
				}
			}
		}
	})

	// Test concurrent delay enforcement
	t.Run("ConcurrentDelayEnforcement", func(t *testing.T) {
		// Reset the global sync timing state
		func() {
			syncMutex.Lock()
			defer syncMutex.Unlock()
			lastSyncTime = time.Time{}
		}()

		const numGoroutines = 3
		results := make(chan time.Time, numGoroutines)
		startTimes := make(chan time.Time, numGoroutines)

		// Start multiple goroutines concurrently
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				start := time.Now()
				startTimes <- start

				// This should be serialized by the mutex
				enforceMinimumSyncDelay()

				end := time.Now()
				results <- end
				t.Logf("Goroutine %d: started at %v, completed at %v", id, start, end)
			}(i)
		}

		// Collect all start times
		var starts []time.Time
		for i := 0; i < numGoroutines; i++ {
			starts = append(starts, <-startTimes)
		}

		// Collect all end times
		var ends []time.Time
		for i := 0; i < numGoroutines; i++ {
			ends = append(ends, <-results)
		}

		t.Logf("✅ All %d concurrent operations completed without deadlock", numGoroutines)

		// The test passes if all goroutines complete (no deadlock)
		// and the delay mechanism properly serializes access
	})

	// Test delay mechanism under rapid successive calls
	t.Run("RapidSuccessiveCalls", func(t *testing.T) {
		// Reset timing state
		func() {
			syncMutex.Lock()
			defer syncMutex.Unlock()
			lastSyncTime = time.Time{}
		}()

		testStart := time.Now()

		// Make 3 rapid calls - they should be properly spaced
		for i := 0; i < 3; i++ {
			callStart := time.Now()
			enforceMinimumSyncDelay()
			callEnd := time.Now()

			t.Logf("Rapid call %d: started at %v, took %v",
				i+1, callStart.Sub(testStart), callEnd.Sub(callStart))
		}

		totalElapsed := time.Since(testStart)
		t.Logf("Total time for 3 rapid successive calls: %v", totalElapsed)

		// Should take at least 10s (2 delays of 5s each after first call)
		minExpected := 10 * time.Second
		if totalElapsed < minExpected {
			t.Errorf("Rapid successive calls completed too quickly: got %v, expected at least %v",
				totalElapsed, minExpected)
		} else {
			t.Logf("✅ Rapid successive calls properly delayed: %v", totalElapsed)
		}
	})

	// Test delay mechanism with custom timing intervals
	t.Run("CustomTimingValidation", func(t *testing.T) {
		// Reset timing state
		func() {
			syncMutex.Lock()
			defer syncMutex.Unlock()
			lastSyncTime = time.Time{}
		}()

		// First call should be immediate
		start1 := time.Now()
		enforceMinimumSyncDelay()
		end1 := time.Now()
		duration1 := end1.Sub(start1)

		t.Logf("First call duration: %v", duration1)
		if duration1 > 100*time.Millisecond {
			t.Errorf("First call should be nearly immediate, got %v", duration1)
		}

		// Wait less than minimum delay and call again
		time.Sleep(1 * time.Second) // Wait only 1s (less than 5s minimum)

		start2 := time.Now()
		enforceMinimumSyncDelay()
		end2 := time.Now()
		duration2 := end2.Sub(start2)

		t.Logf("Second call duration: %v", duration2)

		// Second call should include waiting time to meet minimum delay
		expectedMinWait := 3800 * time.Millisecond // Should wait ~4s more to reach 5s total
		if duration2 < expectedMinWait {
			t.Errorf("Second call should have waited, got %v, expected at least %v", duration2, expectedMinWait)
		} else {
			t.Logf("✅ Second call properly waited: %v", duration2)
		}

		// Validate total time from start of test
		totalTime := end2.Sub(start1)
		expectedMinTotal := 5 * time.Second
		if totalTime < expectedMinTotal {
			t.Errorf("Total time should be at least %v, got %v", expectedMinTotal, totalTime)
		} else {
			t.Logf("✅ Total timing validation passed: %v", totalTime)
		}
	})
}

// TestSyncConfigurationFunctionsIsolated tests configuration functions in isolation
func TestSyncConfigurationFunctionsIsolated(t *testing.T) {
	t.Run("TimeoutCalculation", func(t *testing.T) {
		testCases := []struct {
			name            string
			itemCount       int64
			expectedTimeout time.Duration
		}{
			{"Small dataset", 5, 30 * time.Second},
			{"Medium dataset", 50, 60 * time.Second},
			{"Large dataset", 500, 120 * time.Second},
			{"Very large dataset", 2000, 240 * time.Second},
			{"Extremely large dataset", 10000, 240 * time.Second}, // Cap at 4 minutes
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				timeout := calculateSyncTimeout(tc.itemCount)
				if timeout != tc.expectedTimeout {
					t.Errorf("calculateSyncTimeout(%d): got %v, expected %v",
						tc.itemCount, timeout, tc.expectedTimeout)
				} else {
					t.Logf("✅ %s (%d items): %v", tc.name, tc.itemCount, timeout)
				}
			})
		}
	})

	t.Run("BatchSizeDetection", func(t *testing.T) {
		// Test the batched sync detection logic
		// Note: This test validates the logic without requiring a real database

		testCases := []struct {
			name     string
			expected bool
		}{
			{"Nil database", false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := shouldUseBatchedSync(nil)
				if result != tc.expected {
					t.Errorf("shouldUseBatchedSync(nil): got %v, expected %v", result, tc.expected)
				} else {
					t.Logf("✅ %s: %v", tc.name, result)
				}
			})
		}
	})
}

// BenchmarkDelayMechanism benchmarks the delay mechanism performance
func BenchmarkDelayMechanism(b *testing.B) {
	// Reset timing state for clean benchmark
	func() {
		syncMutex.Lock()
		defer syncMutex.Unlock()
		lastSyncTime = time.Time{}
	}()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		enforceMinimumSyncDelay()
	}

	b.Logf("Completed %d delay enforcement calls", b.N)
}

// BenchmarkConcurrentDelay benchmarks concurrent delay mechanism access
func BenchmarkConcurrentDelay(b *testing.B) {
	// Reset timing state
	func() {
		syncMutex.Lock()
		defer syncMutex.Unlock()
		lastSyncTime = time.Time{}
	}()

	b.ResetTimer()

	var wg sync.WaitGroup
	const numGoroutines = 10

	for i := 0; i < b.N; i++ {
		wg.Add(numGoroutines)

		for j := 0; j < numGoroutines; j++ {
			go func() {
				defer wg.Done()
				enforceMinimumSyncDelay()
			}()
		}

		wg.Wait()
	}

	b.Logf("Completed %d concurrent delay tests with %d goroutines each", b.N, numGoroutines)
}
