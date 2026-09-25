package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	common "github.com/Eskiwi-Organization/infra/commons"
	pb "github.com/Eskiwi-Organization/infra/commons/api"
	"github.com/Eskiwi-Organization/infra/commons/broker"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc"
)

type grpcHandler struct {
	pb.UnimplementedWebServiceServer

	hub     *Hub
	service WebService
	store   WebStore
	channel *amqp.Channel
}

func NewGRPCHandler(grpcServer *grpc.Server, service WebService, store WebStore, hub *Hub, channel *amqp.Channel) {
	handler := &grpcHandler{
		service: service,
		store:   store,
		hub:     hub,
		channel: channel,
	}

	pb.RegisterWebServiceServer(grpcServer, handler)

}

func (h *grpcHandler) CreateChat(ctx context.Context, p *pb.CreateChatRequest) (*pb.CreateChatResponse, error) {

	userObjID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		o := &pb.CreateChatResponse{}
		return o, err
	}

	creatorObjID, err := primitive.ObjectIDFromHex(p.CreatorID)
	if err != nil {
		o := &pb.CreateChatResponse{}
		return o, err
	}

	chat := Chat{
		UserID:    userObjID,
		CreatorID: creatorObjID,
	}

	exists, err := h.store.ChatExists(chat)
	if err != nil {
		log.Printf("Error checking if chat exists: %v", err)
		o := &pb.CreateChatResponse{}
		return o, err
	} else if exists {
		log.Printf("Chat already exists")
		err = fmt.Errorf("chat already exists")
		o := &pb.CreateChatResponse{}
		return o, err
	}

	settings, err := h.store.GetChatPriceAndRules(creatorObjID)
	if err != nil {
		log.Printf("Error fetching chat settings: %v", err)
		o := &pb.CreateChatResponse{}
		return o, err
	}

	chatID := primitive.NewObjectID()

	var payload1 common.CreateChatDonationPayload

	payload1.Creator_id = creatorObjID
	payload1.User_id = userObjID
	payload1.Gems = settings.Price
	payload1.ChatID = chatID

	marshalledRequest, err := json.Marshal(payload1)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		return nil, err
	}
	//queue to send to payments to process the creation of a chat
	q, err := h.channel.QueueDeclare(broker.SendCreateChatCreatedEvent, true, false, false, false, nil)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		return nil, err
	}

	// Declare the callback queue for receiving chat confirmation
	callbackQueue, err := h.channel.QueueDeclare(
		"",
		false,
		true,
		true,
		false,
		nil,
	)

	if err != nil {
		log.Printf("Failed to declare a callback queue: %v", err)
		return nil, err
	}

	msgs, err := h.channel.Consume(
		callbackQueue.Name, // queue
		"",                 // consumer
		true,               // auto-ack
		false,              // exclusive
		false,              // no-local
		false,              // no-wait
		nil,                // args
	)
	if err != nil {
		log.Printf("Failed to register a consumer: %v", err)
		return nil, err
	}

	corrId := generateCorrelationId()

	err = h.channel.PublishWithContext(ctx, "", q.Name, false, false, amqp.Publishing{
		ContentType:   "application/json",
		Body:          marshalledRequest,
		DeliveryMode:  amqp.Persistent,
		CorrelationId: corrId,
		ReplyTo:       callbackQueue.Name,
	})
	if err != nil {
		log.Printf("Failed to publish a payment request: %v", err)
		return nil, err
	}

	// Timeout for waiting on payment confirmation
	timeout := time.After(30 * time.Second) // adjust timeout to suit how long payments should reasonably take

	// Listen for the confirmation
	for {
		select {
		case d := <-msgs:
			if d.CorrelationId == corrId {
				var paymentResponse struct {
					Success bool `json:"success"`
				}
				if err := json.Unmarshal(d.Body, &paymentResponse); err != nil {
					log.Printf("Failed to parse payment response: %v", err)
					return nil, err
				}
				if paymentResponse.Success {
					// Proceed to create the chat
					return h.CreateChatConsumer(ctx, p, chatID)
				}
				return nil, fmt.Errorf("payment failed")
			}
		case <-timeout:
			return nil, fmt.Errorf("payment confirmation timeout")
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (h *grpcHandler) CreateChatConsumer(ctx context.Context, p *pb.CreateChatRequest, chatID primitive.ObjectID) (*pb.CreateChatResponse, error) {

	userObjID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		o := &pb.CreateChatResponse{}
		return o, err
	}

	creatorObjID, err := primitive.ObjectIDFromHex(p.CreatorID)
	if err != nil {
		o := &pb.CreateChatResponse{}
		return o, err
	}

	err, chatExists := h.store.CreateChat(Chat{
		ID:        chatID,
		CreatorID: creatorObjID,
		UserID:    userObjID,
		CreatedAt: time.Now(),
		Request:   true,
	})

	if err != nil {
		o := &pb.CreateChatResponse{}
		return o, err
	}

	if chatExists {
		err = fmt.Errorf("status:chat already exists")
		o := &pb.CreateChatResponse{}
		return o, err
	}

	chatID, err = h.store.GetChatID(userObjID, creatorObjID)
	if err != nil {
		o := &pb.CreateChatResponse{}
		return o, err
	}

	list, err := h.store.GetChat(chatID, userObjID)
	if err != nil {
		log.Printf("Error searching chat: %v", err)
		o := &pb.CreateChatResponse{}
		return o, err
	}

	var response []*pb.ChatWithUser

	for _, chatwithuser := range list {
		grpcChat := h.store.ConvertToGrpcChat(&chatwithuser.Chat)
		grpcUser := h.store.ConvertToGrpcUser(&chatwithuser.User)
		o := &pb.ChatWithUser{
			Chat:   grpcChat,
			User:   grpcUser,
			Unread: int32(chatwithuser.Unread),
		}
		response = append(response, o)
	}

	o := &pb.CreateChatResponse{
		Chats: response,
	}
	return o, nil
}

func (h *grpcHandler) CreateRoom(ctx context.Context, p *pb.CreateRoomRequest) (*pb.CreateRoomResponse, error) {

	if h.hub.Rooms[p.Id] != nil {
		o := &pb.CreateRoomResponse{
			Created: true,
		}
		return o, nil
	}

	h.hub.Rooms[p.Id] = &Room{
		ID:      p.Id,
		Clients: make(map[string]*Client),
		Request: p.Request,
	}

	o := &pb.CreateRoomResponse{
		Created: false,
	}
	return o, nil

}

func (h *grpcHandler) GetLatestsChats(ctx context.Context, p *pb.GetLatestsChatsRequest) (*pb.GetLatestsChatsResponse, error) {

	userObjID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		o := &pb.GetLatestsChatsResponse{}
		return o, err
	}

	list, err := h.store.GetLatestsChats(int(p.Page), 10, userObjID, false)
	if err != nil {
		o := &pb.GetLatestsChatsResponse{}
		log.Printf("Error searching chats: %v", err)
		return o, err
	}

	var response []*pb.ChatWithUser

	for _, chatwithuser := range list {
		grpcChat := h.store.ConvertToGrpcChat(&chatwithuser.Chat)
		grpcUser := h.store.ConvertToGrpcUser(&chatwithuser.User)
		o := &pb.ChatWithUser{
			Chat:   grpcChat,
			User:   grpcUser,
			Unread: int32(chatwithuser.Unread),
		}
		response = append(response, o)
	}

	o := &pb.GetLatestsChatsResponse{
		Chats: response,
	}
	return o, nil
}

func (h *grpcHandler) GetLatestsRequestChats(ctx context.Context, p *pb.GetLatestsChatsRequest) (*pb.GetLatestsChatsResponse, error) {

	userObjID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		o := &pb.GetLatestsChatsResponse{}
		return o, err
	}

	list, err := h.store.GetLatestsChatsRequest(int(p.Page), 10, userObjID)
	if err != nil {
		o := &pb.GetLatestsChatsResponse{}
		log.Printf("Error searching chats: %v", err)
		return o, err
	}

	var response []*pb.ChatWithUser

	for _, chatwithuser := range list {
		grpcChat := h.store.ConvertToGrpcChat(&chatwithuser.Chat)
		grpcUser := h.store.ConvertToGrpcUser(&chatwithuser.User)
		o := &pb.ChatWithUser{
			Chat:   grpcChat,
			User:   grpcUser,
			Unread: int32(chatwithuser.Unread),
		}
		response = append(response, o)
	}

	o := &pb.GetLatestsChatsResponse{
		Chats: response,
	}
	return o, nil
}

func (h *grpcHandler) GetChat(ctx context.Context, p *pb.GetChatRequest) (*pb.GetChatResponse, error) {

	userObjID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		o := &pb.GetChatResponse{}
		return o, err
	}
	chatObjID, err := primitive.ObjectIDFromHex(p.ChatID)
	if err != nil {
		o := &pb.GetChatResponse{}
		return o, err
	}

	list, err := h.store.GetChat(chatObjID, userObjID)
	if err != nil {
		log.Printf("Error searching chat: %v", err)
		o := &pb.GetChatResponse{}
		return o, err
	}

	var response []*pb.ChatWithUser

	for _, chatwithuser := range list {
		grpcChat := h.store.ConvertToGrpcChat(&chatwithuser.Chat)
		grpcUser := h.store.ConvertToGrpcUser(&chatwithuser.User)
		o := &pb.ChatWithUser{
			Chat:   grpcChat,
			User:   grpcUser,
			Unread: int32(chatwithuser.Unread),
		}
		response = append(response, o)
	}

	o := &pb.GetChatResponse{
		Chats: response,
	}
	return o, nil
}

func (h *grpcHandler) GetLatestsMessages(ctx context.Context, p *pb.GetLatestsMessagesRequest) (*pb.GetLatestsMessagesResponse, error) {

	chatObjID, err := primitive.ObjectIDFromHex(p.ChatID)
	if err != nil {
		o := &pb.GetLatestsMessagesResponse{}
		return o, err
	}

	list, err := h.store.UpdateAndRetrievePaginatedMessages(chatObjID, int(p.Page), 10)
	if err != nil {
		o := &pb.GetLatestsMessagesResponse{}
		return o, err
	}

	var response []*pb.Message

	for _, message := range list {
		grpcMessage := h.store.ConvertToGrpcMessage(&message)

		response = append(response, grpcMessage)
	}

	o := &pb.GetLatestsMessagesResponse{
		Messages: response,
	}
	return o, nil
}

func (h *grpcHandler) AcceptChatReq(ctx context.Context, p *pb.AcceptChatReqRequest) (*pb.AcceptChatReqResponse, error) {

	chatObjID, err := primitive.ObjectIDFromHex(p.ChatID)
	if err != nil {
		o := &pb.AcceptChatReqResponse{}
		return o, err
	}

	var payload1 common.AcceptChatPayload

	payload1.ChatID = chatObjID

	marshalledRequest, err := json.Marshal(payload1)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		return nil, err
	}

	//queue to send to payments to process the creation of a chat
	q, err := h.channel.QueueDeclare(broker.SendAcceptChatCreatedEvent, true, false, false, false, nil)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		return nil, err
	}

	// Declare the callback queue for receiving chat confirmation
	callbackQueue, err := h.channel.QueueDeclare(
		"",
		false,
		true,
		true,
		false,
		nil,
	)

	if err != nil {
		log.Printf("Failed to declare a callback queue: %v", err)
		return nil, err
	}

	msgs, err := h.channel.Consume(
		callbackQueue.Name, // queue
		"",                 // consumer
		true,               // auto-ack
		false,              // exclusive
		false,              // no-local
		false,              // no-wait
		nil,                // args
	)
	if err != nil {
		log.Printf("Failed to register a consumer: %v", err)
		return nil, err
	}

	corrId := generateCorrelationId()

	err = h.channel.PublishWithContext(ctx, "", q.Name, false, false, amqp.Publishing{
		ContentType:   "application/json",
		Body:          marshalledRequest,
		DeliveryMode:  amqp.Persistent,
		CorrelationId: corrId,
		ReplyTo:       callbackQueue.Name,
	})
	if err != nil {
		log.Printf("Failed to publish a payment request: %v", err)
		return nil, err
	}

	// Timeout for waiting on payment confirmation
	timeout := time.After(30 * time.Second) // adjust timeout to suit how long payments should reasonably take

	// Listen for the confirmation
	for {
		select {
		case d := <-msgs:
			if d.CorrelationId == corrId {
				var paymentResponse struct {
					Success bool `json:"success"`
				}
				if err := json.Unmarshal(d.Body, &paymentResponse); err != nil {
					log.Printf("Failed to parse payment response: %v", err)
					return nil, err
				}
				if paymentResponse.Success {
					// Proceed to create the chat
					return h.AcceptChatReqConsumer(ctx, p)
				}
				return nil, fmt.Errorf("payment failed")
			}
		case <-timeout:
			return nil, fmt.Errorf("payment confirmation timeout")
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (h *grpcHandler) AcceptChatReqConsumer(ctx context.Context, p *pb.AcceptChatReqRequest) (*pb.AcceptChatReqResponse, error) {

	chatObjID, err := primitive.ObjectIDFromHex(p.ChatID)
	if err != nil {
		o := &pb.AcceptChatReqResponse{}
		return o, err
	}

	err = h.store.UpdateChatRequestStatus(chatObjID)
	if err != nil {
		o := &pb.AcceptChatReqResponse{}
		return o, err
	}

	o := &pb.AcceptChatReqResponse{
		Status: "Updated",
	}
	return o, nil
}

func (h *grpcHandler) RejectChatReq(ctx context.Context, p *pb.AcceptChatReqRequest) (*pb.AcceptChatReqResponse, error) {

	chatObjID, err := primitive.ObjectIDFromHex(p.ChatID)
	if err != nil {
		o := &pb.AcceptChatReqResponse{}
		return o, err
	}

	var payload1 common.AcceptChatPayload

	payload1.ChatID = chatObjID

	marshalledRequest, err := json.Marshal(payload1)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		return nil, err
	}

	//queue to send to payments to process the creation of a chat
	q, err := h.channel.QueueDeclare(broker.SendDenyChatCreatedEvent, true, false, false, false, nil)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		return nil, err
	}

	// Declare the callback queue for receiving chat confirmation
	callbackQueue, err := h.channel.QueueDeclare(
		"",
		false,
		true,
		true,
		false,
		nil,
	)

	if err != nil {
		log.Printf("Failed to declare a callback queue: %v", err)
		return nil, err
	}

	msgs, err := h.channel.Consume(
		callbackQueue.Name, // queue
		"",                 // consumer
		true,               // auto-ack
		false,              // exclusive
		false,              // no-local
		false,              // no-wait
		nil,                // args
	)
	if err != nil {
		log.Printf("Failed to register a consumer: %v", err)
		return nil, err
	}

	corrId := generateCorrelationId()

	err = h.channel.PublishWithContext(ctx, "", q.Name, false, false, amqp.Publishing{
		ContentType:   "application/json",
		Body:          marshalledRequest,
		DeliveryMode:  amqp.Persistent,
		CorrelationId: corrId,
		ReplyTo:       callbackQueue.Name,
	})
	if err != nil {
		log.Printf("Failed to publish a payment request: %v", err)
		return nil, err
	}

	// Timeout for waiting on payment confirmation
	timeout := time.After(30 * time.Second) // adjust timeout to suit how long payments should reasonably take

	// Listen for the confirmation
	for {
		select {
		case d := <-msgs:
			if d.CorrelationId == corrId {
				var paymentResponse struct {
					Success bool `json:"success"`
				}
				if err := json.Unmarshal(d.Body, &paymentResponse); err != nil {
					log.Printf("Failed to parse payment response: %v", err)
					return nil, err
				}
				if paymentResponse.Success {
					// Proceed to create the chat
					return h.RejectChatReqConsumer(ctx, p)
				}
				return nil, fmt.Errorf("payment failed")
			}
		case <-timeout:
			return nil, fmt.Errorf("payment confirmation timeout")
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (h *grpcHandler) RejectChatReqConsumer(ctx context.Context, p *pb.AcceptChatReqRequest) (*pb.AcceptChatReqResponse, error) {

	chatObjID, err := primitive.ObjectIDFromHex(p.ChatID)
	if err != nil {
		o := &pb.AcceptChatReqResponse{}
		return o, err
	}

	err = h.store.DeleteChat(chatObjID)
	if err != nil {
		o := &pb.AcceptChatReqResponse{}
		return o, err
	}

	o := &pb.AcceptChatReqResponse{
		Status: "Deleted",
	}
	return o, nil
}

func (h *grpcHandler) AcceptChatInsideReq(ctx context.Context, p *pb.AcceptChatInsideReqRequest) (*pb.AcceptChatInsideReqResponse, error) {

	var payload1 common.AcceptMessageReqPayload

	payload1.TransID = int(p.TransID)

	marshalledRequest, err := json.Marshal(payload1)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		return nil, err
	}

	//queue to send to payments to process the creation of a chat
	q, err := h.channel.QueueDeclare(broker.SendAcceptMessageReqCreatedEvent, true, false, false, false, nil)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		return nil, err
	}

	// Declare the callback queue for receiving chat confirmation
	callbackQueue, err := h.channel.QueueDeclare(
		"",
		false,
		true,
		true,
		false,
		nil,
	)

	if err != nil {
		log.Printf("Failed to declare a callback queue: %v", err)
		return nil, err
	}

	msgs, err := h.channel.Consume(
		callbackQueue.Name, // queue
		"",                 // consumer
		true,               // auto-ack
		false,              // exclusive
		false,              // no-local
		false,              // no-wait
		nil,                // args
	)
	if err != nil {
		log.Printf("Failed to register a consumer: %v", err)
		return nil, err
	}

	corrId := generateCorrelationId()

	err = h.channel.PublishWithContext(ctx, "", q.Name, false, false, amqp.Publishing{
		ContentType:   "application/json",
		Body:          marshalledRequest,
		DeliveryMode:  amqp.Persistent,
		CorrelationId: corrId,
		ReplyTo:       callbackQueue.Name,
	})
	if err != nil {
		log.Printf("Failed to publish a payment request: %v", err)
		return nil, err
	}

	// Timeout for waiting on payment confirmation
	timeout := time.After(30 * time.Second) // adjust timeout to suit how long payments should reasonably take

	// Listen for the confirmation
	for {
		select {
		case d := <-msgs:
			if d.CorrelationId == corrId {
				var paymentResponse struct {
					Success bool `json:"success"`
				}
				if err := json.Unmarshal(d.Body, &paymentResponse); err != nil {
					log.Printf("Failed to parse payment response: %v", err)
					return nil, err
				}
				if paymentResponse.Success {
					// Proceed to create the chat
					return h.AcceptChatInsideReqConsumer(ctx, p)
				}
				return nil, fmt.Errorf("payment failed")
			}
		case <-timeout:
			return nil, fmt.Errorf("payment confirmation timeout")
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (h *grpcHandler) AcceptChatInsideReqConsumer(ctx context.Context, p *pb.AcceptChatInsideReqRequest) (*pb.AcceptChatInsideReqResponse, error) {

	err := h.store.UpdateMessageRequestStatus(int(p.TransID))
	if err != nil {
		o := &pb.AcceptChatInsideReqResponse{}
		return o, err
	}

	o := &pb.AcceptChatInsideReqResponse{
		Status: "Updated",
	}
	return o, nil
}

func (h *grpcHandler) RejectChatInsideReq(ctx context.Context, p *pb.AcceptChatInsideReqRequest) (*pb.AcceptChatInsideReqResponse, error) {

	var payload1 common.AcceptMessageReqPayload

	payload1.TransID = int(p.TransID)

	marshalledRequest, err := json.Marshal(payload1)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		return nil, err
	}

	//queue to send to payments to process the creation of a chat
	q, err := h.channel.QueueDeclare(broker.SendRejectMessageReqCreatedEvent, true, false, false, false, nil)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		return nil, err
	}

	// Declare the callback queue for receiving chat confirmation
	callbackQueue, err := h.channel.QueueDeclare(
		"",
		false,
		true,
		true,
		false,
		nil,
	)

	if err != nil {
		log.Printf("Failed to declare a callback queue: %v", err)
		return nil, err
	}

	msgs, err := h.channel.Consume(
		callbackQueue.Name, // queue
		"",                 // consumer
		true,               // auto-ack
		false,              // exclusive
		false,              // no-local
		false,              // no-wait
		nil,                // args
	)
	if err != nil {
		log.Printf("Failed to register a consumer: %v", err)
		return nil, err
	}

	corrId := generateCorrelationId()

	err = h.channel.PublishWithContext(ctx, "", q.Name, false, false, amqp.Publishing{
		ContentType:   "application/json",
		Body:          marshalledRequest,
		DeliveryMode:  amqp.Persistent,
		CorrelationId: corrId,
		ReplyTo:       callbackQueue.Name,
	})
	if err != nil {
		log.Printf("Failed to publish a payment request: %v", err)
		return nil, err
	}

	// Timeout for waiting on payment confirmation
	timeout := time.After(30 * time.Second) // adjust timeout to suit how long payments should reasonably take

	// Listen for the confirmation
	for {
		select {
		case d := <-msgs:
			if d.CorrelationId == corrId {
				var paymentResponse struct {
					Success bool `json:"success"`
				}
				if err := json.Unmarshal(d.Body, &paymentResponse); err != nil {
					log.Printf("Failed to parse payment response: %v", err)
					return nil, err
				}
				if paymentResponse.Success {
					// Proceed to create the chat
					return h.AcceptChatInsideReqConsumer(ctx, p)
				}
				return nil, fmt.Errorf("payment failed")
			}
		case <-timeout:
			return nil, fmt.Errorf("payment confirmation timeout")
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (h *grpcHandler) RejectChatInsideReqConsumer(ctx context.Context, p *pb.AcceptChatInsideReqRequest) (*pb.AcceptChatInsideReqResponse, error) {

	err := h.store.DeleteMessageByTransactionID(int(p.TransID))
	if err != nil {
		o := &pb.AcceptChatInsideReqResponse{}
		return o, err
	}

	o := &pb.AcceptChatInsideReqResponse{
		Status: "Updated",
	}
	return o, nil
}

func generateCorrelationId() string {
	id, err := uuid.NewRandom()
	if err != nil {
		log.Fatalf("Failed to generate UUID: %v", err)
		return ""
	}
	return id.String()
}
