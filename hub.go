package main

import (
	"log"
	"strings"
)

type Hub struct {
	groups  map[string]*Group // to send all clients in group by group name
	clients *ClientMap        // collect all clients in hub by sessionId

	connectGroup chan *RedisData
	sendGroup    chan *RedisData

	connect    chan *Client
	disconnect chan *Client
}

func newHub() *Hub {
	return &Hub{
		clients:      NewClientMap(),
		groups:       make(map[string]*Group),
		connect:      make(chan *Client),
		disconnect:   make(chan *Client),
		connectGroup: make(chan *RedisData),
		sendGroup:    make(chan *RedisData, 1000), // arbitrary
	}
}

// create/update groups from redis' response; add client to group
func (h *Hub) groupsFromRedis(newGroups *RedisData) {

	client, ok := h.clients.Get(newGroups.channel)
	if !ok {
		log.Printf("Not found client for %s", newGroups.channel)
		return
	}

	// check empty response for client and disconnect
	if len(newGroups.data) == 0 {
		log.Printf("empty response for client - disconnect %s", newGroups.channel)
		h.clients.Delete(client)
		return
	}

	groupNames := strings.Split(string(newGroups.data), ",")
	groups := make(map[*Group]struct{})
	for _, groupName := range groupNames {
		group, ok := h.groups[groupName]
		if !ok {
			group = &Group{name: groupName, clients: make(map[*Client]struct{})}
			h.groups[groupName] = group
		}
		group.clients[client] = struct{}{}
		groups[group] = struct{}{}
	}
	h.clients.SetGroups(client, groups)
}

func (h *Hub) run() {
	log.Println("HUB started")

	for {
		select {
		case newGroups := <-h.connectGroup:
			h.groupsFromRedis(newGroups)
		case client := <-h.connect:
			h.clients.Add(client)
			redisHandler.pub <- &RedisData{
				channel: config.Redis.NewClient,
				data:    []byte(client.sessionId),
			}
		case dataToGroup := <-h.sendGroup:
			if group, ok := hub.groups[dataToGroup.channel]; ok {
				group.send(dataToGroup.data)
			}
		case client := <-h.disconnect:
			h.clients.Delete(client)
		}
	}
}
