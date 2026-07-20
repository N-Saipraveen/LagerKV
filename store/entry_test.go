package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEntry_IsTombstone(t *testing.T) {
	t.Parallel()

	// Case 1: Regular entry (ValueSize > 0)
	activeEntry := &Entry{
		KeySize:   5,
		ValueSize: 10,
	}
	assert.False(t, activeEntry.IsTombstone(), "Active entry should not be a tombstone")

	// Case 2: Tombstone entry (ValueSize == 0)
	tombstoneEntry := &Entry{
		KeySize:   5,
		ValueSize: 0,
	}
	assert.True(t, tombstoneEntry.IsTombstone(), "Entry with ValueSize 0 should be a tombstone")
}

func TestEntry_TombstoneEntry(t *testing.T) {
	t.Parallel()

	originalKey := []byte("testkey")
	originalValue := []byte("testvalue")

	originalEntry := &Entry{
		Timestamp: uint32(time.Now().Unix()) - 100, // Old timestamp
		Expiry:    12345,
		KeySize:   uint32(len(originalKey)),
		ValueSize: uint32(len(originalValue)),
		Key:       originalKey,
		Value:     originalValue,
	}

	tombstone := originalEntry.TombstoneEntry()

	// 1. Check Tombstone properties
	assert.True(t, tombstone.IsTombstone(), "Generated entry must be a tombstone")
	assert.Equal(t, uint32(0), tombstone.ValueSize, "Tombstone ValueSize must be 0")
	assert.Nil(t, tombstone.Value, "Tombstone Value must be nil")

	// 2. Check Key and Size retention
	assert.Equal(t, originalEntry.Key, tombstone.Key, "Tombstone must retain the original Key")
	assert.Equal(t, originalEntry.KeySize, tombstone.KeySize, "Tombstone must retain the original KeySize")

	// 3. Check Timestamp update (must be greater than the original)
	assert.GreaterOrEqual(t, tombstone.Timestamp, originalEntry.Timestamp, "Tombstone timestamp must be updated")

	// 4. Tombstone has no expiry
	assert.Equal(t, uint32(0), tombstone.Expiry, "Tombstone expiry should be 0")
}

func TestEntry_Size(t *testing.T) {
	t.Parallel()

	// Fixed header size: Checksum (4) + Timestamp (4) + Expiry (4) + KeySize (4) + ValueSize (4) = 20 bytes
	const headerSize = 20

	t.Run("Zero Size", func(t *testing.T) {
		entry := &Entry{
			KeySize:   0,
			ValueSize: 0,
		}
		assert.Equal(t, headerSize, entry.Size(), "Size should be 20 bytes for zero key/value")
	})

	t.Run("Standard Entry", func(t *testing.T) {
		keyLen := 5
		valLen := 10
		entry := &Entry{
			KeySize:   uint32(keyLen),
			ValueSize: uint32(valLen),
		}
		expectedSize := headerSize + keyLen + valLen
		assert.Equal(t, expectedSize, entry.Size(), "Size should be header + key length + value length")
	})

	t.Run("Tombstone Size", func(t *testing.T) {
		keyLen := 7
		entry := &Entry{
			KeySize:   uint32(keyLen),
			ValueSize: 0,
		}
		expectedSize := headerSize + keyLen // Value length is 0
		assert.Equal(t, expectedSize, entry.Size(), "Size should be header + key length for tombstone")
	})
}

func TestEntry_SerializeDeserialize(t *testing.T) {
	t.Parallel()

	testTime := uint32(time.Now().Unix())
	expiryTime := testTime + 3600
	key := []byte("mykey")
	value := []byte("myvalue")

	// 1. Standard Active Entry
	t.Run("Active Entry", func(t *testing.T) {
		original := &Entry{
			Timestamp: testTime,
			Expiry:    expiryTime,
			KeySize:   uint32(len(key)),
			ValueSize: uint32(len(value)),
			Key:       key,
			Value:     value,
		}

		serializedData := original.Serialize()
		deserialized, err := DeserializeEntry(serializedData)

		assert.NoError(t, err)
		assert.Equal(t, original.Size(), len(serializedData), "Serialized data length should match Size()")
		assert.Equal(t, original.Timestamp, deserialized.Timestamp)
		assert.Equal(t, original.Expiry, deserialized.Expiry)
		assert.Equal(t, original.KeySize, deserialized.KeySize)
		assert.Equal(t, original.ValueSize, deserialized.ValueSize)
		assert.Equal(t, original.Key, deserialized.Key)
		assert.Equal(t, original.Value, deserialized.Value)
		assert.NotZero(t, deserialized.Checksum, "Checksum should be calculated")
	})

	// 2. Tombstone Entry (ValueSize = 0, Value = nil)
	t.Run("Tombstone Entry", func(t *testing.T) {
		original := &Entry{
			Timestamp: testTime,
			Expiry:    0,
			KeySize:   uint32(len(key)),
			ValueSize: 0,
			Key:       key,
			Value:     nil,
		}

		serializedData := original.Serialize()
		deserialized, err := DeserializeEntry(serializedData)

		assert.NoError(t, err)
		assert.Equal(t, original.Size(), len(serializedData), "Serialized data length should match Size()")
		assert.Equal(t, original.ValueSize, deserialized.ValueSize)
		assert.Nil(t, deserialized.Value, "Value should be nil after deserializing a tombstone")
		assert.Equal(t, original.Key, deserialized.Key)
	})
}

func TestDeserializeEntry_Errors(t *testing.T) {
	t.Parallel()

	// Case 1: Data too short (less than 20 bytes header)
	t.Run("Short Header", func(t *testing.T) {
		_, err := DeserializeEntry([]byte{1, 2, 3, 4, 5})
		assert.ErrorIs(t, err, ErrInvalidEntry, "Should fail if data is shorter than 20 bytes")
	})

	// Case 2: Data length mismatch (header says more data follows, but buffer ends)
	t.Run("Data Length Mismatch", func(t *testing.T) {
		entry := &Entry{
			Timestamp: uint32(time.Now().Unix()),
			Expiry:    0,
			KeySize:   10,
			ValueSize: 10,
			Key:       []byte("1234567890"),
			Value:     []byte("1234567890"),
		}
		data := entry.Serialize()
		
		// Corrupt the length by trimming the last 5 bytes
		corruptedData := data[:len(data)-5]
		_, err := DeserializeEntry(corruptedData)
		// It will fail either on checksum or size check
		assert.Error(t, err, "Should fail if data is truncated")
	})

	// Case 3: Checksum Mismatch
	t.Run("Checksum Mismatch", func(t *testing.T) {
		entry := &Entry{
			Timestamp: uint32(time.Now().Unix()),
			Expiry:    0,
			KeySize:   3,
			ValueSize: 3,
			Key:       []byte("key"),
			Value:     []byte("val"),
		}
		data := entry.Serialize()
		// Corrupt a byte in the payload
		data[15] ^= 0xFF
		
		_, err := DeserializeEntry(data)
		assert.ErrorIs(t, err, ErrChecksumMismatch, "Should fail with checksum mismatch if data is corrupted")
	})
}
