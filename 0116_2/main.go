package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/satori/go.uuid"
)

var (
	POST  = "post"
	SUB   = "sub"
	UNSUB = "unsub"
)

type PubSub struct {
	Clients       []Client
	Subscriptions []Subscription
}

type Client struct {
	ID         string
	Name       string
	Connection *websocket.Conn
}

type Message struct {
	Action  string          `json: "action"`
	Topic   string          `json: "topic"`
	Message json.RawMessage `json:"message"`
}

type Subscription struct {
	Topic  string
	Client *Client
}

func (ps *PubSub) AddClient(client Client) *PubSub {

	ps.Clients = append(ps.Clients, client)
	// payload := []byte("Hello Client ID" + client.ID)
	// client.Connection.WriteMessage(1, payload)
	return ps
}

func (ps *PubSub) RemoveClient(client Client) *PubSub {

	// remove subscriptions first:
	for index, sub := range ps.Subscriptions {
		if client.ID == sub.Client.ID {
			ps.Subscriptions = append(ps.Subscriptions[:index], ps.Subscriptions[index+1:]...)
		}
	}

	// remove Client from Clients list
	for index, c := range ps.Clients {
		if client.ID == c.ID {
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

// var timeout = time.Duration(30 * time.Second)

func (ps *PubSub) TimeoutUnsubscribe(client *Client, topic string) *PubSub {

	for index, sub := range ps.Subscriptions {
		if sub.Client.ID == client.ID && sub.Topic == topic {
			ps.Subscriptions = append(ps.Subscriptions[:index], ps.Subscriptions[index+1:]...)
			fmt.Println("Client disconnected: timeout: 30s", client.ID)
			fmt.Println("Total count of subscribers: ", len(ps.Subscriptions))
			payload := []byte("Timeout disconnection 30s for channel: " + topic)
			client.Connection.WriteMessage(1, payload)
		}
	}
	return ps
}

func (ps *PubSub) Subscribe(client *Client, topic string) *PubSub {

	clientSubscription := ps.GetSubscriptions(topic, client)
	if len(clientSubscription) > 0 {
		return ps
	}

	newSubscription := Subscription{
		Topic:  topic,
		Client: client,
	}

	ps.Subscriptions = append(ps.Subscriptions, newSubscription)
	return ps
}

func (ps *PubSub) Managesubscriptions(client *Client, topic string) *PubSub {

	ps.Subscribe(client, topic)

	time.AfterFunc(30*time.Second, func() { ps.TimeoutUnsubscribe(client, topic) })

	return ps
}

func (ps *PubSub) Publish(topic string, message []byte, excludeClient *Client) {

	subscriptions := ps.GetSubscriptions(topic, nil)

	for _, sub := range subscriptions {
		fmt.Printf("Sending to Client %s message: %s\n", sub.Client.ID, message)
		sub.Client.Send(message)
	}
}

func (client *Client) Send(message []byte) error {
	return client.Connection.WriteMessage(1, message)
}

func (ps *PubSub) Unsubscribe(client *Client, topic string) *PubSub {

	for index, sub := range ps.Subscriptions {
		if sub.Client.ID == client.ID && sub.Topic == topic {
			ps.Subscriptions = append(ps.Subscriptions[:index], ps.Subscriptions[index+1:]...)
		}
	}
	return ps
}

func (ps *PubSub) handleReceiveMessage(client Client, messageType int, payload []byte) *PubSub {

	msg := Message{}
	err := json.Unmarshal(payload, &msg)
	if err != nil {
		fmt.Println("This is not correct message payload")
		return ps
	}

	switch msg.Action {

	case POST:
		fmt.Println("New message posted")
		ps.Publish(msg.Topic, msg.Message, nil)
		break

	case SUB:
		ps.Managesubscriptions(&client, msg.Topic)
		fmt.Println("New subscriber to topic: ", msg.Topic)
		fmt.Println("\nTotal count of subscriptions: ", len(ps.Subscriptions))
		break

	case UNSUB:
		ps.Unsubscribe(&client, msg.Topic)
		fmt.Println("Client unsubscribed topic: ", msg.Topic)
		fmt.Println("\nTotal count of subscribers: ", len(ps.Subscriptions))
		break

	default:
		break
	}

	return ps
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func AutoId() string {
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
		ID:         AutoId(),
		Connection: conn,
	}

	// add client into the list
	ps.AddClient(*client)
	fmt.Println("New Client connected, total count: ", len(ps.Clients))

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println("Something went wrong", err)

			ps.RemoveClient(*client)
			fmt.Println("Total count of clients and subscriptions", len(ps.Clients), len(ps.Subscriptions))

			return
		}

		ps.handleReceiveMessage(*client, messageType, p)
	}
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		http.ServeFile(w, r, "static")

	})

	http.HandleFunc("/ws", websocketHandler)
	log.Println("http server started on :3000")
	http.ListenAndServe(":3000", nil)
	fmt.Println("Server is running...")
}
