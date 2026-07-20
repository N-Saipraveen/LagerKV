package store

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/N-Saipraveen/LagerKV/pkg/config"
	"go.uber.org/zap/zaptest"
)

func setupStoreIntegration(t *testing.T) (*Store, string) {
	logger := zaptest.NewLogger(t)
	tempDir, err := os.MkdirTemp("", "store_int_test")
	require.NoError(t, err)

	cfg := &config.Config{
		DataDir:       tempDir,
		FsyncStrategy: "none",
	}

	// We use store.New so it initializes closeChan and handles locking.
	store, err := New(logger, cfg)
	require.NoError(t, err)

	return store, tempDir
}

func TestStore_SetGetIntegration(t *testing.T) {
	t.Parallel()
	store, tempDir := setupStoreIntegration(t)
	defer os.RemoveAll(tempDir)
	defer store.Close()

	key := "user_id"
	value := "12345"

	err := store.Set(key, value, 0)
	assert.NoError(t, err, "Set should succeed")

	result, err := store.Get(key)
	assert.NoError(t, err, "Get should succeed")
	assert.Equal(t, value, result, "Retrieved value must match set value")
}

func TestStore_DeleteIntegration(t *testing.T) {
	t.Parallel()
	store, tempDir := setupStoreIntegration(t)
	defer os.RemoveAll(tempDir)
	defer store.Close()

	key := "old_data"
	value := "to_be_deleted"

	store.Set(key, value, 0)
	_, err := store.Get(key)
	require.NoError(t, err)

	err = store.Delete(key)
	assert.NoError(t, err, "Delete should succeed")

	_, err = store.Get(key)
	assert.ErrorIs(t, err, ErrKeyNotFound, "Get after Delete should return ErrKeyNotFound")

	keys, _ := store.List()
	found := false
	for _, k := range keys {
		if k == key {
			found = true
			break
		}
	}
	assert.False(t, found, "Deleted key should not appear in List()")
}

func TestStore_Set_Error(t *testing.T) {
	t.Parallel()
	store, tempDir := setupStoreIntegration(t)
	defer os.RemoveAll(tempDir)
	
	// Close it and set segment manager to nil to trigger initialization errors
	store.Close()
	store.segmentManager = nil

	err := store.Set("foo", "bar", 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "store not properly initialized")
}

func TestStore_Get_KeyNotFound(t *testing.T) {
	t.Parallel()
	store, tempDir := setupStoreIntegration(t)
	defer os.RemoveAll(tempDir)
	defer store.Close()

	_, err := store.Get("unknown_key")
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

func TestStore_Delete_KeyNotFound(t *testing.T) {
	t.Parallel()
	store, tempDir := setupStoreIntegration(t)
	defer os.RemoveAll(tempDir)
	defer store.Close()

	err := store.Delete("nonexistent")
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

func TestStore_New_MkdirAllFail(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.Config{DataDir: "/root/invalid_dir"}
	_, err := New(logger, cfg)
	assert.Error(t, err, "Should fail because locking or directory creation fails")
}

func TestStore_LoadSegment_Tombstone(t *testing.T) {
	t.Parallel()
	store, tempDir := setupStoreIntegration(t)
	defer os.RemoveAll(tempDir)

	err := store.Set("foo", "bar", 0)
	assert.NoError(t, err)

	err = store.Delete("foo")
	assert.NoError(t, err)

	// Close the original store so that it releases the flock
	store.Close()

	reloadedStore, err := New(store.logger, &config.Config{DataDir: tempDir})
	assert.NoError(t, err)
	defer reloadedStore.Close()

	_, err = reloadedStore.Get("foo")
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

func TestStore_Stats_NoSegmentManager(t *testing.T) {
	store := &Store{
		hashTable:      NewHashTable(),
		segmentManager: nil,
	}

	stats, err := store.Stats()
	assert.NoError(t, err)
	assert.Equal(t, 0, stats.Segments)
}

func TestStore_LoadFromSegments_WithValidData(t *testing.T) {
	t.Parallel()
	store, tempDir := setupStoreIntegration(t)
	defer os.RemoveAll(tempDir)

	err := store.Set("hello", "world", 0)
	require.NoError(t, err)

	// Close to release flock
	store.Close()

	newStore, err := New(store.logger, &config.Config{DataDir: tempDir})
	require.NoError(t, err)
	defer newStore.Close()

	val, err := newStore.Get("hello")
	assert.NoError(t, err)
	assert.Equal(t, "world", val)
}

func TestStore_Close_NoSegmentManager(t *testing.T) {
	store := &Store{
		hashTable:      NewHashTable(),
		segmentManager: nil,
		closeChan:      make(chan struct{}),
	}

	err := store.Close()
	assert.NoError(t, err, "Close with nil segmentManager should not fail")
}

// TTL EXPIRE TEST
func TestStore_TTL(t *testing.T) {
	t.Parallel()
	store, tempDir := setupStoreIntegration(t)
	defer os.RemoveAll(tempDir)
	defer store.Close()

	// Set key with 1s TTL
	err := store.Set("expires_soon", "quick_value", 1*time.Second)
	assert.NoError(t, err)

	// Immediate Get should work
	val, err := store.Get("expires_soon")
	assert.NoError(t, err)
	assert.Equal(t, "quick_value", val)

	// Wait 2s to guarantee expiration (Unix time is second-precision)
	time.Sleep(2 * time.Second)

	// Get should now return ErrKeyNotFound
	_, err = store.Get("expires_soon")
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

// TTL COMPACTION / MERGE TEST
func TestStore_TTL_Compaction(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	tempDir, err := os.MkdirTemp("", "ttl_compaction_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Use tiny max segment size to force rotation and generate multiple segments
	cfg := &config.Config{
		DataDir:       tempDir,
		FsyncStrategy: "none",
	}

	store, err := New(logger, cfg)
	require.NoError(t, err)

	// Set active segment size/entries limit very small to simulate multiple files
	store.segmentManager.segments[store.segmentManager.activeID].maxSize = 100
	store.segmentManager.segments[store.segmentManager.activeID].maxEntries = 1

	// Set expired key (1s TTL)
	err = store.Set("key1", "val1", 1*time.Second)
	assert.NoError(t, err)

	// Set non-expired key
	err = store.Set("key2", "val2", 10*time.Hour)
	assert.NoError(t, err)

	// Force rotation by writing more keys
	err = store.Set("key3", "val3", 0)
	assert.NoError(t, err)

	// Wait 2s to let key1 expire
	time.Sleep(2 * time.Second)

	// Run compaction
	err = store.Merge()
	assert.NoError(t, err)

	// Verify key1 is not found
	_, err = store.Get("key1")
	assert.ErrorIs(t, err, ErrKeyNotFound)

	// Verify key2 and key3 are still retrievable
	val2, err := store.Get("key2")
	assert.NoError(t, err)
	assert.Equal(t, "val2", val2)

	val3, err := store.Get("key3")
	assert.NoError(t, err)
	assert.Equal(t, "val3", val3)

	store.Close()
}

// PROCESS LOCK COLLISION TEST
func TestStore_ProcessLock(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	tempDir, err := os.MkdirTemp("", "lock_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{DataDir: tempDir}

	// Open first store instance (should succeed)
	store1, err := New(logger, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, store1)
	defer store1.Close()

	// Open second store instance on same directory (should fail with ErrDatabaseLocked)
	store2, err := New(logger, cfg)
	assert.ErrorIs(t, err, ErrDatabaseLocked)
	assert.Nil(t, store2)
}

// CHECKSUM CORRUPTION TEST
func TestStore_ChecksumCorruption(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	tempDir, err := os.MkdirTemp("", "checksum_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{DataDir: tempDir}
	store, err := New(logger, cfg)
	require.NoError(t, err)

	err = store.Set("corrupt_me", "vulnerable_data", 0)
	require.NoError(t, err)

	// Read to ensure it works
	val, err := store.Get("corrupt_me")
	assert.NoError(t, err)
	assert.Equal(t, "vulnerable_data", val)

	// Retrieve segment file path and corrupt it
	activeSeg, err := store.segmentManager.GetActiveSegment()
	require.NoError(t, err)
	path := activeSeg.Path()

	// Close the store first to release file descriptors
	store.Close()

	// Corrupt file by opening and changing a byte
	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	require.NoError(t, err)
	defer file.Close()

	stat, err := file.Stat()
	require.NoError(t, err)

	// Write garbage byte near the end (the value portion)
	_, err = file.WriteAt([]byte{0xFF}, stat.Size()-5)
	require.NoError(t, err)

	// Reopen store (should fail due to checksum mismatch on load)
	store2, err := New(logger, cfg)
	assert.ErrorIs(t, err, ErrChecksumMismatch)
	assert.Nil(t, store2)
}
