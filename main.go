package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

var (
	config       *Config
	redisHandler *RedisHandler
	hub          *Hub
)

func init() {
	config = newConfig()
	hub = newHub()
	redisHandler = newRedisHandler(config.Redis.Address)
}

func trailingSlashesMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimSuffix(r.URL.Path, "/")
		next.ServeHTTP(w, r)
	})
}

func statusPage(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintln(w, len(hub.connections.clients))
}

func clientsView(w http.ResponseWriter, _ *http.Request) {
	jsonData, err := json.Marshal(hub.connections.getClientDetails())
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonData)
	} else {
		w.Write([]byte(err.Error()))
	}
}

func groupsView(w http.ResponseWriter, _ *http.Request) {
	jsonData, err := json.Marshal(hub.connections.getGroupDetails())
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonData)
	} else {
		w.Write([]byte(err.Error()))
	}
}

func sessionsView(w http.ResponseWriter, _ *http.Request) {
	jsonData, err := json.Marshal(hub.connections.getSessions())
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonData)
	} else {
		w.Write([]byte(err.Error()))
	}
}

func main() {
	redisHandler.run()
	go hub.run()

	router := http.NewServeMux()
	router.HandleFunc("/status", statusPage)
	router.HandleFunc("/clients", clientsView)
	router.HandleFunc("/groups", groupsView)
	router.HandleFunc("/sessions", sessionsView)
	router.HandleFunc(config.ServerUrl, serveWS)

	server := http.Server{
		Addr:         config.Address,
		Handler:      trailingSlashesMiddleware(router),
		ReadTimeout:  config.WebSocket.PongWait,
		WriteTimeout: config.WebSocket.WriteWait,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
