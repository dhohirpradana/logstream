package parser

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

type LogEntry struct {
	Timestamp time.Time
	Level     string
	Message   string
	Source    string
	Raw       string
	Fields    map[string]string
}

type Parser interface {
	Parse(line string) (*LogEntry, error)
}

type JSONParser struct{}

func (p *JSONParser) Parse(line string) (*LogEntry, error) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(line), &data); err != nil {
		return nil, err
	}
	entry := &LogEntry{Raw: line, Fields: make(map[string]string)}
	if ts, ok := data["timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			entry.Timestamp = t
		}
	} else if ts, ok := data["time"].(string); ok {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			entry.Timestamp = t
		}
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	if lvl, ok := data["level"].(string); ok {
		entry.Level = lvl
	}
	if msg, ok := data["message"].(string); ok {
		entry.Message = msg
	} else if msg, ok := data["msg"].(string); ok {
		entry.Message = msg
	}
	for k, v := range data {
		if s, ok := v.(string); ok {
			entry.Fields[k] = s
		}
	}
	return entry, nil
}

type SyslogParser struct {
	re *regexp.Regexp
}

func NewSyslogParser() *SyslogParser {
	return &SyslogParser{re: regexp.MustCompile(`^<(\d+)>(\w{3}\s+\d+\s+\d+:\d+:\d+)\s+(\S+)\s+([^:]+):\s*(.*)$`)}
}

func (p *SyslogParser) Parse(line string) (*LogEntry, error) {
	matches := p.re.FindStringSubmatch(line)
	if matches == nil {
		return &LogEntry{Timestamp: time.Now(), Message: line, Raw: line, Fields: make(map[string]string)}, nil
	}
	ts, _ := time.Parse("Jan 2 15:04:05", matches[2])
	if ts.IsZero() {
		ts = time.Now()
	}
	return &LogEntry{Timestamp: ts, Source: matches[3], Message: matches[5], Raw: line,
		Fields: map[string]string{"priority": matches[1], "hostname": matches[3], "tag": matches[4]}}, nil
}

type NginxParser struct {
	re *regexp.Regexp
}

func NewNginxParser() *NginxParser {
	return &NginxParser{re: regexp.MustCompile(`^(\S+) - (\S+) \[([^\]]+)\] "([^"]+)" (\d+) (\d+) "([^"]*)" "([^"]*)"$`)}
}

func (p *NginxParser) Parse(line string) (*LogEntry, error) {
	matches := p.re.FindStringSubmatch(line)
	if matches == nil {
		return &LogEntry{Timestamp: time.Now(), Message: line, Raw: line, Fields: make(map[string]string)}, nil
	}
	ts, _ := time.Parse("02/Jan/2006:15:04:05 -0700", matches[3])
	if ts.IsZero() {
		ts = time.Now()
	}
	return &LogEntry{Timestamp: ts, Message: matches[4], Raw: line,
		Fields: map[string]string{"ip": matches[1], "user": matches[2], "request": matches[4],
			"status": matches[5], "bytes": matches[6], "referer": matches[7], "user_agent": matches[8]}}, nil
}

type RegexParser struct {
	re           *regexp.Regexp
	timestampIdx int
	messageIdx   int
	levelIdx     int
	timestampFmt string
}

func NewRegexParser(pattern string, timestampIdx, messageIdx, levelIdx int, timestampFmt string) (*RegexParser, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &RegexParser{re: re, timestampIdx: timestampIdx, messageIdx: messageIdx, levelIdx: levelIdx, timestampFmt: timestampFmt}, nil
}

func (p *RegexParser) Parse(line string) (*LogEntry, error) {
	matches := p.re.FindStringSubmatch(line)
	if matches == nil {
		return &LogEntry{Timestamp: time.Now(), Message: line, Raw: line, Fields: make(map[string]string)}, nil
	}
	entry := &LogEntry{Raw: line, Fields: make(map[string]string)}
	if p.timestampIdx > 0 && p.timestampIdx < len(matches) {
		if ts, err := time.Parse(p.timestampFmt, matches[p.timestampIdx]); err == nil {
			entry.Timestamp = ts
		}
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	if p.messageIdx > 0 && p.messageIdx < len(matches) {
		entry.Message = matches[p.messageIdx]
	}
	if p.levelIdx > 0 && p.levelIdx < len(matches) {
		entry.Level = matches[p.levelIdx]
	}
	return entry, nil
}

type AutoParser struct {
	parsers []Parser
}

func NewAutoParser() *AutoParser {
	return &AutoParser{parsers: []Parser{&JSONParser{}, NewSyslogParser(), NewNginxParser()}}
}

func (p *AutoParser) Parse(line string) (*LogEntry, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}
	for _, parser := range p.parsers {
		if entry, err := parser.Parse(line); err == nil && entry != nil {
			return entry, nil
		}
	}
	return &LogEntry{Timestamp: time.Now(), Message: line, Raw: line, Fields: make(map[string]string)}, nil
}
