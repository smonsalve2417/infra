// client.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	common "github.com/Eskiwi-Organization/infra/commons"
	"github.com/Eskiwi-Organization/infra/commons/broker"
	"github.com/gorilla/websocket"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Client represents the websocket client at the server
type Client struct { // Use chatID as the roomID
	conn    *websocket.Conn
	Message chan *Message
	Chatid  primitive.ObjectID `json:"chat_id"`
	ID      string             `json:"id"`
	UserID  primitive.ObjectID `json:"user_id"`
	RoomID  string             `json:"roomId"`
	store   WebStore
	channel *amqp.Channel
}

type Message struct {
	Text      string             `json:"text"`
	RoomID    string             `json:"roomId"`
	SenderID  primitive.ObjectID `json:"clientId"`
	Gems      int                `json:"gems"`
	CreatedAt time.Time          `json:"createdAt"  bson:"createdAt"`
	Read      bool               `json:"read" bson:"read"`
	Contents  []string           `json:"contents"`
	Error     bool               `json:"error"`
}

func (c *Client) writeMessage() {
	defer func() {
		c.conn.Close()
	}()

	for {

		message, ok := <-c.Message
		if !ok {
			return
		}

		c.conn.WriteJSON(message)
	}
}

func (c *Client) sendErrorMessage(errorMessage string) {
	errPayload := MessagePayload{
		Text:     errorMessage,
		Gems:     0, // or any default value
		Contents: nil,
		Error:    true,
	}

	// Marshal the error message as JSON
	errorMsg, err := json.Marshal(errPayload)
	if err != nil {
		log.Printf("error marshaling error message: %v", err)
		return
	}

	// Send the error message over WebSocket
	err = c.conn.WriteMessage(websocket.TextMessage, errorMsg)
	if err != nil {
		log.Printf("error sending error message over WebSocket: %v", err)
	}
}

func (c *Client) readMessage(hub *Hub) {
	defer func() {
		hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, m, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		var payload MessagePayload

		// Unmarshal the JSON message into the struct.
		if err := json.Unmarshal(m, &payload); err != nil {
			log.Printf("error unmarshaling message: %v", err)
			continue
		}

		isRequestTrue, chatUserID, chatCreatorID, err := c.store.IsChatRequestTrueAndUserID(c.Chatid)
		if err != nil {
			log.Printf("error checking chat request status: %v", err)
			continue
		}
		log.Printf("request is: %v", isRequestTrue)

		msg := &Message{
			Text:      payload.Text,
			RoomID:    c.RoomID,
			SenderID:  c.UserID,
			Gems:      payload.Gems,
			CreatedAt: time.Now(),
			Contents:  payload.Contents,
		}

		mongoMessage := MongoMessage{
			Text:      payload.Text,
			Chatid:    c.Chatid,
			Userid:    c.UserID,
			Gems:      payload.Gems,
			CreatedAt: time.Now(),
			Read:      false,
			Contents:  payload.Contents,
		}

		rabbitPayload := common.GemsOnChatPayload{
			ChatID:    c.Chatid,
			UserID:    c.UserID,
			CreatorID: chatCreatorID,
			Gems:      payload.Gems,
		}

		log.Print("se crearon los mensajes")

		if isRequestTrue {
			// Check if the sender is the user_id
			if c.UserID == chatUserID {
				log.Print("eres el user del chat")
				// Query MongoDB to see how many messages this user has sent in this chat
				messageCount, err := c.store.CountUserMessagesInChat(c.Chatid, c.UserID)
				if err != nil {
					log.Printf("error counting user messages: %v", err)

					errorMessage := "Error while processing your request."
					c.sendErrorMessage(errorMessage)
					continue
				}

				// If the user has already sent a message, prevent them from sending another one
				if messageCount >= 1 {
					log.Print("hay mas de un mensaje tuyo")
					log.Printf("User %v is not allowed to send more than one free message in this chat", c.UserID)
					errorMessage := "You are not allowed to send more than one free message in this chat."
					c.sendErrorMessage(errorMessage)
					continue
				}
				log.Print("no toca pagar")
				err = c.handleMessageCreationAndBroadcast(hub, mongoMessage, msg, payload)
				if err != nil {
					log.Printf("error during message creation: %v", err)
					errorMessage := fmt.Sprintf("Error during message creation: %v", err)
					c.sendErrorMessage(errorMessage)
					continue
				}
				continue
			}
		}

		if payload.Gems == 0 {
			if c.UserID == chatUserID {
				log.Print("eres el user del chat y no hay gemas")
				log.Print("cant sent a message with no gems")
				errorMessage := "Cant sent a message with no gems."
				c.sendErrorMessage(errorMessage)
				continue
			}
		}

		if c.UserID == chatUserID {
			log.Print("toca pagar")
			log.Print("eres el user del chat")
			err = c.processMessageWithQueue(context.Background(), hub, payload, mongoMessage, msg, rabbitPayload)
			if err != nil {
				log.Printf("Error in message processing: %v", err)
				errorMessage := fmt.Sprintf("Error in message processing: %v", err)
				c.sendErrorMessage(errorMessage)
				continue
			}
		} else {
			log.Print("no toca pagar")
			log.Print("eres el creador del chat")
			err := c.handleMessageCreationAndBroadcast(hub, mongoMessage, msg, payload)
			if err != nil {
				log.Printf("error during message creation: %v", err)
				errorMessage := fmt.Sprintf("Error during message creation: %v", err)
				c.sendErrorMessage(errorMessage)
				continue
			}
		}

	}
}

// ////////////////////////////YA NO FALTA MUCHO AQUI PERO PRIMERO TOCA CREAR EL PAYLOAD ANTES DE LLAMAR LA FUNCION Y DESPUES RECIBIRLA EN EL AMQP Y COPIAR EL COMENT DONATION QUE ES MISMA LOGICA
func (c *Client) processMessageWithQueue(ctx context.Context, hub *Hub, payload MessagePayload, mongoMessage MongoMessage, msg *Message, rabbiyPayload common.GemsOnChatPayload) error {
	// Marshal the payload
	marshalledRequest, err := json.Marshal(rabbiyPayload)
	if err != nil {
		return fmt.Errorf("internal server error: %v", err)
	}

	// Declare the RabbitMQ queue for chat creation
	q, err := c.channel.QueueDeclare(broker.SendMessageGemsCreatedEvent, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("internal server error: %v", err)
	}

	// Declare the callback queue for receiving confirmation
	callbackQueue, err := c.channel.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		return fmt.Errorf("failed to declare a callback queue: %v", err)
	}

	// Start consuming the messages from the callback queue
	msgs, err := c.channel.Consume(callbackQueue.Name, "", true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("failed to register a consumer: %v", err)
	}

	// Generate correlation ID
	corrId := generateCorrelationId()

	// Publish the message to the queue
	err = c.channel.PublishWithContext(ctx, "", q.Name, false, false, amqp.Publishing{
		ContentType:   "application/json",
		Body:          marshalledRequest,
		DeliveryMode:  amqp.Persistent,
		CorrelationId: corrId,
		ReplyTo:       callbackQueue.Name,
	})
	if err != nil {
		return fmt.Errorf("failed to publish a chat request: %v", err)
	}

	// Set a timeout for the confirmation
	timeout := time.After(30 * time.Second)

	// Listen for the confirmation
	for {
		select {
		case d := <-msgs:
			if d.CorrelationId == corrId {
				var response struct {
					Success bool `json:"success"`
				}
				if err := json.Unmarshal(d.Body, &response); err != nil {
					return fmt.Errorf("failed to parse confirmation response: %v", err)
				}
				if response.Success {
					// Proceed with creating and broadcasting the message
					err := c.handleMessageCreationAndBroadcast(hub, mongoMessage, msg, payload)
					if err != nil {
						return fmt.Errorf("error during message creation: %v", err)
					}
					return nil
				}
				return fmt.Errorf("chat creation failed")
			}
		case <-timeout:
			return fmt.Errorf("payment confirmation timeout")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *Client) handleMessageCreationAndBroadcast(hub *Hub, mongoMessage MongoMessage, msg *Message, payload MessagePayload) error {
	// Create the message in MongoDB
	messageID, err := c.store.CreateMessage(mongoMessage)
	if err != nil {
		return fmt.Errorf("error saving message: %v", err)
	}

	// Update the message status in MongoDB
	err = c.store.UpdateMessageStatus(c.Chatid, payload.Text)
	if err != nil {
		return fmt.Errorf("error updating chat: %v", err)
	}

	// Broadcast the message to the chat room
	hub.broadcast <- msg

	// Update read status or send a notification based on the number of clients in the room
	if len(hub.Rooms[c.RoomID].Clients) == 2 {
		err = c.store.UpdateMessageReadStatus(messageID)
		if err != nil {
			return fmt.Errorf("error updating message read status: %v", err)
		}
	} else {
		err = c.store.SendMessageNotification(c.UserID, c.Chatid, payload.Text)
		if err != nil {
			return fmt.Errorf("error sending notification: %v", err)
		}
	}

	return nil
}
