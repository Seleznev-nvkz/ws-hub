# WS-HUB
## In a nutshell
Microservice for handling Redis pub/sub events and websocket connections.
## Main Concept 
![](https://svgshare.com/i/9R5.svg)
- the client connecting and publish *clientKey* to channel *ws-hub:client-new*
- the listener publish the list of *groups* by *clientKey* to channel *ws-hub:groups-new:<clientKey>*
- some service publish data to Redis to channel *ws-hub:group-data:<group>* and WS-Hub broadcast data to clients from the group
- client' data publish to Redis on channel *ws-hub:client-data:<clientKey>*


## How To
### Connect on client side
Common connection to Web Socket. **session_key** can be changed in config.
```
var conn = new WebSocket("ws://127.0.0.1:8080/ws?session_key=123456");
conn.onmessage = function(event) {
    console.log(event);
};
```
### Running
```
go run .
```
### Or using Docker
Create or copy config file and run docker container:
```
docker run -d -ti -v ${PWD}/config.yaml:/root/config.yaml -p 8080:8080 -p 6379:6379 --name ws-hub seleznev/ws-hub:latest
```
## Examples
### Config
Example of config file (example/config.yaml):
```
address: ":8080"            # address of server
clientKey: "session_key"    # name of key to associate new client
serverUrl: "/ws"            # url for new connections

redis:
  address: "localhost:6379" # endpoint of Redis
  timeout: 120              # in seconds; close connections after remaining idle for this duration
  idleConn: 4               # maximum number of connections allocated by the pool
  channelPrefix: "ws-hub:"  # prefix for all channels names

webSocket:
  writeWait: 10             # in seconds; write deadline on the underlying network connection
  pongWait: 60              # in seconds; read deadline on the underlying network connection
  maxMessageSize: 128       # maximum size for a message read from the peer
```
Put the config file next to the service being started.
### Listener
You can look to the example of the listener on Python in *example/listener.py*

---
## ToDo
* Tests
* Refactoring storing of relations between Groups and Clients
* Get rid of goroutines for every connection - get rid of Gorilla ([more info here](https://godoc.org/github.com/gorilla/websocket#hdr-Concurrency))
