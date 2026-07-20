# LagerKV Store - Append-Only Logs Meet In-Memory Hash Indexes

[![Go Report Card](https://goreportcard.com/badge/github.com/N-Saipraveen/LagerKV)](https://goreportcard.com/report/github.com/N-Saipraveen/LagerKV)
[![Go 1.24+](https://img.shields.io/badge/Go-1.24%2B-blue.svg)](https://go.dev/)
[![MIT License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![GitHub Pages](https://img.shields.io/badge/Live-GitHub%20Pages-blueviolet.svg)](https://N-Saipraveen.github.io/LagerKV/)

A lightweight, high-performance Key-Value store built in Go, inspired by the [Bitcask paper](https://riak.com/assets/bitcask-intro.pdf). 

Built for speed, simplicity, and durability, this project is a great showcase of storage engine design principles, combining sequential append-only writes with memory-resident hash indexes.

👉 **Try the Live Interactive Simulator on GitHub Pages:** [https://N-Saipraveen.github.io/LagerKV/](https://N-Saipraveen.github.io/LagerKV/)

---

## Design Architecture

LagerKV Store leverages the classic **Bitcask** layout:
1. **Append-Only Write Log (Segments):** All set and delete operations are written sequentially to segment files on disk. Deletions append a tombstone record.
2. **In-Memory Hash Index (KeyDir):** A memory-resident Hash Table stores the mapping of all active keys to their precise segment file ID, value offset, value size, and expiry timestamp.
3. **Lookup Flow:** Reads check the in-memory index for O(1) searches. If found, the engine performs exactly one disk seek to read the value directly from the segment offset.

---

## Implemented Storage Engine Features

- **CRC32 Checksums:** Every log record on disk is protected by a 4-byte IEEE CRC32 checksum. Checksums are verified during startup (loading segments) and on every subsequent read to prevent bit rot and data corruption.
- **TTL (Time to Live) Expiry:** Expiry timestamp is embedded directly in the disk record header. Keys expire automatically, returning `Key Not Found` upon read, and are pruned from the disk during log compaction.
- **Process locking (`lock.db`):** Uses an exclusive `flock` lock file inside the data directory to ensure that only a single process can write to the database folder at any given time.
- **Fsync Strategies:** Durability configurations supporting `"always"` (fsync after every write), `"interval"` (periodic background flushing), or `"none"` (relying on OS default disk buffering).
- **Log Compaction / Compaction-Merge:** Background and manual compaction loops sweep away old inactive segment logs, reclaiming disk space by discarding tombstones, stale versions, and expired keys.

---

## Binary Record Layout (On-Disk)

Each entry written to a segment file is serialized in the following binary format:

```text
+------------------+-------------------+-----------------+------------------+--------------------+---------+-----------+
| Checksum (CRC32) | Timestamp (Unix)  | Expiry (Unix)   | Key Size (Bytes) | Value Size (Bytes) | Key     | Value     |
| 4 Bytes          | 4 Bytes           | 4 Bytes         | 4 Bytes          | 4 Bytes            | N Bytes | M Bytes   |
+------------------+-------------------+-----------------+------------------+--------------------+---------+-----------+
```

---

## Getting Started

### Prerequisites

- Go 1.24+
- macOS or Linux (Windows works with minor path adjustments)

### Clone & Build

```bash
# Clone the repository
git clone https://github.com/N-Saipraveen/LagerKV.git
cd LagerKV

# Compile server and CLI client binaries
go build -o bin/lagerkvd ./cmd/lagerkvd
go build -o bin/lagerkv ./cmd/lagerkv-cli
```

### Running the Server

Start the database server daemon (defaults to port `:8080` and saves data under `./data`):

```bash
# Run server
./bin/lagerkvd
```

### Using the CLI Client

Open a separate terminal window and interact with the database using the client CLI:

```bash
# Set a key with 60 seconds TTL (Time to Live)
./bin/lagerkv set user_session "auth_token_xyz" --ttl 60s

# Retrieve a key
./bin/lagerkv get user_session

# List all live, unexpired keys
./bin/lagerkv list

# Get engine statistics (number of keys, segments, disk size)
./bin/lagerkv stats
```

---

## Run Tests

Run the full suite of unit and integration tests (validating TTL expiration, compaction, lock collisions, and checksum corruptions):

```bash
# Configure GOPATH/GOCACHE within directory if running in standard sandbox
export GOPATH=$(pwd)/.gopath
export GOCACHE=$(pwd)/.gocache

# Run tests
go test ./... -v -cover
```

---

## Contributing

Contributions are welcome! Please open an issue or submit a pull request. Keep changes focused, cleanly written, and fully covered by tests.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
