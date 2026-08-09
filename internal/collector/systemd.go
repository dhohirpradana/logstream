package collector

import (
	"bufio"
	"context"
	"os/exec"

	"logstream/internal/parser"
)

type SystemdCollector struct {
	parser parser.Parser
}

func NewSystemdCollector(p parser.Parser) *SystemdCollector {
	return &SystemdCollector{parser: p}
}

func (s *SystemdCollector) Collect(ctx context.Context, ch chan<- *parser.LogEntry) error {
	cmd := exec.CommandContext(ctx, "journalctl", "-f", "-o", "short-iso", "-n", "10")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		defer cmd.Wait()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if entry, err := s.parser.Parse(line); err == nil && entry != nil {
				entry.Source = "systemd"
				ch <- entry
			}
		}
	}()
	return nil
}
