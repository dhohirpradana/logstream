# Logstream

Go log collector + parser core. Streams from Docker, systemd, files, webhooks → SQLite with FTS5 full-text search.

## Architecture

```
Log Sources → Collectors → Parser → Store (SQLite + FTS5)
```

**Package Structure:**
- `cmd/logstream` - Main binary
- `internal/collector` - Docker, systemd, file tail, webhook receiver
- `internal/parser` - JSON, syslog, nginx, custom regex, auto-detect
- `internal/store` - SQLite schema with FTS5 full-text index

## Features

- **Multi-source collectors:**
  - Docker logs (SDK stream)
  - Systemd journalctl tail
  - File tail (inotify watch)
  - Webhook receiver (HTTP POST)

- **Parsers:**
  - JSON (common timestamp/level/message fields)
  - Syslog (RFC3164 style)
  - Nginx common log format
  - Custom regex with capture groups
  - Auto-detect (tries all parsers, falls back to plain text)

- **Storage:**
  - SQLite with FTS5 full-text index
  - Batch inserts (100 entries or 2s flush)
  - Level/source/time filters
  - Full-text search across message and raw log

## Build

Requires CGO + FTS5 support:

```bash
CGO_ENABLED=1 go build -tags fts5 -o bin/logstream ./cmd/logstream
```

## Usage

```bash
# Collect Docker logs
./bin/logstream -docker

# Tail a file
./bin/logstream -file /var/log/app.log

# Systemd logs
./bin/logstream -systemd

# Webhook receiver
./bin/logstream -webhook :8080

# Combine sources
./bin/logstream -docker -systemd -file /var/log/nginx/access.log
```

**Database:** `/tmp/logstream/logs.db` (default)

## Query Examples

```sql
-- Full-text search
SELECT source, message FROM logs 
WHERE id IN (SELECT rowid FROM logs_fts WHERE logs_fts MATCH 'error OR failed')
LIMIT 10;

-- By level
SELECT * FROM logs WHERE level = 'ERROR' ORDER BY timestamp DESC LIMIT 10;

-- By source + time
SELECT * FROM logs 
WHERE source LIKE 'docker:%' 
AND timestamp >= datetime('now', '-1 hour')
ORDER BY timestamp DESC;

-- Stats
SELECT level, COUNT(*) FROM logs GROUP BY level;
SELECT source, COUNT(*) FROM logs GROUP BY source;
```

## Tests

```bash
CGO_ENABLED=1 go test -tags fts5 ./...
```

**Test Coverage:**
- Parser tests: JSON, syslog, nginx, regex, auto-detect
- Store tests: insert, batch, FTS5 search, filters
- Collector tests: file tail, systemd (if journalctl available)

## Demo

Collected 41 logs from 5 Docker containers in 10 seconds:

```
$ ./bin/logstream -docker
2026/08/09 12:23:29 logstream started
^C
2026/08/09 12:23:39 Shutting down...
Total logs: 41

$ sqlite3 /tmp/logstream/logs.db 'SELECT COUNT(*), source FROM logs GROUP BY source'
1|docker:3f14ea2afafa
10|docker:417d9be844b7
10|docker:478765808b60
10|docker:790d76ea1d07
10|docker:d790a519bc50
```

## Requirements

- Go 1.20+
- CGO enabled
- Docker SDK (for Docker collector)
- systemd (for journalctl collector)
- inotify support (for file collector)

## Dependencies

- `github.com/mattn/go-sqlite3` - SQLite driver with FTS5
- `github.com/moby/moby/client` - Docker API
- `github.com/fsnotify/fsnotify` - File watch

## License

MIT
