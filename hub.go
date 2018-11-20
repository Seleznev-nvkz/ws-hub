package main

import (
	"log"
	"strings"
)

type Hub struct {
	groups       map[string]*Group    // to send all clients in group by group name
	clients      map[string]*Client   // collect all clients in hub by sessionId
	extraClients map[*Client][]*Group // for faster delete from groups

	connectGroup chan *RedisData
	sendGroup    chan *RedisData

	connect    chan *Client
	disconnect chan Instance
}

func newHub() *Hub {
	return &Hub{
		groups:       make(map[string]*Group),
		clients:      make(map[string]*Client),
		extraClients: make(map[*Client][]*Group),
		connect:      make(chan *Client),
		disconnect:   make(chan Instance),
		connectGroup: make(chan *RedisData),
		sendGroup:    make(chan *RedisData, 1000), // arbitrary
	}
}

// create/update groups from redis' response; add client to group
func (h *Hub) groupsFromRedis(newGroups *RedisData) {
	client, ok := h.clients[newGroups.name]
	if !ok {
		log.Printf("Not found client for %s", newGroups.name)
		return
	}

	groupNames := strings.Split(string(newGroups.data), ",")
	h.extraClients[client] = make([]*Group, 0, len(groupNames))
	for _, groupName := range groupNames {
		group, ok := h.groups[groupName]
		if !ok {
			group = &Group{name: groupName, clients: make(map[*Client]struct{})}
			h.groups[groupName] = group
		}
		h.extraClients[client] = append(h.extraClients[client], group)
		group.clients[client] = struct{}{}
	}
}

func (h *Hub) run() {
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
			group, ok := hub.groups[dataToGroup.name]
			if ok {
				group.send(dataToGroup.data)
			} else {
				log.Printf("Not found group %s", dataToGroup.name)
			}
		case instance := <-h.disconnect:
			instance.delete()
		}
	}
}
