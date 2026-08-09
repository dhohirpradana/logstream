package store

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
	"logstream/internal/parser"
)

type Store struct {
	db *sql.DB
}

func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.init(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		level TEXT,
		message TEXT,
		source TEXT,
		raw TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_logs_timestamp ON logs(timestamp);
	CREATE INDEX IF NOT EXISTS idx_logs_level ON logs(level);
	CREATE INDEX IF NOT EXISTS idx_logs_source ON logs(source);
	CREATE VIRTUAL TABLE IF NOT EXISTS logs_fts USING fts5(message, raw, content=logs, content_rowid=id);
	CREATE TRIGGER IF NOT EXISTS logs_ai AFTER INSERT ON logs BEGIN
		INSERT INTO logs_fts(rowid, message, raw) VALUES (new.id, new.message, new.raw);
	END;
	CREATE TRIGGER IF NOT EXISTS logs_ad AFTER DELETE ON logs BEGIN
		INSERT INTO logs_fts(logs_fts, rowid, message, raw) VALUES('delete', old.id, old.message, old.raw);
	END;
	CREATE TRIGGER IF NOT EXISTS logs_au AFTER UPDATE ON logs BEGIN
		INSERT INTO logs_fts(logs_fts, rowid, message, raw) VALUES('delete', old.id, old.message, old.raw);
		INSERT INTO logs_fts(rowid, message, raw) VALUES (new.id, new.message, new.raw);
	END;
	`
	_, err := s.db.Exec(schema)
	return err
}

func (s *Store) Insert(entry *parser.LogEntry) error {
	_, err := s.db.Exec(`INSERT INTO logs (timestamp, level, message, source, raw) VALUES (?, ?, ?, ?, ?)`,
		entry.Timestamp, entry.Level, entry.Message, entry.Source, entry.Raw)
	return err
}

func (s *Store) InsertBatch(entries []*parser.LogEntry) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO logs (timestamp, level, message, source, raw) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, entry := range entries {
		if _, err := stmt.Exec(entry.Timestamp, entry.Level, entry.Message, entry.Source, entry.Raw); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type SearchOptions struct {
	Query     string
	Level     string
	Source    string
	StartTime *time.Time
	EndTime   *time.Time
	Limit     int
}

func (s *Store) Search(opts SearchOptions) ([]*parser.LogEntry, error) {
	var query string
	var args []interface{}
	if opts.Query != "" {
		query = `SELECT l.id, l.timestamp, l.level, l.message, l.source, l.raw FROM logs_fts f JOIN logs l ON l.id = f.rowid WHERE logs_fts MATCH ?`
		args = append(args, opts.Query)
	} else {
		query = `SELECT id, timestamp, level, message, source, raw FROM logs WHERE 1=1`
	}
	if opts.Level != "" {
		query += " AND level = ?"
		args = append(args, opts.Level)
	}
	if opts.Source != "" {
		query += " AND source = ?"
		args = append(args, opts.Source)
	}
	if opts.StartTime != nil {
		query += " AND timestamp >= ?"
		args = append(args, opts.StartTime)
	}
	if opts.EndTime != nil {
		query += " AND timestamp <= ?"
		args = append(args, opts.EndTime)
	}
	query += " ORDER BY timestamp DESC"
	if opts.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, opts.Limit)
	} else {
		query += " LIMIT 100"
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []*parser.LogEntry
	for rows.Next() {
		var id int
		entry := &parser.LogEntry{Fields: make(map[string]string)}
		if err := rows.Scan(&id, &entry.Timestamp, &entry.Level, &entry.Message, &entry.Source, &entry.Raw); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (s *Store) Stats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&total); err != nil {
		return nil, err
	}
	stats["total"] = total
	rows, err := s.db.Query("SELECT level, COUNT(*) FROM logs GROUP BY level")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	levels := make(map[string]int)
	for rows.Next() {
		var level string
		var count int
		if err := rows.Scan(&level, &count); err != nil {
			return nil, err
		}
		levels[level] = count
	}
	stats["by_level"] = levels
	return stats, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}
