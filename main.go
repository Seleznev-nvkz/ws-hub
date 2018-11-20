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

	http.HandleFunc(config.ServerUrl, serveWS)
	http.HandleFunc("/status", statusPage)

	err := http.ListenAndServe(config.Address, nil)
	if err != nil {
		log.Fatal(err)
	}
}
