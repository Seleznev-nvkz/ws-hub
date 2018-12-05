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
	clientsCount := len(hub.clients)
	fmt.Fprintln(w, clientsCount)
}

func clientsPage(w http.ResponseWriter, _ *http.Request) {
	clients := make([]string, 0, len(hub.clients))
	for key := range hub.clients {
		clients = append(clients, key)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clients)
}

func main() {
	redisHandler.run()
	go hub.run()

	router := http.NewServeMux()
	router.HandleFunc("/status", statusPage)
	router.HandleFunc("/clients", clientsPage)
	router.HandleFunc(config.ServerUrl, serveWS)

	err := http.ListenAndServe(config.Address, trailingSlashesMiddleware(router))
	if err != nil {
		log.Fatal(err)
	}
}
