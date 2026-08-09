package collector

import (
	"bufio"
	"context"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	"logstream/internal/parser"
)

type FileCollector struct {
	path   string
	parser parser.Parser
}

func NewFileCollector(path string, p parser.Parser) *FileCollector {
	return &FileCollector{path: path, parser: p}
}

func (f *FileCollector) Collect(ctx context.Context, ch chan<- *parser.LogEntry) error {
	file, err := os.Open(f.path)
	if err != nil {
		return err
	}
	defer file.Close()
	stat, _ := file.Stat()
	if stat.Size() > 0 {
		file.Seek(-min(stat.Size(), 4096), os.SEEK_END)
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if entry, err := f.parser.Parse(line); err == nil && entry != nil {
			entry.Source = "file:" + f.path
			ch <- entry
		}
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()
	if err := watcher.Add(f.path); err != nil {
		return err
	}
	pos, _ := file.Seek(0, os.SEEK_CUR)
	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-watcher.Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				file.Seek(pos, os.SEEK_SET)
				scanner := bufio.NewScanner(file)
				for scanner.Scan() {
					line := scanner.Text()
					if entry, err := f.parser.Parse(line); err == nil && entry != nil {
						entry.Source = "file:" + f.path
						ch <- entry
					}
				}
				pos, _ = file.Seek(0, os.SEEK_CUR)
			}
		case <-time.After(time.Second):
		}
	}
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
