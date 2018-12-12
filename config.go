package main

import (
	"github.com/spf13/viper"
	"log"
	"time"
)

type Config struct {
	Address    string
	ClientKey  string // unique keys to identify user
	ServerUrl  string
	FromHeader bool // to get 'clientKey' from Header or Query

	Redis struct {
		Address       string
		ChannelPrefix string
		Timeout       time.Duration
		IdleConn      int
		Db            int

		NewClient      string // to send new 'clientKey'
		DataToGroup    string // to broadcast on group
		NewGroups      string // to receive msg with new groups
		DataFromClient string // to send data from client to redis
	}
	WebSocket struct {
		WriteWait      time.Duration
		PongWait       time.Duration
		PingPeriod     time.Duration
		MaxMessageSize int64
	}
}

func newConfig() *Config {
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.AddConfigPath("example/")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatal(err)
	}

	conf := &Config{}
	err := viper.Unmarshal(conf)
	if err != nil {
		log.Fatal(err)
	}

	conf.Redis.Timeout = conf.Redis.Timeout * time.Second
	conf.WebSocket.PongWait = conf.WebSocket.PongWait * time.Second
	conf.WebSocket.WriteWait = conf.WebSocket.WriteWait * time.Second
	conf.WebSocket.PingPeriod = (conf.WebSocket.PongWait * 9) / 10

	conf.Redis.DataToGroup = conf.Redis.ChannelPrefix + "group-data:"
	conf.Redis.NewGroups = conf.Redis.ChannelPrefix + "groups-new:"
	conf.Redis.DataFromClient = conf.Redis.ChannelPrefix + "client-data:"
	conf.Redis.NewClient = conf.Redis.ChannelPrefix + "client-new"
	return conf
}
