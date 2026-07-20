package store

import (
	"encoding/binary"
	"hash/crc32"
	"time"
)

// Entry represents a single entry in the append-only log
type Entry struct {
	Checksum  uint32 // CRC32 checksum
	Timestamp uint32 // Unix timestamp
	Expiry    uint32 // Expiry Unix timestamp (0 means no TTL)
	KeySize   uint32 // Size of the key in bytes
	ValueSize uint32 // Size of the value in bytes
	Key       []byte // Key data
	Value     []byte // Value data
}

// TombstoneEntry represents a deleted entry (tombstone)
func (e *Entry) TombstoneEntry() *Entry {
	return &Entry{
		Timestamp: uint32(time.Now().Unix()),
		Expiry:    0,
		KeySize:   e.KeySize,
		ValueSize: 0, // Zero value size indicates tombstone
		Key:       e.Key,
		Value:     nil,
	}
}

// IsTombstone checks if this entry is a tombstone (deleted entry)
func (e *Entry) IsTombstone() bool {
	return e.ValueSize == 0
}

// Size returns the total size of the entry in bytes
func (e *Entry) Size() int {
	return 20 + int(e.KeySize) + int(e.ValueSize) // 20 bytes for checksum + timestamp + expiry + keysize + valuesize
}

// Serialize converts the entry to bytes for writing to disk
func (e *Entry) Serialize() []byte {
	buf := make([]byte, e.Size())
	offset := 4 // Checksum is placed at bytes 0-4 after calculation

	// Write timestamp (4 bytes)
	binary.LittleEndian.PutUint32(buf[offset:], e.Timestamp)
	offset += 4

	// Write expiry (4 bytes)
	binary.LittleEndian.PutUint32(buf[offset:], e.Expiry)
	offset += 4

	// Write key size (4 bytes)
	binary.LittleEndian.PutUint32(buf[offset:], e.KeySize)
	offset += 4

	// Write value size (4 bytes)
	binary.LittleEndian.PutUint32(buf[offset:], e.ValueSize)
	offset += 4

	// Write key data
	copy(buf[offset:], e.Key)
	offset += int(e.KeySize)

	// Write value data
	if e.ValueSize > 0 {
		copy(buf[offset:], e.Value)
	}

	// Compute checksum on all contents after the 4-byte checksum field
	checksum := crc32.ChecksumIEEE(buf[4:])
	binary.LittleEndian.PutUint32(buf[0:4], checksum)

	return buf
}

// DeserializeEntry creates an entry from bytes read from disk
func DeserializeEntry(data []byte) (*Entry, error) {
	if len(data) < 20 {
		return nil, ErrInvalidEntry
	}

	// Read checksum
	checksum := binary.LittleEndian.Uint32(data[0:4])

	// Calculate and verify checksum
	computed := crc32.ChecksumIEEE(data[4:])
	if checksum != computed {
		return nil, ErrChecksumMismatch
	}

	entry := &Entry{
		Checksum: checksum,
	}

	// Read timestamp
	entry.Timestamp = binary.LittleEndian.Uint32(data[4:8])

	// Read expiry
	entry.Expiry = binary.LittleEndian.Uint32(data[8:12])

	// Read key size
	entry.KeySize = binary.LittleEndian.Uint32(data[12:16])

	// Read value size
	entry.ValueSize = binary.LittleEndian.Uint32(data[16:20])

	// Validate sizes
	if int(entry.KeySize)+int(entry.ValueSize) != len(data)-20 {
		return nil, ErrInvalidEntry
	}

	// Read key data
	entry.Key = make([]byte, entry.KeySize)
	copy(entry.Key, data[20:20+entry.KeySize])

	// Read value data
	if entry.ValueSize > 0 {
		entry.Value = make([]byte, entry.ValueSize)
		copy(entry.Value, data[20+entry.KeySize:20+entry.KeySize+entry.ValueSize])
	}

	return entry, nil
}
