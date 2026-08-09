package store

import (
	"os"
	"testing"
	"time"

	"logstream/internal/parser"
)

func TestStore(t *testing.T) {
	tmpfile := "/tmp/test_logstream.db"
	defer os.Remove(tmpfile)

	s, err := New(tmpfile)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	entry := &parser.LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   "test message",
		Source:    "test",
		Raw:       "raw log line",
		Fields:    make(map[string]string),
	}

	if err := s.Insert(entry); err != nil {
		t.Fatal(err)
	}

	results, err := s.Search(SearchOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Message != "test message" {
		t.Errorf("expected 'test message', got %s", results[0].Message)
	}

	results, err = s.Search(SearchOptions{Query: "test", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 FTS result, got %d", len(results))
	}

	stats, err := s.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if stats["total"].(int) != 1 {
		t.Errorf("expected total=1, got %v", stats["total"])
	}
}

func TestStoreBatch(t *testing.T) {
	tmpfile := "/tmp/test_logstream_batch.db"
	defer os.Remove(tmpfile)

	s, err := New(tmpfile)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	entries := []*parser.LogEntry{
		{Timestamp: time.Now(), Level: "INFO", Message: "msg1", Raw: "raw1", Fields: make(map[string]string)},
		{Timestamp: time.Now(), Level: "ERROR", Message: "msg2", Raw: "raw2", Fields: make(map[string]string)},
		{Timestamp: time.Now(), Level: "INFO", Message: "msg3", Raw: "raw3", Fields: make(map[string]string)},
	}

	if err := s.InsertBatch(entries); err != nil {
		t.Fatal(err)
	}

	results, err := s.Search(SearchOptions{Level: "INFO", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 INFO logs, got %d", len(results))
	}
}
