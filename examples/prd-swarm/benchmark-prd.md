# StreamDB: Local-First Streaming Timeseries Database

Build a production-grade, single-binary timeseries database in Rust with streaming ingestion, columnar storage, and SQL query support.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         StreamDB                                 │
├─────────────────────────────────────────────────────────────────┤
│  CLI / HTTP API                                                  │
│    - streamdb serve --port 8080                                 │
│    - streamdb query "SELECT * FROM metrics WHERE time > now()-1h"│
│    - streamdb ingest --stream sensors < data.json               │
├─────────────────────────────────────────────────────────────────┤
│  Query Engine                                                    │
│    - SQL Parser (sqlparser-rs)                                  │
│    - Logical Planner                                            │
│    - Physical Planner with pushdown                             │
│    - Streaming Execution Engine                                  │
├─────────────────────────────────────────────────────────────────┤
│  Ingestion Layer                                                 │
│    - Durable Write-Ahead Stream (Electric-style)                │
│    - In-memory Buffer (configurable flush threshold)            │
│    - Background Flusher (buffer → chunks)                       │
│    - Compactor (small chunks → large chunks)                    │
├─────────────────────────────────────────────────────────────────┤
│  Storage Layer                                                   │
│    - Arrow RecordBatches (columnar in-memory)                   │
│    - Parquet Chunks (columnar on-disk)                          │
│    - Chunk Manifest (metadata index)                            │
│    - Time-based Partitioning                                    │
├─────────────────────────────────────────────────────────────────┤
│  Local Storage Backend                                           │
│    - Single directory structure                                  │
│    - WAL: streams/{stream_id}/wal/*.log                         │
│    - Chunks: streams/{stream_id}/chunks/*.parquet               │
│    - Manifest: streams/{stream_id}/manifest.json                │
└─────────────────────────────────────────────────────────────────┘
```

## Technical Requirements

### Core Data Types
```rust
// Timestamp with nanosecond precision
pub type Timestamp = i64;

// Stream identifier
pub struct StreamId(pub String);

// A single data point
pub struct Point {
    pub timestamp: Timestamp,
    pub tags: HashMap<String, String>,
    pub fields: HashMap<String, FieldValue>,
}

pub enum FieldValue {
    Float(f64),
    Int(i64),
    Bool(bool),
    String(String),
}

// Time range for queries
pub struct TimeRange {
    pub start: Timestamp,
    pub end: Timestamp,
}
```

### Storage Layer

**Arrow Integration:**
- Use `arrow-rs` for in-memory columnar representation
- Schema: timestamp (i64), tags (struct), fields (struct)
- RecordBatch as the unit of data transfer

**Parquet Chunks:**
- Use `parquet` crate for on-disk storage
- One chunk = one Parquet file
- Chunk size: configurable (default 64MB or 1M rows)
- Compression: ZSTD

**Manifest:**
```rust
pub struct ChunkManifest {
    pub chunks: Vec<ChunkMeta>,
}

pub struct ChunkMeta {
    pub id: String,
    pub path: PathBuf,
    pub time_range: TimeRange,
    pub row_count: u64,
    pub size_bytes: u64,
    pub min_values: HashMap<String, FieldValue>,
    pub max_values: HashMap<String, FieldValue>,
}
```

### Durable Streams (Electric-style)

**Write-Ahead Log:**
- Append-only log files
- Each entry: `[length: u32][crc32: u32][payload: bytes]`
- Sync on write (configurable fsync policy)
- Segment rotation at 64MB

**Durability Guarantees:**
- Writes are durable before acknowledgment
- Crash recovery replays WAL from last checkpoint
- Checkpoints written after successful chunk flush

```rust
pub trait DurableStream {
    async fn append(&self, points: &[Point]) -> Result<Offset>;
    async fn read(&self, from: Offset) -> Result<impl Stream<Item = Point>>;
    async fn checkpoint(&self, offset: Offset) -> Result<()>;
}
```

### Ingestion Pipeline

**Buffer Manager:**
- In-memory buffer per stream
- Configurable size limit (default 16MB)
- Triggers flush when limit reached or on interval

**Flusher:**
- Converts buffer to Arrow RecordBatch
- Writes as Parquet chunk
- Updates manifest
- Advances WAL checkpoint

**Compactor:**
- Background process
- Merges small chunks into larger ones
- Maintains time-ordering
- Removes tombstoned data

### Query Engine

**SQL Support:**
```sql
-- Basic queries
SELECT * FROM metrics WHERE time > now() - interval '1 hour';

-- Aggregations
SELECT
    time_bucket('5 minutes', timestamp) as bucket,
    avg(value) as avg_value,
    max(value) as max_value
FROM sensors
WHERE tags->>'location' = 'warehouse-1'
GROUP BY bucket
ORDER BY bucket DESC;

-- Window functions
SELECT
    timestamp,
    value,
    avg(value) OVER (ORDER BY timestamp ROWS BETWEEN 10 PRECEDING AND CURRENT ROW) as moving_avg
FROM temperature;
```

**Query Pipeline:**
1. Parse SQL → AST (sqlparser-rs)
2. AST → Logical Plan
3. Logical Plan → Optimized Logical Plan (predicate pushdown, projection pruning)
4. Logical Plan → Physical Plan
5. Physical Plan → Execution (streaming RecordBatches)

**Pushdown Optimizations:**
- Time range pushdown to chunk selection
- Tag filters to Parquet row group filtering
- Projection pushdown (only read needed columns)

### CLI Interface

```bash
# Start server
streamdb serve --data-dir ./data --port 8080

# Ingest data
echo '{"timestamp": 1234567890, "tags": {"host": "server1"}, "fields": {"cpu": 45.2}}' | \
    streamdb ingest --stream metrics

# Query
streamdb query "SELECT * FROM metrics LIMIT 10"

# Stream status
streamdb streams list
streamdb streams info metrics

# Compaction
streamdb compact --stream metrics
```

### HTTP API

```
POST /v1/streams/{stream}/ingest
  Body: JSON array of points
  Response: { "offset": 12345, "count": 100 }

GET /v1/query?sql=SELECT...
  Response: JSON array of rows

GET /v1/streams
  Response: List of streams with metadata

GET /v1/streams/{stream}/chunks
  Response: Chunk manifest
```

## Project Structure

```
streamdb/
├── Cargo.toml
├── src/
│   ├── main.rs                 # CLI entry point
│   ├── lib.rs                  # Library root
│   ├── types/
│   │   ├── mod.rs
│   │   ├── point.rs            # Point, FieldValue
│   │   ├── timestamp.rs        # Timestamp utilities
│   │   └── stream.rs           # StreamId, TimeRange
│   ├── storage/
│   │   ├── mod.rs
│   │   ├── arrow_codec.rs      # Point <-> RecordBatch
│   │   ├── chunk.rs            # Parquet read/write
│   │   ├── manifest.rs         # Chunk manifest
│   │   └── backend.rs          # Local filesystem backend
│   ├── stream/
│   │   ├── mod.rs
│   │   ├── wal.rs              # Write-ahead log
│   │   ├── buffer.rs           # In-memory buffer
│   │   ├── flusher.rs          # Buffer → Chunk
│   │   └── compactor.rs        # Chunk merging
│   ├── query/
│   │   ├── mod.rs
│   │   ├── parser.rs           # SQL parsing
│   │   ├── planner.rs          # Logical planning
│   │   ├── optimizer.rs        # Plan optimization
│   │   ├── executor.rs         # Physical execution
│   │   └── functions.rs        # time_bucket, now(), etc.
│   ├── server/
│   │   ├── mod.rs
│   │   ├── http.rs             # HTTP API (axum)
│   │   └── handler.rs          # Request handlers
│   └── cli/
│       ├── mod.rs
│       ├── serve.rs
│       ├── query.rs
│       ├── ingest.rs
│       └── streams.rs
└── tests/
    ├── integration/
    │   ├── ingest_test.rs
    │   ├── query_test.rs
    │   └── recovery_test.rs
    └── benches/
        ├── ingest_bench.rs
        └── query_bench.rs
```

## Dependencies

```toml
[dependencies]
# Arrow ecosystem
arrow = "52"
parquet = "52"

# SQL parsing
sqlparser = "0.45"

# Async runtime
tokio = { version = "1", features = ["full"] }

# HTTP server
axum = "0.7"
tower = "0.4"

# CLI
clap = { version = "4", features = ["derive"] }

# Serialization
serde = { version = "1", features = ["derive"] }
serde_json = "1"

# Error handling
thiserror = "1"
anyhow = "1"

# Utilities
uuid = { version = "1", features = ["v4"] }
chrono = "0.4"
tracing = "0.1"
tracing-subscriber = "0.3"
crc32fast = "1"
```

## Success Criteria

1. **Ingestion**: 100K points/second sustained write throughput
2. **Query**: Sub-second response for 1-hour range queries over 1B points
3. **Durability**: Zero data loss on crash with WAL replay
4. **Single Binary**: `cargo build --release` produces one executable
5. **Local-First**: Works entirely offline, no external dependencies
6. **SQL**: Supports SELECT, WHERE, GROUP BY, ORDER BY, LIMIT, time functions
