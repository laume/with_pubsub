package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/satori/go.uuid"
)

var (
	PUBLISH     = "publish"
	SUBSCRIBE   = "subscribe"
	UNSUBSCRIBE = "unsubscribe"
)

//from pubsub file
type PubSub struct {
	Clients       []Client
	Subscriptions []Subscription
}

type Client struct {
	ID         string
	Connection *websocket.Conn
}

type Message struct {
	Action  string          `json: "action"`
	Topic   string          `json: "topic"`
	Message json.RawMessage `json: "message"`
}

type Subscription struct {
	Topic  string
	Client *Client
}

func (ps *PubSub) AddClient(client Client) *PubSub {

	ps.Clients = append(ps.Clients, client)

	//fmt.Println("Adding new client to the list", client.ID, len(ps.Clients))
	payload := []byte("Hello Client ID: " + client.ID)
	client.Connection.WriteMessage(1, payload)
	return ps
}

func (ps *PubSub) RemoveClient(client Client) *PubSub {

	// first remove all subscriptions by this client
	for index, sub := range ps.Subscriptions {

		if client.ID == sub.Client.ID {
			ps.Subscriptions = append(ps.Subscriptions[:index], ps.Subscriptions[index+1:]...)
		}
	}

	// remove client from the list
	for index, c := range ps.Clients {
		if c.ID == client.ID {
			ps.Clients = append(ps.Clients[:index], ps.Clients[index+1:]...)
		}
	}

	return ps
}

func (ps *PubSub) GetSubscriptions(topic string, client *Client) []Subscription {

	var subscriptionList []Subscription
	for _, subscription := range ps.Subscriptions {
		if client != nil {
			if subscription.Client.ID == client.ID && subscription.Topic == topic {
				subscriptionList = append(subscriptionList, subscription)
			}
		} else {
			if subscription.Topic == topic {
				subscriptionList = append(subscriptionList, subscription)
			}
		}
	}
	return subscriptionList
}

func (ps *PubSub) Subscribe(client *Client, topic string) *PubSub {

	clientSubs := ps.GetSubscriptions(topic, client)
	if len(clientSubs) > 0 {
		// client is subscribed this topic before
		return ps
	}

	newSubscription := Subscription{
		Topic:  topic,
		Client: client,
	}

	ps.Subscriptions = append(ps.Subscriptions, newSubscription)
	return ps
}

func (ps *PubSub) Publish(topic string, message []byte, excludeClient *Client) {

	subscriptions := ps.GetSubscriptions(topic, nil)

	for _, sub := range subscriptions {

		fmt.Printf("sending to client id %s message: %s\n", sub.Client.ID, message)
		//sub.Client.Connection.WriteMessage(1, message)
		sub.Client.Send(message)
	}
}

func (client *Client) Send(message []byte) error {

	return client.Connection.WriteMessage(1, message)

}

func (ps *PubSub) Unsubscribe(client *Client, topic string) *PubSub {
	// clientSubscriptions := ps.GetSubscriptions(topic, client)

	for index, sub := range ps.Subscriptions {
		if sub.Client.ID == client.ID && sub.Topic == topic {
			// found this subscription from client and want to remove it
			ps.Subscriptions = append(ps.Subscriptions[:index], ps.Subscriptions[index+1:]...)
		}
	}
	return ps
}

func (ps *PubSub) handleReceiveMessage(client Client, messageType int, payload []byte) *PubSub {

	m := Message{}
	err := json.Unmarshal(payload, &m)
	if err != nil {
		fmt.Println("this is not correct message payload")
		return ps
	}
	//fmt.Println("Correclt client message payload: ", m.Action, m.Message, m.Topic)

	switch m.Action {

	case PUBLISH:
		fmt.Println("This is publish new message")

		ps.Publish(m.Topic, m.Message, nil)

		break

	case SUBSCRIBE:
		//fmt.Println("This is subscribe new message")
		// by & we do reference to client
		ps.Subscribe(&client, m.Topic)
		fmt.Println("new subscriber to topic", m.Topic, len(ps.Subscriptions), client.ID)
		break

	case UNSUBSCRIBE:
		fmt.Println("Client want to unsubscribe the topic", m.Topic, client.ID)
		ps.Unsubscribe(&client, m.Topic)
		break

	default:
		break
	}
	return ps
}

// from pubsub end

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func autoId() string {
	return uuid.Must(uuid.NewV4()).String()
}

var ps = &PubSub{}

func websocketHandler(w http.ResponseWriter, r *http.Request) {

	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	client := &Client{
		ID:         autoId(),
		Connection: conn,
	}

	// add this client into the list
	ps.AddClient(*client)

	fmt.Println("New client connected, total count: ", len(ps.Clients))

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println("Something went wrong", err)

			// this client disconnect or error connection
			// we do need remove subscriptions and remove client from pubsub
			ps.RemoveClient(*client)

			fmt.Println("total count clients and subscriptions", len(ps.Clients), len(ps.Subscriptions))

			return
		}

		// //if we don't want to rewrite the same message in server
		// aMessage := []byte("Hi client, I am server")
		// // if err := conn.WriteMessage(messageType, p); err != nil {
		// if err := conn.WriteMessage(messageType, aMessage); err != nil {
		// 	log.Println(err)
		// 	return
		// }
		// fmt.Printf("New mwssage from client: %s\n", p)

		// first parameter shows msg from
		ps.handleReceiveMessage(*client, messageType, p)
	}
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		http.ServeFile(w, r, "static")
		// payload := map[string]interface{}{
		// 	"message": "Hello Go",
		// }
		// w.Header().Set("Content-Type", "application/json")
		// json.NewEncoder(w).Encode(payload)
	})

	http.HandleFunc("/ws", websocketHandler)

	http.ListenAndServe(":3000", nil)
	fmt.Println("Server is running...")
}
