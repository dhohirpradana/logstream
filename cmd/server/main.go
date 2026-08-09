package main

import (
	"flag"
	"log"
	"net/http"

	"logstream/internal/api"
	"logstream/internal/db"
)

func main() {
	dbPath := flag.String("db", "/tmp/logstream/logs.db", "SQLite database path")
	port := flag.String("port", "8080", "HTTP port")
	flag.Parse()

	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer database.Close()

	server := api.NewServer(database)

	log.Printf("Starting server on :%s", *port)
	log.Fatal(http.ListenAndServe(":"+*port, server.Router()))
}
