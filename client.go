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

type ClientsRelations struct {
	m  map[*Client][]*Group
	mx sync.RWMutex
}

func NewClientsRelations() *ClientsRelations {
	return &ClientsRelations{
		m: make(map[*Client][]*Group),
	}
}

func (cr *ClientsRelations) Get(client *Client) ([]*Group, bool) {
	cr.mx.RLock()
	val, ok := cr.m[client]
	cr.mx.RUnlock()
	return val, ok
}

func (cr *ClientsRelations) Set(client *Client, groups []*Group) {
	cr.mx.Lock()
	cr.m[client] = groups
	cr.mx.Unlock()
}

func (cr *ClientsRelations) Delete(c *Client) {
	cr.mx.Lock()
	delete(cr.m, c)
	cr.mx.Unlock()
}

func (cr *ClientsRelations) Range() map[*Client][]*Group {
	cr.mx.RLock()
	defer cr.mx.RUnlock()
	return cr.m
}

type Group struct {
	name    string
	clients map[*Client]struct{}
}

func (g *Group) String() string {
	return fmt.Sprint(g.name, fmt.Sprint(g.clients))
}

type Client struct {
	conn      *websocket.Conn
	sessionId string
	send      chan []byte
}

func (c *Client) String() string {
	return c.sessionId
}

// total delete with closing conn
func (c *Client) delete() {
	c.deleteFromGroups()
	if v, ok := hub.clients[c.sessionId]; ok {
		if v == c {
			// check pointer - if user fast reconnected
			delete(hub.clients, c.sessionId)
			log.Println("Disconnected", c)
		}
	}
	c.conn.Close()
}

// delete client from all groups
// lookup all groups of user in clientsRelations and remove from all places
func (c *Client) deleteFromGroups() {
	if groups, ok := hub.clientsRelations.Get(c); ok {
		for _, group := range groups {
			if _, ok := group.clients[c]; ok {
				delete(group.clients, c)
			}
			if len(group.clients) < 1 {
				delete(hub.groups, group.name)
			}
		}
		hub.clientsRelations.Delete(c)
	}
}

func (g *Group) send(data []byte) {
	for client := range g.clients {
		client.send <- data
	}
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
			name: config.Redis.DataFromClient + c.sessionId,
			data: data,
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

	client := &Client{sessionId: sessionId[0], conn: conn, send: make(chan []byte)}
	hub.connect <- client

	go client.readPump()
	go client.writePump()
}
