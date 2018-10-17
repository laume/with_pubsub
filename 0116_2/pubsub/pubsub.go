package pubsub

import (
	"fmt"

	"github.com/gorilla/websocket"
)

type PubSub struct {
	Clients []Client
}

type Client struct {
	ID         string
	Connection *websocket.Conn
}

func (ps *PubSub) AddClient(client Client) *PubSub {

	ps.Clients = append(ps.Clients, client)

	fmt.Println("Adding new client to the list", client.ID)
	return ps
}
