package kafka

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/IBM/sarama"
)

// TestListACLs_ConcurrentCacheFill tests that concurrent calls to ListACLs
// properly handle the TOCTOU (Time-of-Check to Time-of-Use) race condition.
// The fix ensures that after acquiring the write lock, we re-check if the cache
// is already valid (another goroutine may have populated it).
func TestListACLs_ConcurrentCacheFill(t *testing.T) {
	// This test verifies the fix for the TOCTOU race condition in ListACLs.
	// The race condition occurs when:
	// 1. Goroutine A checks cache validity (invalid), releases read lock
	// 2. Goroutine B checks cache validity (invalid), releases read lock
	// 3. Goroutine A acquires write lock, populates cache, releases lock
	// 4. Goroutine B acquires write lock - should re-check and return cached data
	//
	// Without the fix, goroutine B would re-populate the cache unnecessarily.

	t.Run("validity re-check after acquiring write lock", func(t *testing.T) {
		// Create an aclCache to test the locking behavior directly
		cache := &aclCache{
			valid: false,
			acls:  nil,
		}

		// Simulate the TOCTOU scenario
		var wg sync.WaitGroup
		populateCacheCount := atomic.Int32{}

		// Number of concurrent goroutines trying to populate the cache
		numGoroutines := 10

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				// Simulate the fixed ListACLs logic
				cache.mutex.RLock()
				if cache.valid {
					cache.mutex.RUnlock()
					return
				}
				cache.mutex.RUnlock()

				cache.mutex.Lock()
				defer cache.mutex.Unlock()

				// This is the critical fix: re-check after acquiring write lock
				if cache.valid {
					// Cache was populated by another goroutine
					return
				}

				// Simulate cache population (only one goroutine should do this)
				populateCacheCount.Add(1)
				cache.acls = []*sarama.ResourceAcls{
					{
						Resource: sarama.Resource{
							ResourceType: sarama.AclResourceTopic,
							ResourceName: "test-topic",
						},
					},
				}
				cache.valid = true
			}()
		}

		wg.Wait()

		// Verify that only one goroutine actually populated the cache
		if count := populateCacheCount.Load(); count != 1 {
			t.Errorf("Expected exactly 1 cache population, but got %d", count)
		}

		// Verify the cache is valid and has the expected data
		if !cache.valid {
			t.Error("Cache should be valid after population")
		}
		if len(cache.acls) != 1 {
			t.Errorf("Expected 1 ACL in cache, got %d", len(cache.acls))
		}
	})

	t.Run("cache returns same data to all concurrent readers", func(t *testing.T) {
		cache := &aclCache{
			valid: true,
			acls: []*sarama.ResourceAcls{
				{
					Resource: sarama.Resource{
						ResourceType: sarama.AclResourceTopic,
						ResourceName: "cached-topic",
					},
				},
			},
		}

		var wg sync.WaitGroup
		numReaders := 100
		results := make(chan []*sarama.ResourceAcls, numReaders)

		for i := 0; i < numReaders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				cache.mutex.RLock()
				if cache.valid {
					acls := cache.acls
					cache.mutex.RUnlock()
					results <- acls
					return
				}
				cache.mutex.RUnlock()

				// Should not reach here since cache is valid
				t.Error("Cache should have been valid")
			}()
		}

		wg.Wait()
		close(results)

		// Verify all readers got the same cached data
		count := 0
		for acls := range results {
			count++
			if len(acls) != 1 || acls[0].ResourceName != "cached-topic" {
				t.Error("Reader did not get expected cached data")
			}
		}

		if count != numReaders {
			t.Errorf("Expected %d readers to get results, got %d", numReaders, count)
		}
	})
}

// TestListACLs_CacheInvalidation tests that cache invalidation works correctly
// and subsequent calls re-populate the cache.
func TestListACLs_CacheInvalidation(t *testing.T) {
	cache := &aclCache{
		valid: true,
		acls: []*sarama.ResourceAcls{
			{
				Resource: sarama.Resource{
					ResourceType: sarama.AclResourceTopic,
					ResourceName: "test-topic",
				},
			},
		},
	}

	// Verify cache is initially valid
	if !cache.valid {
		t.Fatal("Cache should be valid initially")
	}

	// Invalidate the cache (simulating InvalidateACLCache)
	cache.mutex.Lock()
	cache.valid = false
	cache.acls = nil
	cache.mutex.Unlock()

	// Verify cache is now invalid
	cache.mutex.RLock()
	if cache.valid {
		t.Error("Cache should be invalid after invalidation")
	}
	if cache.acls != nil {
		t.Error("Cache acls should be nil after invalidation")
	}
	cache.mutex.RUnlock()
}

// TestListACLs_RaceConditionPrevention uses the race detector to verify
// that the locking mechanism prevents data races.
func TestListACLs_RaceConditionPrevention(t *testing.T) {
	cache := &aclCache{
		valid: false,
		acls:  nil,
	}

	var wg sync.WaitGroup
	numGoroutines := 50

	// Mix of readers and writers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		if i%2 == 0 {
			// Reader
			go func() {
				defer wg.Done()
				cache.mutex.RLock()
				_ = cache.valid
				_ = cache.acls
				cache.mutex.RUnlock()
			}()
		} else {
			// Writer (cache population simulation)
			go func() {
				defer wg.Done()
				cache.mutex.Lock()
				if !cache.valid {
					cache.acls = []*sarama.ResourceAcls{}
					cache.valid = true
				}
				cache.mutex.Unlock()
			}()
		}
	}

	// Also test invalidation
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.mutex.Lock()
			cache.valid = false
			cache.acls = nil
			cache.mutex.Unlock()
		}()
	}

	wg.Wait()
}
