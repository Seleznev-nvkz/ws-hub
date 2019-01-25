package main

import (
	"log"
	"strings"
	"sync"
)

type ConnectionsMap struct {
	groups  map[string]*Group // to send all clients in group by group name
	clients map[string]*Client
	sync.RWMutex
}

func NewConnectionsMap() *ConnectionsMap {
	return &ConnectionsMap{
		groups:  make(map[string]*Group),
		clients: make(map[string]*Client),
	}
}

func (cm *ConnectionsMap) GetClientBySession(sessionId string) (*Client, bool) {
	cm.RLock()
	val, ok := cm.clients[sessionId]
	cm.RUnlock()
	return val, ok
}

func (cm *ConnectionsMap) AddClient(client *Client) {
	cm.Lock()
	cm.clients[client.sessionId] = client
	cm.Unlock()

	redisHandler.pub <- &RedisData{
		channel: config.Redis.NewClient,
		data:    []byte(client.sessionId),
	}
}

// get/create groups from list of names and add to client
func (cm *ConnectionsMap) UpdateOrCreateGroups(names []string, c *Client) {
	cm.Lock()
	defer cm.Unlock()

	for _, name := range names {
		group, ok := cm.groups[name]
		if !ok {
			group = &Group{name: name, clients: make(map[*Client]struct{})}
			cm.groups[name] = group
		}
		// add relation client<->group
		group.clients[c] = struct{}{}
		c.groups[group] = struct{}{}
	}
}

// create/update groups from redis' response; add client to group
func (cm *ConnectionsMap) SetGroupsFromRedis(redisData *RedisData) {
	// check that client connected
	client, ok := cm.GetClientBySession(redisData.channel)
	if !ok {
		log.Printf("Not found client for %s", redisData.channel)
		return
	}

	// check empty response for client and disconnect
	if len(redisData.data) == 0 {
		log.Printf("empty response for client - disconnect %s", redisData.channel)
		cm.DeleteClient(client)
		return
	}

	cm.UpdateOrCreateGroups(strings.Split(string(redisData.data), ","), client)
}

func (cm *ConnectionsMap) SendDataFromRedis(redisData *RedisData) {
	cm.RLock()
	defer cm.RUnlock()

	if group, ok := cm.groups[redisData.channel]; ok {
		for client := range group.clients {
			client.send <- redisData.data
		}
	}
}

// remove client from all groups
func (cm *ConnectionsMap) DeleteClient(c *Client) {
	cm.Lock()
	defer cm.Unlock()

	if v, ok := cm.clients[c.sessionId]; ok && v == c {
		for group := range c.groups {
			if _, ok := group.clients[c]; ok {
				delete(group.clients, c)
			}
			// remove empty group
			if len(group.clients) < 1 {
				delete(cm.groups, group.name)
			}
		}
		delete(cm.clients, c.sessionId)
		c.conn.Close()
	}
}

// return map with names of clients and groups
func (cm *ConnectionsMap) getDetails() map[string][]string {
	res := map[string][]string{}
	cm.RLock()
	defer cm.RUnlock()

	for session, client := range cm.clients {
		res[session] = make([]string, 0, len(client.groups))
		for group := range client.groups {
			res[session] = append(res[session], group.name)
		}
	}
	return res
}
