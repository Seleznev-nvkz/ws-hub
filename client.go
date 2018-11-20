package main

import (
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"time"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Instance interface{ delete() }

type Group struct {
	name    string
	clients map[*Client]struct{}
}

type Client struct {
	conn      *websocket.Conn
	sessionId string
	send      chan []byte
}

// delete client from hub and all groups
func (c *Client) delete() {
	c.conn.Close()
	if groups, ok := hub.extraClients[c]; ok {
		for _, group := range groups {
			group.deleteClient(c)
		}
		delete(hub.extraClients, c)
	}
	if _, ok := hub.clients[c.sessionId]; ok {
		delete(hub.clients, c.sessionId)
	}
}

func (g *Group) delete() {
	delete(hub.groups, g.name)
}

func (g *Group) deleteClient(c *Client) {
	for client := range g.clients {
		if c == client {
			delete(g.clients, client)
		}
	}
}

func (g *Group) send(data []byte) {
	if len(g.clients) > 0 {
		for client := range g.clients {
			client.send <- data
		}
	} else {
		hub.disconnect <- g
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

	conn.SetReadLimit(config.WebSocket.MaxMessageSize)
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(config.WebSocket.PongWait))
		return nil
	})
	client := &Client{sessionId: sessionId[0], conn: conn, send: make(chan []byte)}
	hub.connect <- client

	go client.readPump()
	go client.writePump()
}
