package db

import (
	"os"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) *DB {
	tmpfile := "/tmp/test_db_" + t.Name() + ".db"
	os.Remove(tmpfile)
	
	db, err := Open(tmpfile)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		os.Remove(tmpfile)
	})
	
	return db
}

func TestInsert(t *testing.T) {
	db := setupTestDB(t)
	
	entry := &LogEntry{
		Timestamp: time.Now().Unix(),
		Level:     "INFO",
		Source:    "test",
		Message:   "test message",
		Metadata:  map[string]string{"key": "value"},
	}
	
	if err := db.Insert(entry); err != nil {
		t.Fatal(err)
	}
	
	if entry.ID == 0 {
		t.Error("expected ID to be set after insert")
	}
}

func TestSearch(t *testing.T) {
	db := setupTestDB(t)
	
	now := time.Now().Unix()
	entries := []LogEntry{
		{Timestamp: now, Level: "INFO", Source: "app", Message: "hello world"},
		{Timestamp: now + 1, Level: "ERROR", Source: "db", Message: "connection failed"},
		{Timestamp: now + 2, Level: "INFO", Source: "app", Message: "goodbye world"},
	}
	
	for i := range entries {
		if err := db.Insert(&entries[i]); err != nil {
			t.Fatal(err)
		}
	}
	
	tests := []struct {
		name      string
		filter    QueryFilter
		wantCount int
	}{
		{"all", QueryFilter{Limit: 10}, 3},
		{"text search", QueryFilter{Query: "world", Limit: 10}, 2},
		{"level filter", QueryFilter{Levels: []string{"ERROR"}, Limit: 10}, 1},
		{"source filter", QueryFilter{Sources: []string{"app"}, Limit: 10}, 2},
		{"limit", QueryFilter{Limit: 2}, 2},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := db.Search(tt.filter)
			if err != nil {
				t.Fatal(err)
			}
			
			if len(results) != tt.wantCount {
				t.Errorf("got %d results, want %d", len(results), tt.wantCount)
			}
		})
	}
}

func TestRecent(t *testing.T) {
	db := setupTestDB(t)
	
	for i := 0; i < 5; i++ {
		entry := &LogEntry{
			Timestamp: time.Now().Unix() + int64(i),
			Level:     "INFO",
			Source:    "test",
			Message:   "message",
		}
		db.Insert(entry)
	}
	
	results, err := db.Recent(3)
	if err != nil {
		t.Fatal(err)
	}
	
	if len(results) != 3 {
		t.Errorf("got %d results, want 3", len(results))
	}
	
	if results[0].Timestamp < results[1].Timestamp {
		t.Error("results not ordered by timestamp desc")
	}
}

func TestStats(t *testing.T) {
	db := setupTestDB(t)
	
	entries := []LogEntry{
		{Timestamp: 1000, Level: "INFO", Source: "app", Message: "one"},
		{Timestamp: 2000, Level: "ERROR", Source: "db", Message: "two"},
		{Timestamp: 3000, Level: "INFO", Source: "api", Message: "three"},
	}
	
	for i := range entries {
		db.Insert(&entries[i])
	}
	
	stats, err := db.Stats()
	if err != nil {
		t.Fatal(err)
	}
	
	if stats["total_logs"] != int64(3) {
		t.Errorf("got total_logs %v, want 3", stats["total_logs"])
	}
	if stats["unique_sources"] != int64(3) {
		t.Errorf("got unique_sources %v, want 3", stats["unique_sources"])
	}
	if stats["unique_levels"] != int64(2) {
		t.Errorf("got unique_levels %v, want 2", stats["unique_levels"])
	}
	if stats["oldest"] != int64(1000) {
		t.Errorf("got oldest %v, want 1000", stats["oldest"])
	}
	if stats["newest"] != int64(3000) {
		t.Errorf("got newest %v, want 3000", stats["newest"])
	}
}
