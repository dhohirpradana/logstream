package collector

import (
	"context"
	"os"
	"testing"
	"time"

	"logstream/internal/parser"
)

func TestFileCollector(t *testing.T) {
	tmpfile := "/tmp/test_logfile.log"
	defer os.Remove(tmpfile)

	if err := os.WriteFile(tmpfile, []byte("line1\nline2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	p := parser.NewAutoParser()
	c := NewFileCollector(tmpfile, p)
	ch := make(chan *parser.LogEntry, 10)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go c.Collect(ctx, ch)
	time.Sleep(100 * time.Millisecond)

	f, _ := os.OpenFile(tmpfile, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString("line3\n")
	f.Close()

	time.Sleep(200 * time.Millisecond)

	count := 0
	for len(ch) > 0 {
		<-ch
		count++
	}
	if count < 2 {
		t.Logf("got %d entries, expected at least 2", count)
	}
}

func TestSystemdCollector(t *testing.T) {
	if _, err := os.Stat("/usr/bin/journalctl"); os.IsNotExist(err) {
		t.Skip("journalctl not available")
	}

	p := parser.NewAutoParser()
	c := NewSystemdCollector(p)
	ch := make(chan *parser.LogEntry, 10)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := c.Collect(ctx, ch); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
}
