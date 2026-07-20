package store

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/N-Saipraveen/LagerKV/pkg/config"
	"go.uber.org/zap"
)

// Store represents a Bitcask-like append-only log key-value store
type Store struct {
	mu             sync.RWMutex
	basePath       string
	segmentManager *SegmentManager
	hashTable      *HashTable
	logger         *zap.Logger
	isMerging      atomic.Bool
	lockFile       *os.File      // Exclusive lock on database directory
	fsyncStrategy  string        // "always", "interval", "none"
	fsyncInterval  time.Duration // periodic interval
	closeChan      chan struct{} // channel to stop background threads
}

// New creates a new Bitcask-like store
func New(logger *zap.Logger, cfg *config.Config) (*Store, error) {
	dataDir := cfg.DataDir
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		logger.Warn("Could not create data directory", zap.String("path", dataDir), zap.Error(err))
	}

	// Acquire process lock file to prevent multiple instances from running
	lockFile, err := acquireLock(dataDir)
	if err != nil {
		return nil, err
	}

	store := &Store{
		basePath:      dataDir,
		hashTable:     NewHashTable(),
		logger:        logger,
		lockFile:      lockFile,
		fsyncStrategy: cfg.FsyncStrategy,
		fsyncInterval: cfg.FsyncInterval,
		closeChan:     make(chan struct{}),
	}

	// Initialize segment manager
	segmentManager, err := NewSegmentManager(dataDir)
	if err != nil {
		_ = releaseLock(lockFile)
		logger.Warn("Could not initialize segment manager", zap.String("path", dataDir), zap.Error(err))
		// Proceed without segment manager
		return store, nil
	}
	store.segmentManager = segmentManager

	// Load existing data from segments
	if err := store.loadFromSegments(); err != nil {
		_ = releaseLock(lockFile)
		logger.Error("Could not load data from segments", zap.String("path", dataDir), zap.Error(err))
		return nil, err
	}

	// Periodically trigger background merges at MergeInterval.
	if cfg.MergeInterval > 0 {
		go func() {
			ticker := time.NewTicker(cfg.MergeInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					logger.Info("Starting compaction...")
					if err := store.Merge(); err != nil {
						logger.Error("Compaction failed", zap.Error(err))
					} else {
						logger.Info("Compaction was successful")
					}
				case <-store.closeChan:
					return
				}
			}
		}()
	}

	// Periodically trigger background fsyncs at FsyncInterval.
	if cfg.FsyncStrategy == "interval" && cfg.FsyncInterval > 0 {
		go func() {
			ticker := time.NewTicker(cfg.FsyncInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if store.segmentManager != nil {
						if err := store.segmentManager.FlushAll(); err != nil {
							logger.Error("Compaction periodic fsync failed", zap.Error(err))
						}
					}
				case <-store.closeChan:
					return
				}
			}
		}()
	}

	return store, nil
}

// loadFromSegments loads all existing data from segment files into the HashTable
func (s *Store) loadFromSegments() error {
	if s.segmentManager == nil {
		s.logger.Error("Segment manager is not initialized; cannot load segments")
		return fmt.Errorf("segment manager is not initialized")
	}

	segmentIDs := s.segmentManager.GetSegmentIDs()

	for _, segmentID := range segmentIDs {
		segment, exists := s.segmentManager.GetSegment(segmentID)
		if !exists {
			continue
		}

		// Read all entries from the segment
		if err := s.loadSegmentIntoKeyDir(segment); err != nil {
			s.logger.Error("Failed to load segment", zap.Int("segmentID", segmentID), zap.Error(err))
			return fmt.Errorf("failed to load segment %d: %w", segmentID, err)
		}
	}

	return nil
}

// loadSegmentIntoKeyDir loads all entries from a segment into the HashTable
func (s *Store) loadSegmentIntoKeyDir(segment *Segment) error {
	pos := int64(0)
	segmentSize := segment.Size()
	now := uint32(time.Now().Unix())

	for pos < segmentSize {
		entry, err := segment.Read(pos)
		if err != nil {
			return fmt.Errorf("failed to read entry at position %d: %w", pos, err)
		}

		key := string(entry.Key)

		// Only add to HashTable if it's not a tombstone and not expired
		if !entry.IsTombstone() && (entry.Expiry == 0 || entry.Expiry > now) {
			s.hashTable.Put(key, segment.ID(), pos, entry.ValueSize, entry.Timestamp, entry.Expiry)
		} else {
			// Remove from HashTable if it's a tombstone or expired
			s.hashTable.Delete(key)
		}

		// Move to next entry
		pos += int64(entry.Size())
	}

	return nil
}

// Get retrieves a value by key
func (s *Store) Get(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.hashTable.Get(key)
	if !exists {
		return "", ErrKeyNotFound
	}

	// Check if entry has expired
	if entry.Expiry > 0 && entry.Expiry < uint32(time.Now().Unix()) {
		return "", ErrKeyNotFound
	}

	// Read the entry from the segment
	logEntry, err := s.segmentManager.Read(entry.FileID, entry.ValuePos)
	if err != nil {
		return "", fmt.Errorf("failed to read entry: %w", err)
	}

	return string(logEntry.Value), nil
}

// Set stores a key-value pair with an optional TTL
func (s *Store) Set(key, value string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Println("Setting key:", key, "Value:", value)

	if s.segmentManager == nil {
		return fmt.Errorf("store not properly initialized")
	}

	var expiry uint32
	if ttl > 0 {
		expiry = uint32(time.Now().Add(ttl).Unix())
	}

	// Create entry
	entry := &Entry{
		Timestamp: uint32(time.Now().Unix()),
		Expiry:    expiry,
		KeySize:   uint32(len(key)),
		ValueSize: uint32(len(value)),
		Key:       []byte(key),
		Value:     []byte(value),
	}

	// Append to active segment
	segmentID, offset, err := s.segmentManager.Append(entry)
	if err != nil {
		return fmt.Errorf("failed to append entry: %w", err)
	}

	// Update HashTable
	s.hashTable.Put(key, segmentID, offset, entry.ValueSize, entry.Timestamp, entry.Expiry)

	// Fsync strategy "always"
	if s.fsyncStrategy == "always" {
		_ = s.segmentManager.FlushAll()
	}

	return nil
}

// Delete removes a key (creates a tombstone entry)
func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.segmentManager == nil {
		return fmt.Errorf("store not properly initialized")
	}

	// Check if key exists
	entry, exists := s.hashTable.Get(key)
	if !exists {
		return ErrKeyNotFound
	}
	if entry.Expiry > 0 && entry.Expiry < uint32(time.Now().Unix()) {
		return ErrKeyNotFound
	}

	// Create tombstone entry
	tombstoneEntry := &Entry{
		Timestamp: uint32(time.Now().Unix()),
		Expiry:    0,
		KeySize:   uint32(len(key)),
		ValueSize: 0, // Zero value size indicates tombstone
		Key:       []byte(key),
		Value:     nil,
	}

	// Append tombstone to active segment
	_, _, err := s.segmentManager.Append(tombstoneEntry)
	if err != nil {
		return fmt.Errorf("failed to append tombstone: %w", err)
	}

	// Remove from HashTable
	s.hashTable.Delete(key)

	// Fsync strategy "always"
	if s.fsyncStrategy == "always" {
		_ = s.segmentManager.FlushAll()
	}

	return nil
}

// List returns all keys
func (s *Store) List() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := uint32(time.Now().Unix())
	keys := make([]string, 0)
	for _, key := range s.hashTable.List() {
		entry, exists := s.hashTable.Get(key)
		if exists && (entry.Expiry == 0 || entry.Expiry > now) {
			keys = append(keys, key)
		}
	}

	return keys, nil
}

type Stats struct {
	TotalKeys int
	TotalSize int64
	Segments  int
}

// Stats returns database statistics
func (s *Store) Stats() (Stats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Filter out expired keys from statistics
	now := uint32(time.Now().Unix())
	totalKeys := 0
	totalSize := int64(0)

	for _, key := range s.hashTable.List() {
		entry, exists := s.hashTable.Get(key)
		if exists && (entry.Expiry == 0 || entry.Expiry > now) {
			totalKeys++
			totalSize += int64(entry.ValueSize)
		}
	}

	// Count segments
	segmentCount := 0
	if s.segmentManager != nil {
		segmentCount = len(s.segmentManager.GetSegmentIDs())
	}

	return Stats{
		TotalKeys: totalKeys,
		TotalSize: totalSize,
		Segments:  segmentCount,
	}, nil
}

// Close closes the store and all its resources
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Close closeChan to signal background threads
	select {
	case <-s.closeChan:
		// already closed
	default:
		close(s.closeChan)
	}

	var lastErr error
	if s.segmentManager != nil {
		if err := s.segmentManager.Close(); err != nil {
			lastErr = err
		}
	}

	if s.lockFile != nil {
		if err := releaseLock(s.lockFile); err != nil {
			lastErr = err
		}
		s.lockFile = nil
	}

	return lastErr
}

// Merge compacts inactive segments by copying only live (non-tombstone) records.
func (s *Store) Merge() error {
	if s.isMerging.Load() {
		return ErrMergeInProgress
	}

	s.isMerging.Store(true)
	defer s.isMerging.Store(false)

	sm := s.segmentManager
	ids := sm.GetInactiveSegmentIDs()
	if len(ids) == 0 {
		s.logger.Info("No inactive segments to compact")
		return nil
	}

	s.logger.Info("Starting compaction", zap.Ints("segments", ids))

	tmpDir := filepath.Join(s.basePath, "merge_tmp")
	_ = os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("create tmp dir: %w", err)
	}

	mergeSM, err := NewSegmentManager(tmpDir)
	if err != nil {
		return err
	}

	mergeHT := NewHashTable()
	snap := s.hashTable.Clone() // snap for checking updated keys while compacting
	now := uint32(time.Now().Unix())

	for _, id := range ids {
		seg, ok := s.segmentManager.GetSegment(id)
		if !ok {
			continue
		}

		var pos int64
		size := seg.Size()
		for pos < size {
			se, err := seg.Read(pos)
			if err != nil {
				return fmt.Errorf("compaction failed seg=%d off=%d: %w", id, pos, err)
			}

			oldOff := pos
			pos += int64(se.Size()) // advance regardless of branch

			if se.IsTombstone() {
				continue
			}

			// If the record has expired, skip it during merge
			if se.Expiry > 0 && se.Expiry < now {
				continue
			}

			key := string(se.Key)
			he, ok := snap.Get(key)
			if !ok || he.FileID != id || he.ValuePos != oldOff {
				continue
			}

			newId, newOff, err := mergeSM.Append(se)
			if err != nil {
				return fmt.Errorf("failed to append entry: %w", err)
			}

			mergeHT.Put(key, newId, newOff, se.ValueSize, se.Timestamp, se.Expiry)
		}
	}

	// Ensure merged files are durable before swapping.
	mergeSM.FlushAll()

	// Short stop-the-world: move files, rebuild segment manager, commit index.
	s.mu.Lock()
	defer s.mu.Unlock()
	// Remove old segments
	for _, id := range ids {
		if err := s.segmentManager.DeleteSegment(id); err != nil {
			return fmt.Errorf("delete seg %d: %w", id, err)
		}
	}

	// Move merged files into base dir.
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		_, err := file.Info()
		if err != nil {
			return err
		}

		err = os.Rename(
			path.Join(tmpDir, file.Name()),
			path.Join(s.basePath, file.Name()),
		)
		if err != nil {
			return err
		}
	}

	// Merge segment managers
	s.segmentManager.Merge(mergeSM)
	// Merge hash tables
	s.hashTable.Merge(mergeHT, snap)

	return nil
}

// GetExpiry returns the absolute expiry Unix timestamp for a key, and whether it exists
func (s *Store) GetExpiry(key string) (uint32, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.hashTable.Get(key)
	if !exists {
		return 0, false
	}
	return entry.Expiry, true
}
