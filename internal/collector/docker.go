package collector

import (
	"bufio"
	"context"

	"github.com/moby/moby/client"
	"logstream/internal/parser"
)

type DockerCollector struct {
	client *client.Client
	parser parser.Parser
}

func NewDockerCollector(p parser.Parser) (*DockerCollector, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &DockerCollector{client: cli, parser: p}, nil
}

func (d *DockerCollector) Collect(ctx context.Context, ch chan<- *parser.LogEntry) error {
	result, err := d.client.ContainerList(ctx, client.ContainerListOptions{})
	if err != nil {
		return err
	}
	for _, ctr := range result.Items {
		go d.streamContainer(ctx, ctr.ID, ch)
	}
	return nil
}

func (d *DockerCollector) streamContainer(ctx context.Context, containerID string, ch chan<- *parser.LogEntry) {
	opts := client.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Follow: true, Tail: "10", Timestamps: true}
	reader, err := d.client.ContainerLogs(ctx, containerID, opts)
	if err != nil {
		return
	}
	defer reader.Close()
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) > 8 && (line[0] == 1 || line[0] == 2) {
			line = line[8:]
		}
		if entry, err := d.parser.Parse(line); err == nil && entry != nil {
			entry.Source = "docker:" + containerID[:12]
			ch <- entry
		}
	}
}

func (d *DockerCollector) Close() error {
	return d.client.Close()
}
