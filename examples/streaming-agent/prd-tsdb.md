# StreamDB: Modern Timeseries Database

A timeseries database built on durable-streams and object storage.

## Architecture

```
Writes                              Queries
  │                                    │
  ▼                                    ▼
┌──────────────────┐          ┌──────────────────┐
│  Durable Stream  │          │   Query Engine   │
│  (embedded)      │          │   (SQL + streaming)
└────────┬─────────┘          └────────┬─────────┘
         │                             │
         │ consume                     │ read
         ▼                             ▼
┌──────────────────┐          ┌──────────────────┐
│  Write Buffer    │          │  Chunk Reader    │
│  (in-memory)     │          │  (Arrow format)  │
└────────┬─────────┘          └────────┬─────────┘
         │                             │
         │ flush                       │
         ▼                             ▼
┌─────────────────────────────────────────────────┐
│              Object Storage (R2/Local)          │
│  chunks/2024/01/15/00/chunk_001.arrow           │
│  chunks/2024/01/15/01/chunk_002.arrow           │
│  manifest.json                                  │
└─────────────────────────────────────────────────┘
```

## Data Model

```rust
// A single data point
struct DataPoint {
    timestamp: i64,        // Unix micros
    series_id: u64,        // Hash of metric + tags
    value: f64,
}

// Series metadata
struct Series {
    id: u64,
    metric: String,        // e.g., "cpu_usage"
    tags: BTreeMap<String, String>,  // e.g., {"host": "web-1"}
}

// Time-partitioned chunk
struct Chunk {
    id: String,
    start_time: i64,
    end_time: i64,
    row_count: u64,
    size_bytes: u64,
    path: String,          // Object storage path
}
```

## Components

### Stream Layer
- **stream/server.rs** - Embedded HTTP server implementing durable-streams protocol
- **stream/client.rs** - Consumer that reads from stream with offset tracking
- **stream/protocol.rs** - Wire format: newline-delimited JSON

### Storage Layer
- **storage/arrow.rs** - Arrow RecordBatch encode/decode for DataPoints
- **storage/chunk.rs** - Chunk file format (Arrow IPC + footer metadata)
- **storage/object.rs** - Object storage trait + R2/local implementations
- **storage/manifest.rs** - Track all chunks, time ranges, compaction state

### Ingestion Layer
- **ingest/buffer.rs** - In-memory buffer, triggers flush at size/time threshold
- **ingest/flusher.rs** - Converts buffer → Arrow → chunk file → upload
- **ingest/compactor.rs** - Merge small chunks into larger ones (background)

### Query Layer
- **query/parser.rs** - SQL parser: SELECT, WHERE time BETWEEN, GROUP BY time()
- **query/planner.rs** - Prune chunks by time range, push down predicates
- **query/scan.rs** - Scan operator: read chunks, decode Arrow, filter
- **query/aggregate.rs** - Time-window aggregations: SUM, AVG, COUNT, MIN, MAX
- **query/streaming.rs** - Continuous queries: tail stream + emit results

### CLI
- **bin/streamdb.rs** - CLI commands: start, query, ingest, status

## SQL Dialect

```sql
-- Point query
SELECT * FROM metrics
WHERE time BETWEEN '2024-01-15T00:00:00Z' AND '2024-01-15T01:00:00Z'
  AND metric = 'cpu_usage'
  AND tags['host'] = 'web-1';

-- Aggregation with time windows
SELECT
  time_bucket('5m', time) as bucket,
  metric,
  AVG(value) as avg_value,
  MAX(value) as max_value
FROM metrics
WHERE time > now() - interval '1 hour'
GROUP BY bucket, metric;

-- Streaming query (continuous)
SUBSCRIBE TO
  SELECT time_bucket('1m', time), AVG(value)
  FROM metrics
  WHERE metric = 'cpu_usage'
  GROUP BY 1;
```

## File Layout

```
data/
├── stream/              # Durable stream storage
│   └── metrics/
│       ├── 000001.log
│       └── 000002.log
├── chunks/              # Arrow chunk files
│   └── 2024/01/15/
│       ├── 00/
│       │   └── chunk_a1b2c3.arrow
│       └── 01/
│           └── chunk_d4e5f6.arrow
└── manifest.json        # Chunk catalog
```

## Demo Flow

```bash
# Terminal 1: Start the database
streamdb start --data-dir ./data --port 8080

# Terminal 2: Ingest sample metrics
streamdb ingest --generate-sample --rate 1000/s --duration 60s

# Terminal 3: Query
streamdb query "SELECT time_bucket('10s', time), AVG(value) FROM metrics WHERE time > now() - '1m' GROUP BY 1"

# Or streaming query
streamdb subscribe "SELECT time_bucket('5s', time), AVG(value) FROM metrics GROUP BY 1"
```

## Dependencies

```toml
[dependencies]
arrow = "50"
tokio = { version = "1", features = ["full"] }
hyper = "1"
serde = { version = "1", features = ["derive"] }
serde_json = "1"
sqlparser = "0.43"
thiserror = "1"
```
