package main

import (
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
	groups := make([]string, 0, len(hub.groups))
	for key := range hub.groups {
		groups = append(groups, key)
	}

	fmt.Fprintf(w, "Clients count %v\n\nGroups:\n\t%s", clientsCount, strings.Join(groups, "\n\t"))
}

func main() {
	redisHandler.run()
	go hub.run()

	router := http.NewServeMux()
	router.HandleFunc(config.ServerUrl, serveWS)
	router.HandleFunc("/status", statusPage)

	err := http.ListenAndServe(config.Address, trailingSlashesMiddleware(router))
	if err != nil {
		log.Fatal(err)
	}
}
