package main

import (
	"database/sql"
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/log"

	"github.com/jacobdanielrose/bytetalk/internal/database"
	"github.com/jacobdanielrose/bytetalk/internal/middleware"
	"github.com/jacobdanielrose/bytetalk/internal/realtime/ws"
	_ "github.com/joho/godotenv/autoload"
	_ "github.com/lib/pq"
)

func main() {
	dbURL, ok := os.LookupEnv("DATABASE_URL")
	if ok == false {
		log.Fatal("Failed to get DATABASE_URL")
	}

	DB, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("Failed to open database", "error", err)
	}
	db := database.New(DB)

	hub := ws.NewHub(db)
	go hub.Run()

	router := http.NewServeMux()

	router.HandleFunc("GET /users/{name}", func(w http.ResponseWriter, r *http.Request) { getUser(w, r, db) })
	router.HandleFunc("POST /users/", func(w http.ResponseWriter, r *http.Request) { createUser(w, r, db) })

	router.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws.ServeWs(hub, w, r)
	})

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      middleware.Logging(router),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Info("WebSocket server listening", "listenaddr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
