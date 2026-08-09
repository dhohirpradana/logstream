package collector

import (
	"context"
	"io"
	"net/http"

	"logstream/internal/parser"
)

type WebhookCollector struct {
	addr   string
	parser parser.Parser
	server *http.Server
}

func NewWebhookCollector(addr string, p parser.Parser) *WebhookCollector {
	return &WebhookCollector{addr: addr, parser: p}
}

func (w *WebhookCollector) Collect(ctx context.Context, ch chan<- *parser.LogEntry) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(rw, "bad request", http.StatusBadRequest)
			return
		}
		if entry, err := w.parser.Parse(string(body)); err == nil && entry != nil {
			entry.Source = "webhook:" + r.RemoteAddr
			ch <- entry
		}
		rw.WriteHeader(http.StatusOK)
	})
	w.server = &http.Server{Addr: w.addr, Handler: mux}
	go func() {
		<-ctx.Done()
		w.server.Shutdown(context.Background())
	}()
	return w.server.ListenAndServe()
}
