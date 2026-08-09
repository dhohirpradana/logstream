package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"logstream/internal/db"
)

func setupTestDB(t *testing.T) *db.DB {
	tmpfile := "/tmp/test_logstream_" + t.Name() + ".db"
	os.Remove(tmpfile)
	
	database, err := db.Open(tmpfile)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		database.Close()
		os.Remove(tmpfile)
	})
	
	return database
}

func TestSearch(t *testing.T) {
	database := setupTestDB(t)
	server := NewServer(database)
	
	entries := []db.LogEntry{
		{Timestamp: time.Now().Unix(), Level: "INFO", Source: "app", Message: "test message one"},
		{Timestamp: time.Now().Unix(), Level: "ERROR", Source: "db", Message: "connection failed"},
		{Timestamp: time.Now().Unix(), Level: "INFO", Source: "app", Message: "test message two"},
	}
	
	for i := range entries {
		if err := database.Insert(&entries[i]); err != nil {
			t.Fatal(err)
		}
	}
	
	tests := []struct {
		name       string
		query      string
		wantCount  int
	}{
		{"no filter", "", 3},
		{"text search", "?q=connection", 1},
		{"level filter", "?levels=INFO", 2},
		{"source filter", "?sources=db", 1},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/logs/search"+tt.query, nil)
			w := httptest.NewRecorder()
			
			server.handleSearch(w, req)
			
			if w.Code != http.StatusOK {
				t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
			}
			
			var resp map[string]interface{}
			json.NewDecoder(w.Body).Decode(&resp)
			
			count := int(resp["count"].(float64))
			if count != tt.wantCount {
				t.Errorf("got count %d, want %d", count, tt.wantCount)
			}
		})
	}
}

func TestRecent(t *testing.T) {
	database := setupTestDB(t)
	server := NewServer(database)
	
	for i := 0; i < 5; i++ {
		entry := db.LogEntry{
			Timestamp: time.Now().Unix() + int64(i),
			Level:     "INFO",
			Source:    "test",
			Message:   "message",
		}
		database.Insert(&entry)
	}
	
	req := httptest.NewRequest("GET", "/api/logs/recent?limit=3", nil)
	w := httptest.NewRecorder()
	
	server.handleRecent(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", w.Code)
	}
	
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	
	count := int(resp["count"].(float64))
	if count != 3 {
		t.Errorf("got count %d, want 3", count)
	}
}

func TestCreate(t *testing.T) {
	database := setupTestDB(t)
	server := NewServer(database)
	
	entry := db.LogEntry{
		Level:   "INFO",
		Source:  "test",
		Message: "new log entry",
	}
	
	body, _ := json.Marshal(entry)
	req := httptest.NewRequest("POST", "/api/logs/", bytes.NewReader(body))
	w := httptest.NewRecorder()
	
	server.handleCreate(w, req)
	
	if w.Code != http.StatusCreated {
		t.Errorf("got status %d, want 201", w.Code)
	}
	
	var created db.LogEntry
	json.NewDecoder(w.Body).Decode(&created)
	
	if created.ID == 0 {
		t.Error("expected ID to be set")
	}
	if created.Timestamp == 0 {
		t.Error("expected timestamp to be set")
	}
}

func TestStats(t *testing.T) {
	database := setupTestDB(t)
	server := NewServer(database)
	
	entries := []db.LogEntry{
		{Timestamp: 1000, Level: "INFO", Source: "app", Message: "one"},
		{Timestamp: 2000, Level: "ERROR", Source: "db", Message: "two"},
		{Timestamp: 3000, Level: "INFO", Source: "app", Message: "three"},
	}
	
	for i := range entries {
		database.Insert(&entries[i])
	}
	
	req := httptest.NewRequest("GET", "/api/logs/stats", nil)
	w := httptest.NewRecorder()
	
	server.handleStats(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", w.Code)
	}
	
	var stats map[string]interface{}
	json.NewDecoder(w.Body).Decode(&stats)
	
	if int(stats["total_logs"].(float64)) != 3 {
		t.Errorf("got total_logs %v, want 3", stats["total_logs"])
	}
	if int(stats["unique_sources"].(float64)) != 2 {
		t.Errorf("got unique_sources %v, want 2", stats["unique_sources"])
	}
}
