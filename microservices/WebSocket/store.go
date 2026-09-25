package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mime/multipart"
	"time"

	common "github.com/Eskiwi-Organization/infra/commons"
	pb "github.com/Eskiwi-Organization/infra/commons/api"
	"github.com/Eskiwi-Organization/infra/commons/aws_s3"
	"github.com/Eskiwi-Organization/infra/commons/broker"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type store struct {
	client      *mongo.Client
	database    *mongo.Database
	webS3Client *aws_s3.S3Client
	channel     *amqp.Channel
}

func NewStore(client *mongo.Client, webS3Client *aws_s3.S3Client, channel *amqp.Channel) *store {
	return &store{
		client:      client,
		database:    client.Database(mongoDatabaseName),
		webS3Client: webS3Client,
		channel:     channel}
}

func (s *store) CreateChat(chat Chat) (error, bool) {
	collection := s.database.Collection("chats")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if the PostLike already exists
	filter := bson.M{
		"user_id":    chat.UserID,
		"creator_id": chat.CreatorID,
	}
	var existingchat Chat
	err := collection.FindOne(ctx, filter).Decode(&existingchat)
	if err != nil && err != mongo.ErrNoDocuments {
		return err, false
	}

	if existingchat.ID != primitive.NilObjectID {
		return fmt.Errorf("chat already exists"), true
	}

	_, err = collection.InsertOne(ctx, chat)
	if err != nil {
		return err, false
	}
	return nil, false
}

func (s *store) GetChatID(userID, creatorID primitive.ObjectID) (primitive.ObjectID, error) {
	collection := s.database.Collection("chats")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Define the filter to search for a chat with the given userID and creatorID
	filter := bson.M{
		"user_id":    userID,
		"creator_id": creatorID,
	}

	// Use FindOne to get a single document matching the filter
	var chat Chat
	err := collection.FindOne(ctx, filter).Decode(&chat)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return primitive.NilObjectID, nil // No chat found
		}
		return primitive.NilObjectID, err // Error occurred
	}

	return chat.ID, nil // Successfully found and return the chat ID
}

func (s *store) ChatExists(chat Chat) (bool, error) {
	collection := s.database.Collection("chats")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Define the filter to check if the chat already exists
	filter := bson.M{
		"user_id":    chat.UserID,
		"creator_id": chat.CreatorID,
	}

	// Check if the chat exists
	var existingChat Chat
	err := collection.FindOne(ctx, filter).Decode(&existingChat)

	if err != nil {
		// If no document is found, return false, and no error
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
		// Return false with the error if something else went wrong
		return false, err
	}

	// If we found an existing chat, return true
	return true, nil
}

func (s *store) GetLatestsChats(page int, pageSize int, userID primitive.ObjectID, typeRequest bool) ([]ChatWithUser, error) {
	chatCollection := s.database.Collection("chats")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set up the options for sorting, pagination, and limiting
	opts := options.Find()
	opts.SetSort(bson.D{primitive.E{Key: "createdAt", Value: -1}})
	opts.SetSkip(int64((page - 1) * pageSize))
	opts.SetLimit(int64(pageSize))

	// Filter to only include chats where the user_id and creator_id matches

	////////
	//Soy un usuario normal, hago la peticion y quiero que me salgan
	//los posts donde soy el userID, y no importa el request type
	//Donde soy el creador y si quiero que sean request true

	////////
	filter := bson.M{
		"$or": []bson.M{
			{
				"user_id": userID,
				// Aquí no especificamos nada sobre 'request' para ignorarlo completamente
			},
			{
				"$and": []bson.M{
					{"creator_id": userID},
					{"request": false}, // Solo incluye cuando 'request' es false
				},
			},
		},
	}

	cursor, err := chatCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var chats []Chat
	if err = cursor.All(ctx, &chats); err != nil {
		return nil, err
	}

	userCollection := s.database.Collection("users")

	var chatsWithUsers []ChatWithUser

	for _, chat := range chats {
		var user common.User

		var IdToSearch primitive.ObjectID

		if userID == chat.UserID {
			IdToSearch = chat.CreatorID
		} else {
			IdToSearch = chat.UserID
		}

		err := userCollection.FindOne(ctx, bson.M{"_id": IdToSearch}).Decode(&user)
		if err != nil {
			log.Printf("Error finding user: %v", err)
			return nil, err
		}

		unread, err := s.CountUnreadMessages(chat.ID, userID)
		if err != nil {
			log.Printf("Error counting unread messages: %v", err)
			return nil, err
		}

		chatWithUser := ChatWithUser{
			Chat:   chat,
			User:   user,
			Unread: int(unread),
		}
		chatsWithUsers = append(chatsWithUsers, chatWithUser)
	}

	return chatsWithUsers, nil
}

func (s *store) GetLatestsChatsRequest(page int, pageSize int, userID primitive.ObjectID) ([]ChatWithUser, error) {
	chatCollection := s.database.Collection("chats")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set up the options for sorting, pagination, and limiting
	opts := options.Find()
	opts.SetSort(bson.D{primitive.E{Key: "createdAt", Value: -1}})
	opts.SetSkip(int64((page - 1) * pageSize))
	opts.SetLimit(int64(pageSize))

	// Filter to only include chats where the user_id and creator_id matches

	// Soy usuario y quiero ver solamente:
	// en donde yo soy el creator_ID y

	filter := bson.M{
		"$and": []bson.M{ // Aplica condiciones adicionales cuando 'creator_id' es igual a 'userID'
			{"creator_id": userID},
			{"request": true}, // 'request' debe ser true
		},
	}

	cursor, err := chatCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var chats []Chat
	if err = cursor.All(ctx, &chats); err != nil {
		return nil, err
	}

	userCollection := s.database.Collection("users")

	var chatsWithUsers []ChatWithUser

	for _, chat := range chats {
		var user common.User

		var IdToSearch primitive.ObjectID

		if userID == chat.UserID {
			IdToSearch = chat.CreatorID
		} else {
			IdToSearch = chat.UserID
		}

		err := userCollection.FindOne(ctx, bson.M{"_id": IdToSearch}).Decode(&user)
		if err != nil {
			log.Printf("Error finding user: %v", err)
			return nil, err
		}

		unread, err := s.CountUnreadMessages(chat.ID, userID)
		if err != nil {
			log.Printf("Error counting unread messages: %v", err)
			return nil, err
		}

		chatWithUser := ChatWithUser{
			Chat:   chat,
			User:   user,
			Unread: int(unread),
		}
		chatsWithUsers = append(chatsWithUsers, chatWithUser)
	}

	return chatsWithUsers, nil
}

func (s *store) GetChat(chatID primitive.ObjectID, userID primitive.ObjectID) ([]ChatWithUser, error) {
	chatCollection := s.database.Collection("chats")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{
		"_id": chatID,
	}

	cursor, err := chatCollection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var chat Chat
	err = chatCollection.FindOne(ctx, filter).Decode(&chat)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // No chat found
		}
		return nil, err
	}

	userCollection := s.database.Collection("users")

	var chatsWithUsers []ChatWithUser

	var user common.User
	// if the user
	IdToSearch := chat.CreatorID
	if userID == chat.CreatorID {
		IdToSearch = userID
	}
	err = userCollection.FindOne(ctx, bson.M{"_id": IdToSearch}).Decode(&user)
	if err != nil {
		return nil, err
	}
	unread, err := s.CountUnreadMessages(chat.ID, userID)
	if err != nil {
		return nil, err
	}
	chatWithUser := ChatWithUser{
		Chat:   chat,
		User:   user,
		Unread: int(unread),
	}
	chatsWithUsers = append(chatsWithUsers, chatWithUser)

	return chatsWithUsers, nil
}

func (s *store) CreateMessage(message MongoMessage) (primitive.ObjectID, error) {
	collection := s.database.Collection("messages")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := collection.InsertOne(ctx, message)
	if err != nil {
		return primitive.NilObjectID, err // Return nil ObjectID on error
	}

	// Extract the inserted ID, assuming it's an ObjectID
	insertedID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return primitive.NilObjectID, fmt.Errorf("inserted document ID is not an ObjectID")
	}

	return insertedID, nil
}

func (s *store) UpdateMessageStatus(chatID primitive.ObjectID, lastMessage string) error {
	collection := s.database.Collection("chats")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Define the filter to locate the document to update
	filter := bson.M{"_id": chatID}

	// Define the update operation
	update := bson.M{
		"$set": bson.M{
			"last_message": lastMessage,
			"lastSentAt":   time.Now(),
		},
	}
	log.Print(time.Now())

	// Perform the update operation
	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) CountUnreadMessages(chatID primitive.ObjectID, userID primitive.ObjectID) (int64, error) {
	collection := s.database.Collection("messages")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{
		"chat_id": chatID, // Adjust this field name based on your schema if necessary
		"user_id": bson.M{"$ne": userID},
		"read":    false,
	}

	// Count the documents that match the filter
	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (s *store) UpdateAndRetrievePaginatedMessages(chatID primitive.ObjectID, page int, pageSize int) ([]Message, error) {
	messageCollection := s.database.Collection("messages")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Define the filter to find messages for the given chatID
	filter := bson.M{"chat_id": chatID}

	// Define options for sorting, pagination, and limiting the number of messages
	opts := options.Find()
	opts.SetSort(bson.D{primitive.E{Key: "createdAt", Value: -1}})
	opts.SetSkip(int64((page - 1) * pageSize))
	opts.SetLimit(int64(pageSize))

	// Retrieve the messages with pagination
	cursor, err := messageCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var messages []MongoMessage
	if err = cursor.All(ctx, &messages); err != nil {
		return nil, err
	}

	// Prepare the bulk write operations to update unread messages
	var bulkOps []mongo.WriteModel
	for _, message := range messages {
		if !message.Read { // Assuming "Read" is a field in your Message struct
			filter := bson.M{"_id": message.ID}
			update := bson.M{"$set": bson.M{"read": true}}
			bulkOps = append(bulkOps, mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update))
		}
	}

	// Execute the bulk write operations to mark messages as read
	if len(bulkOps) > 0 {
		_, err := messageCollection.BulkWrite(ctx, bulkOps)
		if err != nil {
			return nil, err
		}
	}

	Response := ConvertMongoMessagesToMessages(messages)

	return Response, nil
}

func ConvertMongoMessageToMessage(mongoMsg MongoMessage) Message {
	return Message{
		Text:      mongoMsg.Text,
		RoomID:    mongoMsg.Chatid.Hex(), // Convert ObjectID to string
		SenderID:  mongoMsg.Userid,
		Gems:      mongoMsg.Gems,
		CreatedAt: mongoMsg.CreatedAt,
		Read:      mongoMsg.Read,
		Contents:  mongoMsg.Contents,
	}
}

func ConvertMongoMessagesToMessages(mongoMsgs []MongoMessage) []Message {
	var messages []Message
	for _, mongoMsg := range mongoMsgs {
		message := ConvertMongoMessageToMessage(mongoMsg)
		messages = append(messages, message)
	}
	return messages
}

func (s *store) ConvertToGrpcChat(chat *Chat) *pb.Chat {
	return &pb.Chat{
		Id:          chat.ID.Hex(),                   // Convert ObjectID to string
		CreatorId:   chat.CreatorID.Hex(),            // Convert ObjectID to string
		UserId:      chat.UserID.Hex(),               // Convert ObjectID to string
		CreatedAt:   timestamppb.New(chat.CreatedAt), // Convert time.Time to Timestamp
		Request:     chat.Request,
		LastMessage: chat.LastMessage,
		LastSentAt:  timestamppb.New(chat.LastSentAt),
	}
}

func (s *store) ConvertToGrpcUser(user *common.User) *pb.User {
	return &pb.User{
		Id:          user.ID.Hex(),
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		Username:    user.UserName,
		Email:       user.Email,
		BirthDate:   user.BirthDate,
		CreatedAt:   user.CreatedAt.String(),
		AvatarUri:   user.Avatar_uri,
		BannerUri:   user.Banner_uri,
		Verified:    user.Verified,
		Code:        user.Code,
		Followers:   int64(user.Followers),
		Following:   int64(user.Following),
		Description: user.Description,
	}
}

func (s *store) ConvertToGrpcMessage(message *Message) *pb.Message {
	return &pb.Message{
		Text:      message.Text,
		RoomId:    message.RoomID,
		ClientId:  message.SenderID.Hex(),             // Convert ObjectID to string
		Gems:      int32(message.Gems),                // Convert int to int32
		CreatedAt: timestamppb.New(message.CreatedAt), // Convert time.Time to Timestamp
		Read:      message.Read,
		Contents:  message.Contents,
	}
}

func (s *store) UploadMultiPartContent(files []*multipart.FileHeader) ([]string, error) {
	var contentUrls []string

	for _, fileHeader := range files {
		// Open the file
		file, err := fileHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open file: %v", err)
		}
		defer file.Close()

		// Read the file content
		fileBytes, err := ioutil.ReadAll(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %v", err)
		}

		// Generate a random filename for the file
		filename, err := common.GenerateRandomImageName("jpg")
		if err != nil {
			return nil, err
		}

		// Upload the file content to the S3 bucket
		err = s.webS3Client.Upload(fileBytes, filename)
		if err != nil {
			return nil, fmt.Errorf("failed to upload file to S3: %v", err)
		}

		// Create the URL for the uploaded content
		url := "https://" + CloudFlare_Link + "/" + filename
		log.Println("Uploaded to:", url)

		// Append the URL to the array
		contentUrls = append(contentUrls, url)
	}

	return contentUrls, nil
}

func (s *store) UpdateChatRequestStatus(chatID primitive.ObjectID) error {
	collection := s.database.Collection("chats")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Define the filter to locate the document to update
	filter := bson.M{"_id": chatID}

	// Define the update operation to set the "request" field to false
	update := bson.M{
		"$set": bson.M{
			"request": false,
		},
	}

	// Perform the update operation
	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) DeleteChat(chatID primitive.ObjectID) error {
	collection := s.database.Collection("chats")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Define the filter to locate the document to delete
	filter := bson.M{"_id": chatID}

	// Perform the delete operation
	result, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}

	// Optionally, check if a document was deleted
	if result.DeletedCount == 0 {
		return fmt.Errorf("no document found with _id: %v", chatID)
	}

	return nil
}

func (s *store) UpdateMessageReadStatus(messageID primitive.ObjectID) error {
	collection := s.database.Collection("messages")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Define the filter to locate the document to update
	filter := bson.M{"_id": messageID}

	// Define the update operation
	update := bson.M{
		"$set": bson.M{
			"read": true,
		},
	}

	// Perform the update operation
	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) SendMessageNotification(UserID primitive.ObjectID, ChatID primitive.ObjectID, text string) error {

	var payload common.ChatNotificationPayload

	payload.Chat_id = ChatID
	payload.User_id = UserID
	payload.Text = text

	marshalledRequest, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		return err
	}

	q, err := s.channel.QueueDeclare(broker.SendChatNotificationCreatedEvent, true, false, false, false, nil)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		return err
	}

	s.channel.PublishWithContext(context.Background(), "", q.Name, false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         marshalledRequest,
		DeliveryMode: amqp.Persistent,
	})
	return nil
}

func (s *store) GetChatPriceAndRules(userID primitive.ObjectID) (*common.ChatsSettingsPayload, error) {
	collection := s.database.Collection("chat-settings")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Define the filter to locate the document by ObjectID
	filter := bson.M{"_id": userID}

	// Define the result structure
	var result common.ChatsSettingsPayload

	// Perform the find operation
	err := collection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("no chat settings found for user_id: %s", userID)
		}
		return nil, fmt.Errorf("failed to retrieve chat settings: %v", err)
	}

	return &result, nil
}

func (s *store) IsChatRequestTrueAndUserID(chatID primitive.ObjectID) (bool, primitive.ObjectID, primitive.ObjectID, error) {
	collection := s.database.Collection("chats")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Define the filter by chat ID
	filter := bson.M{"_id": chatID}

	// Define a struct to hold the result
	var result struct {
		Request   bool               `bson:"request"`
		UserID    primitive.ObjectID `bson:"user_id"`
		CreatorID primitive.ObjectID `bson:"creator_id"`
	}

	// Find the chat document and decode the "request" field
	err := collection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, primitive.NilObjectID, primitive.NilObjectID, fmt.Errorf("chat with ID %v not found", chatID)
		}
		return false, primitive.NilObjectID, primitive.NilObjectID, fmt.Errorf("error finding chat: %v", err)
	}

	return result.Request, result.UserID, result.CreatorID, nil
}

func (s *store) CountUserMessagesInChat(chatID, userID primitive.ObjectID) (int64, error) {
	collection := s.database.Collection("messages")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Define the filter to count messages by user and chat
	filter := bson.M{
		"chat_id": chatID,
		"user_id": userID,
	}

	// Count the number of messages
	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("error counting user messages in chat: %v", err)
	}

	return count, nil
}
