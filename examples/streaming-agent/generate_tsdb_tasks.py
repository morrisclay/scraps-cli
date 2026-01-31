#!/usr/bin/env python3
"""
Generate tasks for StreamDB - a timeseries database.

Creates:
1. Foundation files (types, traits, error) - actual code, not tasks
2. 20 parallel implementation tasks - all depend only on foundation
"""

import os
import sys

# Foundation code - created directly, not as tasks
FOUNDATION_FILES = {
    "Cargo.toml": '''[package]
name = "streamdb"
version = "0.1.0"
edition = "2021"

[dependencies]
arrow = { version = "50", features = ["ipc"] }
tokio = { version = "1", features = ["full"] }
hyper = { version = "1", features = ["full"] }
http-body-util = "0.1"
hyper-util = { version = "0.1", features = ["full"] }
serde = { version = "1", features = ["derive"] }
serde_json = "1"
sqlparser = "0.43"
thiserror = "1"
bytes = "1"
parking_lot = "0.12"
tracing = "0.1"
async-trait = "0.1"
uuid = { version = "1", features = ["v4"] }
chrono = "0.4"

[[bin]]
name = "streamdb"
path = "src/bin/streamdb.rs"
''',

    "src/lib.rs": '''//! StreamDB - Modern timeseries database on durable streams + object storage

pub mod types;
pub mod error;
pub mod traits;
pub mod stream;
pub mod storage;
pub mod ingest;
pub mod query;

pub use types::*;
pub use error::*;
pub use traits::*;
''',

    "src/types.rs": '''//! Core data types for StreamDB

use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;

/// Unix timestamp in microseconds
pub type Timestamp = i64;

/// A single timeseries data point
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DataPoint {
    pub timestamp: Timestamp,
    pub series_id: u64,
    pub value: f64,
}

/// Series metadata (metric name + tags)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Series {
    pub id: u64,
    pub metric: String,
    pub tags: BTreeMap<String, String>,
}

impl Series {
    pub fn new(metric: impl Into<String>, tags: BTreeMap<String, String>) -> Self {
        let metric = metric.into();
        let id = Self::compute_id(&metric, &tags);
        Self { id, metric, tags }
    }

    pub fn compute_id(metric: &str, tags: &BTreeMap<String, String>) -> u64 {
        use std::hash::{Hash, Hasher};
        let mut hasher = std::collections::hash_map::DefaultHasher::new();
        metric.hash(&mut hasher);
        for (k, v) in tags {
            k.hash(&mut hasher);
            v.hash(&mut hasher);
        }
        hasher.finish()
    }
}

/// Metadata for a stored chunk
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChunkMeta {
    pub id: String,
    pub start_time: Timestamp,
    pub end_time: Timestamp,
    pub row_count: u64,
    pub size_bytes: u64,
    pub path: String,
}

/// Database configuration
#[derive(Debug, Clone)]
pub struct Config {
    pub data_dir: String,
    pub stream_port: u16,
    pub query_port: u16,
    pub flush_interval_ms: u64,
    pub flush_size_bytes: usize,
    pub chunk_duration_secs: u64,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            data_dir: "./data".into(),
            stream_port: 8081,
            query_port: 8080,
            flush_interval_ms: 5000,
            flush_size_bytes: 1024 * 1024, // 1MB
            chunk_duration_secs: 3600,      // 1 hour
        }
    }
}

/// Time bucket for aggregations
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct TimeBucket {
    pub start: Timestamp,
    pub width_micros: i64,
}

impl TimeBucket {
    pub fn new(timestamp: Timestamp, width_micros: i64) -> Self {
        let start = (timestamp / width_micros) * width_micros;
        Self { start, width_micros }
    }
}
''',

    "src/error.rs": '''//! Error types for StreamDB

use thiserror::Error;

#[derive(Error, Debug)]
pub enum DbError {
    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),

    #[error("Arrow error: {0}")]
    Arrow(#[from] arrow::error::ArrowError),

    #[error("JSON error: {0}")]
    Json(#[from] serde_json::Error),

    #[error("Parse error: {0}")]
    Parse(String),

    #[error("Query error: {0}")]
    Query(String),

    #[error("Storage error: {0}")]
    Storage(String),

    #[error("Stream error: {0}")]
    Stream(String),

    #[error("Not found: {0}")]
    NotFound(String),

    #[error("Invalid argument: {0}")]
    InvalidArgument(String),
}

pub type Result<T> = std::result::Result<T, DbError>;
''',

    "src/traits.rs": '''//! Core traits for StreamDB components

use crate::{ChunkMeta, DataPoint, Result, Timestamp};
use async_trait::async_trait;
use bytes::Bytes;

/// Object storage backend (R2, S3, local filesystem)
#[async_trait]
pub trait ObjectStore: Send + Sync {
    async fn put(&self, path: &str, data: Bytes) -> Result<()>;
    async fn get(&self, path: &str) -> Result<Bytes>;
    async fn delete(&self, path: &str) -> Result<()>;
    async fn list(&self, prefix: &str) -> Result<Vec<String>>;
    async fn exists(&self, path: &str) -> Result<bool>;
}

/// Write buffer for batching data points
pub trait WriteBuffer: Send + Sync {
    fn push(&self, point: DataPoint);
    fn flush(&self) -> Vec<DataPoint>;
    fn len(&self) -> usize;
    fn is_empty(&self) -> bool { self.len() == 0 }
    fn size_bytes(&self) -> usize;
}

/// Query operator (volcano model)
pub trait Operator: Send {
    fn next(&mut self) -> Result<Option<DataPoint>>;
    fn reset(&mut self);
}

/// Aggregate function
pub trait AggregateFunc: Send + Sync {
    fn name(&self) -> &str;
    fn update(&mut self, value: f64);
    fn finalize(&self) -> f64;
    fn reset(&mut self);
}

/// Stream consumer callback
#[async_trait]
pub trait StreamConsumer: Send + Sync {
    async fn on_data(&self, data: &[DataPoint]) -> Result<()>;
}
''',

    "src/stream/mod.rs": '''pub mod server;
pub mod client;
pub mod protocol;

pub use server::*;
pub use client::*;
pub use protocol::*;
''',

    "src/storage/mod.rs": '''pub mod arrow_codec;
pub mod chunk;
pub mod object;
pub mod manifest;

pub use arrow_codec::*;
pub use chunk::*;
pub use object::*;
pub use manifest::*;
''',

    "src/ingest/mod.rs": '''pub mod buffer;
pub mod flusher;
pub mod compactor;

pub use buffer::*;
pub use flusher::*;
pub use compactor::*;
''',

    "src/query/mod.rs": '''pub mod parser;
pub mod planner;
pub mod scan;
pub mod aggregate;
pub mod streaming;

pub use parser::*;
pub use planner::*;
pub use scan::*;
pub use aggregate::*;
pub use streaming::*;
''',
}

# 20 parallel implementation tasks
TASKS = [
    # Stream layer (3)
    ("stream-server", "src/stream/server.rs", "Durable streams HTTP server",
     "Implement embedded HTTP server for durable-streams protocol. Handle PUT (create stream), POST (append), GET (read with offset). Store stream data in files. Return Stream-Next-Offset header.",
     ["HTTP server binds to configured port", "Append returns offset", "Read with offset works", "Live long-poll tailing works"]),

    ("stream-client", "src/stream/client.rs", "Durable streams consumer",
     "Implement stream consumer that reads from stream with offset tracking. Support resumable reads, live tailing. Parse incoming data points using protocol module.",
     ["Reads from stream endpoint", "Tracks offset for resume", "Supports live tail mode", "Parses DataPoints"]),

    ("stream-protocol", "src/stream/protocol.rs", "Stream wire protocol",
     "Implement newline-delimited JSON protocol for stream data. Encode/decode DataPoint to/from JSON lines. Handle batch encoding for efficiency.",
     ["Encode DataPoint to JSON line", "Decode JSON line to DataPoint", "Batch encode/decode", "Handle malformed input"]),

    # Storage layer (4)
    ("arrow-codec", "src/storage/arrow_codec.rs", "Arrow encoding/decoding",
     "Implement Arrow RecordBatch encoding for DataPoints. Schema: timestamp (Int64), series_id (UInt64), value (Float64). Support encode batch and decode batch.",
     ["Create Arrow schema", "Encode Vec<DataPoint> to RecordBatch", "Decode RecordBatch to Vec<DataPoint>", "Handle empty batches"]),

    ("chunk-format", "src/storage/chunk.rs", "Chunk file format",
     "Implement chunk file format using Arrow IPC. Write RecordBatch to file with metadata footer (time range, row count). Read chunks back with metadata.",
     ["Write chunk to bytes", "Read chunk from bytes", "Store/retrieve time range", "Store/retrieve row count"]),

    ("object-store", "src/storage/object.rs", "Object storage backends",
     "Implement ObjectStore trait for local filesystem. Store files in data_dir/chunks/YYYY/MM/DD/HH/ structure. Support put, get, delete, list, exists.",
     ["LocalStore implements ObjectStore", "Hierarchical path structure", "List with prefix filtering", "Atomic writes"]),

    ("manifest", "src/storage/manifest.rs", "Chunk manifest/catalog",
     "Implement manifest that tracks all chunks. Store as JSON file. Support add chunk, remove chunk, query by time range. Thread-safe updates.",
     ["Load/save manifest JSON", "Add chunk metadata", "Query chunks by time range", "Thread-safe with RwLock"]),

    # Ingestion layer (3)
    ("write-buffer", "src/ingest/buffer.rs", "In-memory write buffer",
     "Implement WriteBuffer trait with thread-safe in-memory buffer. Support push (single point), flush (drain all), len, size_bytes. Use parking_lot Mutex.",
     ["Thread-safe push", "Atomic flush returns all points", "Accurate size tracking", "Implements WriteBuffer trait"]),

    ("flusher", "src/ingest/flusher.rs", "Buffer to chunk flusher",
     "Implement flusher that converts buffer contents to Arrow chunks and uploads to object storage. Trigger on size threshold or time interval. Update manifest.",
     ["Flush buffer to Arrow chunk", "Upload chunk to object store", "Update manifest", "Generate unique chunk IDs"]),

    ("compactor", "src/ingest/compactor.rs", "Chunk compaction",
     "Implement background compactor that merges small chunks into larger ones. Scan for chunks in same time window, merge if total size below threshold, update manifest.",
     ["Find chunks to compact", "Merge multiple chunks", "Delete old chunks after merge", "Update manifest atomically"]),

    # Query layer (5)
    ("sql-parser", "src/query/parser.rs", "SQL query parser",
     "Parse SQL queries using sqlparser crate. Support SELECT with time BETWEEN, WHERE filters, GROUP BY time_bucket(), aggregates (SUM, AVG, COUNT, MIN, MAX). Return AST.",
     ["Parse SELECT statements", "Extract time range predicates", "Parse time_bucket() calls", "Parse aggregate functions"]),

    ("query-planner", "src/query/planner.rs", "Query planner",
     "Convert parsed SQL AST to execution plan. Prune chunks by time range using manifest. Push down simple predicates. Create operator tree.",
     ["Prune chunks by time range", "Create scan operators for relevant chunks", "Push down filters", "Build operator tree"]),

    ("scan-operator", "src/query/scan.rs", "Scan operator",
     "Implement scan operator that reads chunks from object storage. Decode Arrow data, apply filters, yield DataPoints. Implement Operator trait.",
     ["Read chunk from object store", "Decode Arrow to DataPoints", "Apply filter predicates", "Implement Operator::next()"]),

    ("aggregate-operator", "src/query/aggregate.rs", "Aggregation operators",
     "Implement aggregate functions: Sum, Avg, Count, Min, Max. Implement time-bucket grouping. Support streaming aggregation (incremental updates).",
     ["Sum, Avg, Count, Min, Max functions", "Time bucket grouping", "Implement AggregateFunc trait", "Streaming/incremental updates"]),

    ("streaming-query", "src/query/streaming.rs", "Streaming query executor",
     "Implement continuous query executor for SUBSCRIBE queries. Tail stream for new data, apply query operators, emit results as they arrive.",
     ["Subscribe to stream", "Apply query operators to new data", "Emit incremental results", "Handle backpressure"]),

    # CLI & integration (3)
    ("cli-main", "src/bin/streamdb.rs", "CLI entry point",
     "Implement CLI with subcommands: start (run server), query (execute SQL), ingest (send data), status (show stats). Use std::env::args for parsing.",
     ["start subcommand", "query subcommand", "ingest subcommand", "status subcommand"]),

    ("database", "src/database.rs", "Database coordinator",
     "Implement Database struct that wires all components together. Start stream server, spawn flusher, provide query interface. Graceful shutdown.",
     ["Initialize all components", "Start background tasks", "Query execution entry point", "Graceful shutdown"]),

    ("sample-generator", "src/sample.rs", "Sample data generator",
     "Implement sample metrics generator for demo. Generate cpu_usage, memory_usage, disk_io with random walk values. Configurable rate and duration.",
     ["Generate realistic metrics", "Multiple metric types", "Configurable rate", "Random walk values"]),

    # Demo (2)
    ("http-api", "src/api.rs", "HTTP query API",
     "Implement HTTP endpoint for queries. POST /query with SQL body, return JSON results. GET /status for health check. Support streaming responses for SUBSCRIBE.",
     ["POST /query endpoint", "JSON response format", "GET /status health check", "Streaming response for subscriptions"]),

    ("demo-example", "examples/demo.rs", "End-to-end demo",
     "Create runnable demo that starts DB, ingests sample data, runs queries, shows results. Print clear output showing the pipeline working.",
     ["Start embedded database", "Generate and ingest sample data", "Run example queries", "Print formatted results"]),
]


def generate_task_file(num: int, slug: str, filepath: str, title: str, description: str, criteria: list[str]) -> tuple[str, str]:
    """Generate task filename and content."""
    filename = f"tasks/{num:03d}-{slug}.md"
    criteria_md = "\n".join(f"- [ ] {c}" for c in criteria)

    content = f"""---
status: pending
claimed_by: null
priority: 2
depends_on: []
owns: [{filepath}]
---
# Task: {title}

## Description
{description}

## File to Create
`{filepath}`

## Acceptance Criteria
{criteria_md}

## Notes
- Import types/traits from `crate::*`
- Use `crate::Result<T>` for error handling
- Keep implementation focused and minimal
"""
    return filename, content


def main():
    if len(sys.argv) < 3:
        print(f"Usage: {sys.argv[0]} <store> <repo>")
        sys.exit(1)

    store = sys.argv[1]
    repo = sys.argv[2]

    from agent_base import ScrapsClient

    scraps = ScrapsClient(store, repo, "main", "task-generator")

    files = {}

    # Add PRD
    script_dir = os.path.dirname(os.path.abspath(__file__))
    with open(os.path.join(script_dir, "prd-tsdb.md")) as f:
        files["prd.md"] = f.read()
    print("  + prd.md")

    # Add foundation files
    print("\nCreating foundation files...")
    for path, content in FOUNDATION_FILES.items():
        files[path] = content
        print(f"  + {path}")

    # Need to add database.rs and sample.rs and api.rs to lib.rs
    files["src/lib.rs"] = files["src/lib.rs"].rstrip() + "\npub mod database;\npub mod sample;\npub mod api;\n"

    # Generate tasks
    print(f"\nGenerating {len(TASKS)} parallel tasks...")
    for i, (slug, filepath, title, desc, criteria) in enumerate(TASKS, 1):
        filename, content = generate_task_file(i, slug, filepath, title, desc, criteria)
        files[filename] = content
        print(f"  + {filename}")

    # Commit all at once
    print(f"\nCommitting {len(files)} files...")
    sha = scraps.commit(f"Initialize StreamDB with {len(TASKS)} parallel tasks", files)
    print(f"Committed: {sha[:8]}")
    print(f"\nReady for {len(TASKS)} agents!")


if __name__ == "__main__":
    main()
