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

type Group struct {
	name    string
	clients map[*Client]struct{}
}

type Client struct {
	conn      *websocket.Conn
	sessionId string
	groups    map[*Group]struct{}
	send      chan []byte
}

func (c *Client) writePump() {
	ticket := time.NewTicker(config.WebSocket.PingPeriod)
	defer func() {
		ticket.Stop()
		c.conn.Close()
		hub.disconnect <- c
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(config.WebSocket.WriteWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Printf("writePump NextWriter error: %v", err)
				return
			}
			w.Write(message)
			if err := w.Close(); err != nil {
				log.Printf("writePump CloseWriter error: %v", err)
				return
			}
		case <-ticket.C:
			c.conn.SetWriteDeadline(time.Now().Add(config.WebSocket.WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("writePump PingMessage error: %v", err)
				return
			}
		}
	}
}

func (c *Client) readPump() {
	defer func() {
		// send to hub about user disc
		c.conn.Close()
		hub.disconnect <- c
	}()

	c.conn.SetReadLimit(config.WebSocket.MaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(config.WebSocket.PongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(config.WebSocket.PongWait))
		return nil
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("readPump error: %v", err)
			return
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
		send:      make(chan []byte, 256),
		groups:    make(map[*Group]struct{}),
	}
	hub.connect <- client

	go client.readPump()
	go client.writePump()
}
