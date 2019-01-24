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
	clientsCount := len(hub.clients.m)
	fmt.Fprintln(w, clientsCount)
}

func detailsView(w http.ResponseWriter, _ *http.Request) {
	jsonData, err := json.Marshal(hub.clients.getDetails())
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
	router.HandleFunc("/details", detailsView)
	router.HandleFunc(config.ServerUrl, serveWS)

	err := http.ListenAndServe(config.Address, trailingSlashesMiddleware(router))
	if err != nil {
		log.Fatal(err)
	}
}
