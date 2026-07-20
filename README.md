# 🍺 LagerKV

> **A High-Performance, Write-Optimized Key-Value Storage Engine in Go**

LagerKV is a lightweight, durable Key-Value store built from scratch in Go. It is inspired by **Riak's Bitcask architecture**, which pairs **sequential append-only log writing** with a **memory-resident hash table index** for ultra-fast performance.

💻 **Interactive Browser Simulator:** Experience how LagerKV works inside your browser!  
👉 **[View the Live Demo Site](https://N-Saipraveen.github.io/LagerKV/)**

---

## 🚀 Why LagerKV? (The Core Idea)

Traditional databases perform random writes across disk blocks, which slows down write throughput. LagerKV solves this by adopting the **Bitcask design**:

1. **High-Speed Writes ($O(1)$):** Every `SET` or `DELETE` is simply appended to the end of a log file (Segment). Because writing sequentially is fast, writes complete instantly.
2. **Instant Lookups ($O(1)$):** An in-memory hash table (called **KeyDir**) keeps track of the file location, size, and byte offset of every key.
3. **Single Disk Seek Read:** When you run `GET`, the engine looks up the key in the in-memory index, seeks directly to that byte offset on disk, and reads the value. **Exactly one disk seek is required to read any value.**

```text
               +-----------------------------------+
               |        Client Write (SET)         |
               +-----------------------------------+
                                 |
                                 v
        +-------------------------------------------------+
        | 1. Append Binary Record to Disk (Active Log)    |
        | 2. Insert offset, size & TTL to In-Memory Index |
        +-------------------------------------------------+
                                 |
        +------------------------+------------------------+
        |                                                 |
        v                                                 v
+-------------------------------+                 +-------------------------------+
|  Active Segment File (Disk)  |                 |    HashTable Index (Memory)   |
|  [Header] [Key] [Value]       |                 |  "user_1" -> Offset: 1024     |
+-------------------------------+                 +-------------------------------+
```

---

## 💎 Advanced Features Implemented

* **🔒 Thread & Process Safety (`lock.db`):** Uses system-level `flock` locking on your database directory. If another process tries to open the same folder concurrently, it is blocked to prevent data corruption.
* **🛡️ CRC32 Checksums (Data Integrity):** Guard against bit rot and file corruption. Each record has a 4-byte CRC32 checksum computed on write and verified automatically on startup and reads.
* **⏱️ Expirations & TTL:** Set keys with a Time-to-Live (TTL). Expired keys disappear from queries instantly and are automatically wiped from the disk during log merges.
* **⚡ Durability Control (Fsync):** Choose how often data is written to physical disk:
  - `Always`: Write and flush to disk on every single write (maximum durability, slower).
  - `Interval`: Flush to disk periodically in the background (great balance).
  - `None`: Let the OS handle write buffering (maximum performance).
* **🧹 Automatic Log Compaction (Merge):** A background worker periodically scans old log files, discards tombstones (deleted keys) and expired keys, and merges live values into new compressed segments.

---

## 📦 How Data is Structured on Disk

Every entry in the append-only logs is serialized into a highly compact binary format:

```text
+-------------------+-------------------+-------------------+-------------------+-------------------+----------+------------+
|  Checksum (CRC32) | Timestamp (Unix)  |  Expiry (Unix)    | Key Size (Bytes)  | Value Size (Bytes)|   Key    |   Value    |
|      4 Bytes      |      4 Bytes      |      4 Bytes      |      4 Bytes      |      4 Bytes      | N Bytes  |  M Bytes   |
+-------------------+-------------------+-------------------+-------------------+-------------------+----------+------------+
```

---

## 🛠️ Build and Setup

### Prerequisites
- Go 1.24+ Installed
- macOS or Linux

### 1. Compile Binaries
Clone and compile the server daemon (`lagerkvd`) and the CLI interactive program (`lagerkv`):

```bash
# Clone the repository
git clone https://github.com/N-Saipraveen/LagerKV.git
cd LagerKV

# Compile binaries
go build -o bin/lagerkvd ./cmd/lagerkvd
go build -o bin/lagerkv ./cmd/lagerkv-cli
```

### 2. Start the Database Daemon
Run the server daemon. It binds to port `8080` by default and stores segments in a `./data` folder:

```bash
./bin/lagerkvd
```

### 3. Use the CLI Client
In a separate terminal, interact with the server:

```bash
# Set a key with 30 seconds TTL (Time to Live)
./bin/lagerkv set session_id "saipraveen_token_123" --ttl 30s
# Output: [SUCCESS] Set session_id = saipraveen_token_123 (TTL: 30s)

# Get the value
./bin/lagerkv get session_id
# Output: saipraveen_token_123

# List all live, unexpired keys
./bin/lagerkv list

# Get live engine stats (Number of keys, log files, disk usage)
./bin/lagerkv stats
```

---

## 🧪 Running Tests

Ensure all unit, concurrent, and integration tests pass successfully:

```bash
# Configure sandbox cache directories
export GOPATH=$(pwd)/.gopath
export GOCACHE=$(pwd)/.gocache

# Run all test suites
go test ./... -v -count=1
```

---

## 📜 License

This project is open-source and licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
