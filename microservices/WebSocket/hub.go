package main

import (
	"encoding/json"
	"log"
)

type Hub struct {
	Rooms      map[string]*Room // Map chatID (roomID) to clients
	broadcast  chan *Message
	register   chan *Client
	unregister chan *Client
}

type Room struct {
	ID      string             `json:"id"`
	Clients map[string]*Client `json:"clients"`
	Request bool               `json:"request"`
}

// NewWebsocketServer creates a new WsServer type
func NewHub() *Hub {
	return &Hub{
		Rooms:      make(map[string]*Room),
		broadcast:  make(chan *Message, 100),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run our websocket server, accepting various requests
func (hub *Hub) Run() {
	for {
		select {
		case client := <-hub.register:
			log.Printf("About to register client %s", client.ID)
			hub.registerClient(client)
			log.Printf("Client %s registered to room %s", client.ID, client.RoomID)

		case client := <-hub.unregister:

			log.Printf("About to unregister client %s", client.ID)
			hub.unregisterClient(client)

			log.Printf("Client %s unregistered from room %s", client.ID, client.RoomID)

		case message := <-hub.broadcast:

			log.Printf("Broadcasting message in room %s", message.RoomID)
			if _, ok := hub.Rooms[message.RoomID]; ok {

				for _, cl := range hub.Rooms[message.RoomID].Clients {
					cl.Message <- message
				}
			}

		}
	}
}

func (h *Hub) registerClient(cl *Client) {
	if _, ok := h.Rooms[cl.RoomID]; ok {
		r := h.Rooms[cl.RoomID]

		if _, ok := r.Clients[cl.ID]; !ok {
			r.Clients[cl.ID] = cl
		}
	}
}

func (h *Hub) unregisterClient(cl *Client) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in unregisterClient: %v", r)
		}
	}()
	if _, ok := h.Rooms[cl.RoomID]; ok {
		if _, ok := h.Rooms[cl.RoomID].Clients[cl.ID]; ok {
			if len(h.Rooms[cl.RoomID].Clients) != 0 {
				log.Printf("Broadcasting 'user left' message for client %s", cl.ID)
				//h.broadcast <- &Message{
				//	Text:   "user left the chat",
				//	RoomID: cl.RoomID,
				//}
			}

			log.Printf("Removing client %s from room %s", cl.ID, cl.RoomID)
			delete(h.Rooms[cl.RoomID].Clients, cl.ID)
			close(cl.Message)
		}
	}
}

// /no esta en uso, esta el codigo en el   case message := <-hub.broadcast:
//func (h *Hub) broadcastMessageToRoom(m *Message) {
//	if _, ok := h.Rooms[m.RoomID]; ok {
//
//		for _, cl := range h.Rooms[m.RoomID].Clients {
//			cl.Message <- m
//		}
//	}
//}

func (m *Message) ToJSON() []byte {
	data, _ := json.Marshal(m)
	return data
}
