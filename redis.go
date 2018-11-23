package main

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	"log"
	"strings"
	"time"
)

type RedisData struct {
	name string
	data []byte
}

func (r *RedisData) String() string {
	return fmt.Sprint(r.name, string(r.data))
}

type RedisHandler struct {
	*redis.Pool
	pub chan *RedisData
}

func newRedisHandler(addr string) *RedisHandler {
	return &RedisHandler{
		Pool: &redis.Pool{
			MaxIdle:     config.Redis.IdleConn,
			IdleTimeout: config.Redis.Timeout,
			Dial: func() (redis.Conn, error) {
				return redis.Dial("tcp", addr)
			},
			TestOnBorrow: func(c redis.Conn, t time.Time) error {
				if time.Since(t) < time.Minute {
					return nil
				}
				_, err := c.Do("PING")
				return err
			},
		},
		pub: make(chan *RedisData),
	}
}

func (r *RedisHandler) run() {
	go r.listenPublish()
	go r.listenNewGroups()
	go r.listenDataForGroup()
}

// publish new data by channel name
func (r *RedisHandler) listenPublish() {
	conn := redisHandler.Get()
	defer conn.Close()

	log.Println("Start Listener of new clients")
	for {
		select {
		case channel := <-r.pub:
			err := conn.Send("PUBLISH", channel.name, channel.data)
			if err != nil {
				log.Fatal(err)
			}
			err = conn.Flush()
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

// will receive groups for client
func (r *RedisHandler) listenNewGroups() {
	conn := redisHandler.Get()
	defer conn.Close()

	log.Println("Start Listener of groups by clientKey")
	psc := redis.PubSubConn{Conn: conn}
	psc.PSubscribe(config.Redis.NewGroups + "*")

	for {
		switch msg := psc.Receive().(type) {
		case redis.Message:
			clientKey := strings.TrimLeft(msg.Channel, config.Redis.NewGroups)
			hub.connectGroup <- &RedisData{name: clientKey, data: msg.Data}
		case error:
			log.Printf("Error %s", msg)
			return
		}
	}
}

// listen for new data for group
func (r *RedisHandler) listenDataForGroup() {
	conn := redisHandler.Get()
	defer conn.Close()

	log.Println("Start Listener of data for groups")
	psc := redis.PubSubConn{Conn: conn}
	psc.PSubscribe(config.Redis.DataToGroup + "*")

	for {
		switch msg := psc.Receive().(type) {
		case redis.Message:
			channelName := strings.TrimPrefix(msg.Channel, config.Redis.DataToGroup)
			hub.sendGroup <- &RedisData{name: channelName, data: msg.Data}
		case error:
			log.Printf("Error %s", msg)
			return
		}
	}
}
