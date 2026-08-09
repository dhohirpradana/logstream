package parser

import "testing"

func TestJSONParser(t *testing.T) {
	p := &JSONParser{}
	line := `{"timestamp":"2026-08-09T12:00:00Z","level":"INFO","message":"test log"}`
	entry, err := p.Parse(line)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Level != "INFO" {
		t.Errorf("expected INFO, got %s", entry.Level)
	}
	if entry.Message != "test log" {
		t.Errorf("expected 'test log', got %s", entry.Message)
	}
}

func TestSyslogParser(t *testing.T) {
	p := NewSyslogParser()
	line := `<134>Aug 9 12:00:00 myhost myapp: test message`
	entry, err := p.Parse(line)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Source != "myhost" {
		t.Errorf("expected myhost, got %s", entry.Source)
	}
	if entry.Message != "test message" {
		t.Errorf("expected 'test message', got %s", entry.Message)
	}
}

func TestNginxParser(t *testing.T) {
	p := NewNginxParser()
	line := `192.168.1.1 - - [09/Aug/2026:12:00:00 +0700] "GET / HTTP/1.1" 200 1234 "-" "curl/7.64.1"`
	entry, err := p.Parse(line)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Fields["ip"] != "192.168.1.1" {
		t.Errorf("expected 192.168.1.1, got %s", entry.Fields["ip"])
	}
	if entry.Fields["status"] != "200" {
		t.Errorf("expected 200, got %s", entry.Fields["status"])
	}
}

func TestRegexParser(t *testing.T) {
	p, err := NewRegexParser(`(\d{4}-\d{2}-\d{2}) (\w+) (.+)`, 1, 3, 2, "2006-01-02")
	if err != nil {
		t.Fatal(err)
	}
	line := "2026-08-09 ERROR something failed"
	entry, err := p.Parse(line)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Level != "ERROR" {
		t.Errorf("expected ERROR, got %s", entry.Level)
	}
	if entry.Message != "something failed" {
		t.Errorf("expected 'something failed', got %s", entry.Message)
	}
}

func TestAutoParser(t *testing.T) {
	p := NewAutoParser()
	tests := []struct {
		name string
		line string
	}{
		{"json", `{"level":"INFO","message":"test"}`},
		{"syslog", `<134>Aug 9 12:00:00 host app: msg`},
		{"nginx", `127.0.0.1 - - [09/Aug/2026:12:00:00 +0700] "GET / HTTP/1.1" 200 123 "-" "-"`},
		{"plain", "plain text log"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := p.Parse(tt.line)
			if err != nil {
				t.Fatal(err)
			}
			if entry == nil {
				t.Fatal("nil entry")
			}
			if entry.Raw != tt.line {
				t.Errorf("raw mismatch")
			}
		})
	}
}
