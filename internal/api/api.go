package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"logstream/internal/db"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
)

type Server struct {
	db       *db.DB
	router   chi.Router
	upgrader websocket.Upgrader
	tailSubs *tailManager
}

type tailManager struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]bool
}

func (tm *tailManager) add(conn *websocket.Conn) {
	tm.mu.Lock()
	tm.clients[conn] = true
	tm.mu.Unlock()
}

func (tm *tailManager) remove(conn *websocket.Conn) {
	tm.mu.Lock()
	delete(tm.clients, conn)
	tm.mu.Unlock()
}

func (tm *tailManager) broadcast(entry *db.LogEntry) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	for conn := range tm.clients {
		conn.WriteJSON(entry)
	}
}

func NewServer(database *db.DB) *Server {
	s := &Server{
		db:     database,
		router: chi.NewRouter(),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		tailSubs: &tailManager{clients: make(map[*websocket.Conn]bool)},
	}

	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.SetHeader("Content-Type", "application/json"))

	s.router.Route("/api/logs", func(r chi.Router) {
		r.Get("/search", s.handleSearch)
		r.Get("/recent", s.handleRecent)
		r.Get("/tail", s.handleTail)
		r.Get("/stats", s.handleStats)
		r.Post("/", s.handleCreate)
	})

	return s
}

func (s *Server) Router() chi.Router {
	return s.router
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := db.QueryFilter{
		Query:       q.Get("q"),
		Limit:       100,
		OrderByDesc: true,
	}

	if limit := q.Get("limit"); limit != "" {
		filter.Limit, _ = strconv.Atoi(limit)
	}
	if offset := q.Get("offset"); offset != "" {
		filter.Offset, _ = strconv.Atoi(offset)
	}
	if from := q.Get("from"); from != "" {
		if ts, err := strconv.ParseInt(from, 10, 64); err == nil {
			t := time.Unix(ts, 0)
			filter.TimeFrom = &t
		}
	}
	if to := q.Get("to"); to != "" {
		if ts, err := strconv.ParseInt(to, 10, 64); err == nil {
			t := time.Unix(ts, 0)
			filter.TimeTo = &t
		}
	}
	if levels := q.Get("levels"); levels != "" {
		filter.Levels = strings.Split(levels, ",")
	}
	if sources := q.Get("sources"); sources != "" {
		filter.Sources = strings.Split(sources, ",")
	}

	results, err := s.db.Search(filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
		"count":   len(results),
	})
}

func (s *Server) handleRecent(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}

	results, err := s.db.Recent(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
		"count":   len(results),
	})
}

func (s *Server) handleTail(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}
	defer conn.Close()

	s.tailSubs.add(conn)
	defer s.tailSubs.remove(conn)

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.db.Stats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var entry db.LogEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if entry.Timestamp == 0 {
		entry.Timestamp = time.Now().Unix()
	}

	if err := s.db.Insert(&entry); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.tailSubs.broadcast(&entry)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(entry)
}
