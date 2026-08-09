package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"logstream/internal/collector"
	"logstream/internal/parser"
	"logstream/internal/store"
)

func main() {
	dbPath := flag.String("db", "/tmp/logstream/logs.db", "SQLite database path")
	dockerFlag := flag.Bool("docker", false, "Collect Docker logs")
	systemdFlag := flag.Bool("systemd", false, "Collect systemd logs")
	fileFlag := flag.String("file", "", "File path to tail")
	webhookFlag := flag.String("webhook", "", "Webhook listen address (e.g., :8080)")
	flag.Parse()

	os.MkdirAll("/tmp/logstream", 0755)

	st, err := store.New(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	p := parser.NewAutoParser()
	ch := make(chan *parser.LogEntry, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if *dockerFlag {
		dc, err := collector.NewDockerCollector(p)
		if err != nil {
			log.Printf("Docker collector error: %v", err)
		} else {
			go func() {
				if err := dc.Collect(ctx, ch); err != nil {
					log.Printf("Docker collect error: %v", err)
				}
			}()
			defer dc.Close()
		}
	}

	if *systemdFlag {
		sc := collector.NewSystemdCollector(p)
		go func() {
			if err := sc.Collect(ctx, ch); err != nil {
				log.Printf("Systemd collect error: %v", err)
			}
		}()
	}

	if *fileFlag != "" {
		fc := collector.NewFileCollector(*fileFlag, p)
		go func() {
			if err := fc.Collect(ctx, ch); err != nil {
				log.Printf("File collect error: %v", err)
			}
		}()
	}

	if *webhookFlag != "" {
		wc := collector.NewWebhookCollector(*webhookFlag, p)
		go func() {
			if err := wc.Collect(ctx, ch); err != nil {
				log.Printf("Webhook collect error: %v", err)
			}
		}()
	}

	batch := make([]*parser.LogEntry, 0, 100)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	log.Println("logstream started")

	for {
		select {
		case entry := <-ch:
			if entry != nil {
				batch = append(batch, entry)
				if len(batch) >= 100 {
					if err := st.InsertBatch(batch); err != nil {
						log.Printf("Insert error: %v", err)
					}
					batch = batch[:0]
				}
			}
		case <-ticker.C:
			if len(batch) > 0 {
				if err := st.InsertBatch(batch); err != nil {
					log.Printf("Insert error: %v", err)
				}
				batch = batch[:0]
			}
		case <-sigCh:
			log.Println("Shutting down...")
			cancel()
			if len(batch) > 0 {
				st.InsertBatch(batch)
			}
			stats, _ := st.Stats()
			fmt.Printf("Total logs: %v\n", stats["total"])
			return
		}
	}
}
