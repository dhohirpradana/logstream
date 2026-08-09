package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type LogEntry struct {
	ID        int64             `json:"id"`
	Timestamp int64             `json:"timestamp"`
	Level     string            `json:"level"`
	Source    string            `json:"source"`
	Message   string            `json:"message"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type QueryFilter struct {
	Query       string
	TimeFrom    *time.Time
	TimeTo      *time.Time
	Levels      []string
	Sources     []string
	Limit       int
	Offset      int
	OrderByDesc bool
}

type DB struct {
	conn *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp INTEGER NOT NULL,
    level TEXT NOT NULL,
    source TEXT NOT NULL,
    message TEXT NOT NULL,
    metadata TEXT
);

CREATE INDEX IF NOT EXISTS idx_logs_timestamp ON logs(timestamp);
CREATE INDEX IF NOT EXISTS idx_logs_level ON logs(level);
CREATE INDEX IF NOT EXISTS idx_logs_source ON logs(source);
CREATE VIRTUAL TABLE IF NOT EXISTS logs_fts USING fts5(message, content=logs, content_rowid=id);

CREATE TRIGGER IF NOT EXISTS logs_ai AFTER INSERT ON logs BEGIN
  INSERT INTO logs_fts(rowid, message) VALUES (new.id, new.message);
END;

CREATE TRIGGER IF NOT EXISTS logs_ad AFTER DELETE ON logs BEGIN
  INSERT INTO logs_fts(logs_fts, rowid, message) VALUES('delete', old.id, old.message);
END;

CREATE TRIGGER IF NOT EXISTS logs_au AFTER UPDATE ON logs BEGIN
  INSERT INTO logs_fts(logs_fts, rowid, message) VALUES('delete', old.id, old.message);
  INSERT INTO logs_fts(rowid, message) VALUES (new.id, new.message);
END;
`

func Open(path string) (*DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if _, err := conn.Exec(schema); err != nil {
		conn.Close()
		return nil, err
	}

	return &DB{conn: conn}, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) Insert(entry *LogEntry) error {
	metadata := ""
	if len(entry.Metadata) > 0 {
		b, _ := json.Marshal(entry.Metadata)
		metadata = string(b)
	}

	result, err := db.conn.Exec(
		"INSERT INTO logs (timestamp, level, source, message, metadata) VALUES (?, ?, ?, ?, ?)",
		entry.Timestamp, entry.Level, entry.Source, entry.Message, metadata,
	)
	if err != nil {
		return err
	}

	entry.ID, _ = result.LastInsertId()
	return nil
}

func (db *DB) Search(filter QueryFilter) ([]LogEntry, error) {
	var query strings.Builder
	var args []interface{}

	if filter.Query != "" {
		query.WriteString(`
			SELECT l.id, l.timestamp, l.level, l.source, l.message, l.metadata
			FROM logs l
			JOIN logs_fts ON l.id = logs_fts.rowid
			WHERE logs_fts MATCH ?
		`)
		args = append(args, filter.Query)
	} else {
		query.WriteString("SELECT id, timestamp, level, source, message, metadata FROM logs WHERE 1=1")
	}

	if filter.TimeFrom != nil {
		query.WriteString(" AND timestamp >= ?")
		args = append(args, filter.TimeFrom.Unix())
	}
	if filter.TimeTo != nil {
		query.WriteString(" AND timestamp <= ?")
		args = append(args, filter.TimeTo.Unix())
	}
	if len(filter.Levels) > 0 {
		placeholders := strings.Repeat("?,", len(filter.Levels))
		placeholders = placeholders[:len(placeholders)-1]
		query.WriteString(fmt.Sprintf(" AND level IN (%s)", placeholders))
		for _, l := range filter.Levels {
			args = append(args, l)
		}
	}
	if len(filter.Sources) > 0 {
		placeholders := strings.Repeat("?,", len(filter.Sources))
		placeholders = placeholders[:len(placeholders)-1]
		query.WriteString(fmt.Sprintf(" AND source IN (%s)", placeholders))
		for _, s := range filter.Sources {
			args = append(args, s)
		}
	}

	if filter.OrderByDesc {
		query.WriteString(" ORDER BY timestamp DESC")
	} else {
		query.WriteString(" ORDER BY timestamp ASC")
	}

	if filter.Limit > 0 {
		query.WriteString(" LIMIT ?")
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query.WriteString(" OFFSET ?")
		args = append(args, filter.Offset)
	}

	rows, err := db.conn.Query(query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []LogEntry
	for rows.Next() {
		var e LogEntry
		var metadata sql.NullString
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Level, &e.Source, &e.Message, &metadata); err != nil {
			return nil, err
		}
		if metadata.Valid && metadata.String != "" {
			json.Unmarshal([]byte(metadata.String), &e.Metadata)
		}
		results = append(results, e)
	}

	return results, rows.Err()
}

func (db *DB) Recent(limit int) ([]LogEntry, error) {
	return db.Search(QueryFilter{Limit: limit, OrderByDesc: true})
}

func (db *DB) Stats() (map[string]interface{}, error) {
	var total, sources, levels int64
	var oldest, newest int64

	db.conn.QueryRow("SELECT COUNT(*) FROM logs").Scan(&total)
	db.conn.QueryRow("SELECT COUNT(DISTINCT source) FROM logs").Scan(&sources)
	db.conn.QueryRow("SELECT COUNT(DISTINCT level) FROM logs").Scan(&levels)
	db.conn.QueryRow("SELECT MIN(timestamp) FROM logs").Scan(&oldest)
	db.conn.QueryRow("SELECT MAX(timestamp) FROM logs").Scan(&newest)

	return map[string]interface{}{
		"total_logs":     total,
		"unique_sources": sources,
		"unique_levels":  levels,
		"oldest":         oldest,
		"newest":         newest,
	}, nil
}
