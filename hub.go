package main

import (
	"log"
	"strings"
)

type Hub struct {
	groups           map[string]*Group  // to send all clients in group by group name
	clients          map[string]*Client // collect all clients in hub by sessionId
	clientsRelations *ClientsRelations  // for faster delete from groups

	connectGroup chan *RedisData
	sendGroup    chan *RedisData

	connect    chan *Client
	disconnect chan *Client
}

func newHub() *Hub {
	return &Hub{
		clientsRelations: NewClientsRelations(),
		groups:           make(map[string]*Group),
		clients:          make(map[string]*Client),
		connect:          make(chan *Client),
		disconnect:       make(chan *Client),
		connectGroup:     make(chan *RedisData),
		sendGroup:        make(chan *RedisData, 1000), // arbitrary
	}
}

// create/update groups from redis' response; add client to group
func (h *Hub) groupsFromRedis(newGroups *RedisData) {
	client, ok := h.clients[newGroups.name]
	if !ok {
		log.Printf("Not found client for %s", newGroups.name)
		return
	}

	// check empty response for client and disconnect
	if len(newGroups.data) == 0 {
		log.Printf("empty response for client - disconnect %s", newGroups.name)
		client.delete()
		return
	}

	// refresh existing client
	if _, ok := h.clientsRelations.Get(client); ok {
		log.Println("Refreshing", client)
		client.deleteFromGroups()
	}

	groupNames := strings.Split(string(newGroups.data), ",")
	allGroups := make([]*Group, 0, len(groupNames))
	for _, groupName := range groupNames {
		group, ok := h.groups[groupName]
		if !ok {
			group = &Group{name: groupName, clients: make(map[*Client]struct{})}
			h.groups[groupName] = group
		}
		allGroups = append(allGroups, group)
		group.clients[client] = struct{}{}
	}
	h.clientsRelations.Set(client, allGroups)
}

func (h *Hub) run() {
	log.Println("HUB started")

	for {
		select {
		case newGroups := <-h.connectGroup:
			h.groupsFromRedis(newGroups)
		case client := <-h.connect:
			h.clients[client.sessionId] = client
			redisHandler.pub <- &RedisData{
				name: config.Redis.NewClient,
				data: []byte(client.sessionId),
			}
		case dataToGroup := <-h.sendGroup:
			if group, ok := hub.groups[dataToGroup.name]; ok {
				group.send(dataToGroup.data)
			} else {
				log.Println("Not found group", dataToGroup.name)
			}
		case client := <-h.disconnect:
			client.delete()
		}
	}
}
