package main

import "log"

type Hub struct {
	connections *ConnectionsMap

	connectGroup chan *RedisData
	sendGroup    chan *RedisData

	connect    chan *Client
	disconnect chan *Client
}

func newHub() *Hub {
	return &Hub{
		connections:  NewConnectionsMap(),
		connect:      make(chan *Client, 100), // arbitrary
		disconnect:   make(chan *Client, 100),
		connectGroup: make(chan *RedisData, 100),
		sendGroup:    make(chan *RedisData),
	}
}

func (h *Hub) run() {
	log.Println("HUB started")

	for {
		select {
		case newGroups := <-h.connectGroup:
			h.connections.SetGroupsFromRedis(newGroups)

		case client := <-h.connect:
			h.connections.AddClient(client)

		case dataToGroup := <-h.sendGroup:
			h.connections.SendDataFromRedis(dataToGroup)

		case client := <-h.disconnect:
			h.connections.DeleteClient(client)
		}
	}
}
