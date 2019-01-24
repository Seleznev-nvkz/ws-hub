package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
	"time"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type ClientMap struct {
	m  map[string]*Client
	mx sync.RWMutex
}

func NewClientMap() *ClientMap {
	return &ClientMap{
		m: make(map[string]*Client),
	}
}

func (cm *ClientMap) Get(sessionId string) (*Client, bool) {
	cm.mx.RLock()
	val, ok := cm.m[sessionId]
	cm.mx.RUnlock()
	return val, ok
}

func (cm *ClientMap) Add(client *Client) {
	cm.mx.Lock()
	cm.m[client.sessionId] = client
	cm.mx.Unlock()
}

func (cm *ClientMap) SetGroups(client *Client, groups map[*Group]struct{}) {
	cm.mx.Lock()
	client.groups = groups
	cm.mx.Unlock()
}

// remove client from all groups
func (cm *ClientMap) Delete(c *Client) {
	cm.mx.Lock()
	defer cm.mx.Unlock()

	if v, ok := cm.m[c.sessionId]; ok && v == c {
		for group := range c.groups {
			if _, ok := group.clients[c]; ok {
				delete(group.clients, c)
			}
			// remove empty group
			if len(group.clients) < 1 {
				delete(hub.groups, group.name)
			}
		}
		delete(cm.m, c.sessionId)
		c.conn.Close()
	}
}

func (cm *ClientMap) getDetails() map[string][]string {
	res := map[string][]string{}
	cm.mx.RLock()
	defer cm.mx.RUnlock()

	for session, client := range cm.m {
		res[session] = make([]string, 0, len(client.groups))
		for group := range client.groups {
			res[session] = append(res[session], group.name)
		}
	}
	return res
}

type Group struct {
	name    string
	clients map[*Client]struct{}
}

func (g *Group) String() string {
	return fmt.Sprint(g.name, fmt.Sprint(g.clients))
}

func (g *Group) send(data []byte) {
	for client := range g.clients {
		client.send <- data
	}
}

type Client struct {
	conn      *websocket.Conn
	sessionId string
	groups    map[*Group]struct{}
	send      chan []byte
}

func (c *Client) String() string {
	return c.sessionId
}

func (c *Client) writePump() {
	ticket := time.NewTicker(config.WebSocket.PingPeriod)
	defer func() {
		hub.disconnect <- c
		ticket.Stop()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(config.WebSocket.WriteWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			err := c.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				return
			}

		case <-ticket.C:
			c.conn.SetWriteDeadline(time.Now().Add(config.WebSocket.WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) readPump() {
	defer func() {
		// send to hub about user disc
		hub.disconnect <- c
	}()

	c.conn.SetReadLimit(config.WebSocket.MaxMessageSize)
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(config.WebSocket.PongWait))
		return nil
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("error: %v", err)
			break
		}
		redisHandler.pub <- &RedisData{
			channel: config.Redis.DataFromClient + c.sessionId,
			data:    data,
		}
	}
}

func serveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	sessionId, ok := r.URL.Query()[config.ClientKey]
	if !ok || len(sessionId) < 1 {
		log.Println("not found session_key in query")
		return
	}

	client := &Client{
		sessionId: sessionId[0],
		conn:      conn,
		send:      make(chan []byte),
		groups:    make(map[*Group]struct{}),
	}
	hub.connect <- client

	go client.readPump()
	go client.writePump()
}
