package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"sort"
	"time"

	common "github.com/Eskiwi-Organization/infra/commons"
	pb "github.com/Eskiwi-Organization/infra/commons/api"
	aws_s3 "github.com/Eskiwi-Organization/infra/commons/aws_s3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type store struct {
	client         *mongo.Client
	database       *mongo.Database
	avatarS3Client *aws_s3.S3Client
	db             *sql.DB
}

var topCreatorsList []*pb.PublicUser

var QueryTimeoutDuration = 10 * time.Second

func NewStore(client *mongo.Client, avatarS3Client *aws_s3.S3Client, db *sql.DB) *store {
	return &store{client: client, database: client.Database(mongoDatabaseName), avatarS3Client: avatarS3Client, db: db}
}
func (s *store) Create(context.Context) error {
	return nil
}

func (s *store) CreateUser(user common.User) (primitive.ObjectID, error) {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := collection.InsertOne(ctx, user)
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

// GetUserByID implements UserStore.
func (s *store) GetUserByID(id string) (*common.User, error) {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Convert the string id to a primitive.ObjectID
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, fmt.Errorf("invalid id: %s", id)
	}

	filter := bson.M{"_id": objectID}
	var user common.User

	err = collection.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("user not found: id %s", id)
		}
		return nil, err
	}
	return &user, nil
}

func (s *store) GetUserByEmail(email string) (*common.User, error) {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"email": email}
	var user common.User

	err := collection.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}

	return &user, err
}

func (s *store) GetUserByUsername(username string) (*common.User, error) {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"username": username}
	var user common.User

	err := collection.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}

	return &user, err
}

func (s *store) GetUserByObjectID(id primitive.ObjectID) (*common.User, error) {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": id}
	var user common.User

	err := collection.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("user not found: id %s", id.Hex())
		}
		return nil, err
	}
	return &user, nil
}

func (s *store) UpdateAvatar(userID primitive.ObjectID, avatarURL string) error {
	//Se selecciona la coleccion y contexto
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": userID}
	update := bson.M{
		"$set": bson.M{
			"avatarUri": avatarURL,
		},
	}
	log.Println(avatarURL)

	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) UpdateBanner(userID primitive.ObjectID, bannerUri string) error {
	//Se selecciona la coleccion y contexto
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": userID}
	update := bson.M{
		"$set": bson.M{
			"bannerUri": bannerUri,
		},
	}

	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) UpdateUserCode(email string, code string) error {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"email": email}
	update := bson.M{"$set": bson.M{"code": code}}

	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) UpdateValidationStatus(email string) error {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"email": email}
	update := bson.M{"$set": bson.M{"verified": "true"}}

	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) CheckAndRetrieveAvatar(userID primitive.ObjectID) (string, error) {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": userID}
	var result struct {
		Avatar string `bson:"avatarUri"`
	}

	err := collection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return "", errors.New("user not found")
		}
		return "", err
	}

	if result.Avatar == "" {
		return "", nil
	}

	return result.Avatar, nil
}

func (s *store) CheckAndRetrieveBanner(userID primitive.ObjectID) (string, error) {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": userID}
	var result struct {
		Banner string `bson:"bannerUri"`
	}

	err := collection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return "", errors.New("user not found")
		}
		return "", err
	}

	if result.Banner == "" {
		return "", nil
	}

	return result.Banner, nil
}

func (s *store) CreateUserFollow(follow UserFollow) error {
	collection := s.database.Collection("user-follows")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if the follow relationship already exists
	filter := bson.M{
		"following_id": follow.FollowingID,
		"follower_id":  follow.FollowerID,
	}
	var existingFollow UserFollow
	err := collection.FindOne(ctx, filter).Decode(&existingFollow)
	if err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	if existingFollow.ID != primitive.NilObjectID {
		return fmt.Errorf("follow relationship already exists")
	}

	// Insert the new follow relationship
	_, err = collection.InsertOne(ctx, follow)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) DeleteUserFollow(follow UserFollow) error {
	collection := s.database.Collection("user-follows")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if the follow relationship exists
	filter := bson.M{
		"following_id": follow.FollowingID,
		"follower_id":  follow.FollowerID,
	}
	var existingFollow UserFollow
	err := collection.FindOne(ctx, filter).Decode(&existingFollow)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("follow relationship doesn't exist")
		}
		return err
	}

	// Delete the follow relationship
	_, err = collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) UserFollowerCount(userID primitive.ObjectID, increase_decrease int) error {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": userID}
	update := bson.M{"$inc": bson.M{"followers": increase_decrease}}

	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) UserFollowingCount(userID primitive.ObjectID, increase_decrease int) error {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": userID}
	update := bson.M{"$inc": bson.M{"following": increase_decrease}}

	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) CheckUserFollowLike(userFollow UserFollow) (bool, error) {
	collection := s.database.Collection("user-follows")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if the PostLike already exists
	filter := bson.M{
		"follower_id":  userFollow.FollowerID,
		"following_id": userFollow.FollowingID,
	}
	var existingUserFollow UserFollow
	err := collection.FindOne(ctx, filter).Decode(&existingUserFollow)
	if err != nil && err != mongo.ErrNoDocuments {
		return false, err
	}

	if existingUserFollow.ID != primitive.NilObjectID {
		return true, nil
	}

	return false, nil
}

func (s *store) UpdateUserDescription(description string, userID primitive.ObjectID) error {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": userID}
	update := bson.M{"$set": bson.M{"description": description}}

	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) AddExpoToken(userID primitive.ObjectID, token string) error {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Define the filter to locate the document to update
	filter := bson.M{"_id": userID}

	// Define the update operation to add the new token to the expoToken array only if it doesn't exist
	update := bson.M{
		"$addToSet": bson.M{
			"expoToken": token,
		},
	}

	// Perform the update operation
	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}

func (s *store) RemoveExpoToken(userID primitive.ObjectID, token string) error {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Define the filter to locate the document to update
	filter := bson.M{"_id": userID}

	// Define the update operation to remove the token from the expoToken array
	update := bson.M{
		"$pull": bson.M{
			"expoToken": token,
		},
	}

	// Perform the update operation
	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}

func ConvertProtoMsgToStruct(grpcMsg *pb.NotificationsSettings) (*common.NotificationsSettingsPayload, error) {
	// Convert the user_id string to a primitive.ObjectID
	userID, err := primitive.ObjectIDFromHex(grpcMsg.UserId)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %v", err)
	}

	// Create the NotificationsSettingsPayload struct with values from the gRPC message
	converted := &common.NotificationsSettingsPayload{
		User_id:        userID,
		UnreadMessages: grpcMsg.UnreadMessages,
		NewPost:        grpcMsg.New_Post,
		NewAboutEskiwi: grpcMsg.New_About_Eskiwi,
		NewSub:         grpcMsg.New_Sub,
		NewComment:     grpcMsg.New_Comment,
		NewLike:        grpcMsg.New_Like,
	}

	return converted, nil
}

func ConvertProtoChatSettingsToStruct(grpcChatSettings *pb.CreateChatsSettingsRequest) (*common.ChatsSettingsPayload, error) {
	// Convert the user_id string to a primitive.ObjectID
	userID, err := primitive.ObjectIDFromHex(grpcChatSettings.Chatsettings.GetUserId())
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %v", err)
	}

	// Create the ChatsSettingsPayload struct with values from the gRPC message
	converted := &common.ChatsSettingsPayload{
		User_id: userID,
		Price:   int(grpcChatSettings.Chatsettings.Price),
		Rules:   grpcChatSettings.Chatsettings.Rules,
	}

	return converted, nil
}

func (s *store) UpsertNotificationSettings(settings common.NotificationsSettingsPayload) error {
	collection := s.database.Collection("user-settings")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Define the filter to locate the document by userID
	filter := bson.M{"_id": settings.User_id}

	// Define the update operation to set all the fields in the NotificationsSettingsPayload
	update := bson.M{
		"$set": bson.M{
			"user_id":        settings.User_id, // Ensure user_id is set
			"unreadMessages": settings.UnreadMessages,
			"newPost":        settings.NewPost,
			"newAboutEskiwi": settings.NewAboutEskiwi,
			"newSub":         settings.NewSub,
			"newComment":     settings.NewComment,
			"newLike":        settings.NewLike,
		},
	}

	// Define the options to allow upsert operation (update if exists, insert if not)
	opts := options.Update().SetUpsert(true)

	// Perform the upsert operation
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to upsert notification settings: %v", err)
	}

	return nil
}

func (s *store) GetUserNotificationSettings(userID primitive.ObjectID) (*common.NotificationsSettingsPayload, error) {
	collection := s.database.Collection("user-settings")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"user_id": userID}
	var userSettings common.NotificationsSettingsPayload
	err := collection.FindOne(ctx, filter).Decode(&userSettings)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("user not found: id %s", userID.Hex())
		}
		return nil, err
	}

	return &userSettings, nil
}

func ConvertStructToProtoNotification(payload *common.NotificationsSettingsPayload) (*pb.NotificationsSettings, error) {
	// Convert the primitive.ObjectID to a string for the gRPC message
	userID := payload.User_id.Hex()

	// Create the gRPC message with values from the struct
	protoMsg := &pb.NotificationsSettings{
		UserId:           userID,
		UnreadMessages:   payload.UnreadMessages,
		New_Post:         payload.NewPost,
		New_About_Eskiwi: payload.NewAboutEskiwi,
		New_Sub:          payload.NewSub,
		New_Comment:      payload.NewComment,
		New_Like:         payload.NewLike,
	}

	return protoMsg, nil
}

func (s *store) GetUsersByUsername(username string) ([]common.PublicUser, error) {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Usamos una expresión regular para hacer una búsqueda parcial
	filter := bson.M{"username": bson.M{"$regex": username, "$options": "i"},
		"roles": "creator",
	}

	var users []common.PublicUser
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &users); err != nil {
		return nil, err
	}

	return users, nil
}

func convertPublicUserToProto(user common.PublicUser) *pb.PublicUser {
	return &pb.PublicUser{
		Id:          user.ID.Hex(),
		DisplayName: user.DisplayName,
		Username:    user.UserName,
		CreatedAt:   timestamppb.New(user.CreatedAt),
		AvatarUri:   user.Avatar_uri,
		BannerUri:   user.Banner_uri,
		Description: user.Description,
		Followers:   int64(user.Followers),
		Following:   int64(user.Following),
		Subscribers: int64(user.Subscribers),
	}
}

func (s *store) convertToSearchCreatorsResponse(users []common.PublicUser) (*pb.SearchCreatorsResponse, error) {
	var protoUsers []*pb.PublicUser

	for _, user := range users {
		protoUser := convertPublicUserToProto(user)
		protoUsers = append(protoUsers, protoUser)
	}

	// Create the SearchCreatorsResponse message
	response := &pb.SearchCreatorsResponse{
		Publicusers: protoUsers,
	}

	return response, nil
}

func (s *store) GetTopUsersByFollowers() ([]common.PublicUser, error) {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Sort by "followers" in descending order and limit the result to 10
	opts := options.Find()
	opts.SetSort(bson.M{"followers": -1})
	opts.SetLimit(10)

	var users []common.PublicUser
	cursor, err := collection.Find(ctx, bson.M{}, opts) // Empty filter to retrieve all users
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &users); err != nil {
		return nil, err
	}

	return users, nil
}

func (s *store) convertToSearchTopCreatorsResponse(users []common.PublicUser) (*pb.GetTopCreatorResponse, error) {
	var protoUsers []*pb.PublicUser

	for _, user := range users {
		protoUser := convertPublicUserToProto(user)
		protoUsers = append(protoUsers, protoUser)
	}

	// Create the SearchCreatorsResponse message
	response := &pb.GetTopCreatorResponse{
		TopCreators: protoUsers,
	}

	return response, nil
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
		err = s.avatarS3Client.Upload(fileBytes, filename)
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

func (s *store) UpdateUserPassword(email string, newPassword string) error {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"email": email}
	update := bson.M{"$set": bson.M{"password": newPassword}}

	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) UpdateUsername(username string, userID primitive.ObjectID) error {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": userID}
	update := bson.M{"$set": bson.M{"username": username}}

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

func (s *store) TotalUnreadMessages(userID primitive.ObjectID) (int64, error) {
	chatsCollection := s.database.Collection("chats")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // Increased timeout for potentially two operations
	defer cancel()

	// Find all chat IDs where the user is either creator or a participant
	chatFilter := bson.M{
		"$or": []bson.M{
			{"creator_id": userID},
			{"user_id": userID},
		},
	}
	cursor, err := chatsCollection.Find(ctx, chatFilter)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	var totalUnread int64
	// Iterate over all found chat documents
	for cursor.Next(ctx) {
		var chat struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		if err := cursor.Decode(&chat); err != nil {
			return 0, err // handle the error appropriately
		}

		// Count unread messages for each chat
		unreadCount, err := s.CountUnreadMessages(chat.ID, userID)
		if err != nil {
			return 0, err // decide whether to continue or return the error
		}
		totalUnread += unreadCount
	}

	if err := cursor.Err(); err != nil {
		return 0, err
	}

	return totalUnread, nil
}

func (s *store) UpsertChatPriceAndRules(settings common.ChatsSettingsPayload) error {
	collection := s.database.Collection("chat-settings")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Define the filter to locate the document by userID
	filter := bson.M{"_id": settings.User_id}

	// Define the update operation to set all the fields in the NotificationsSettingsPayload
	update := bson.M{
		"$set": bson.M{
			"user_id": settings.User_id, // Ensure user_id is set
			"price":   settings.Price,
			"rules":   settings.Rules,
		},
	}

	// Define the options to allow upsert operation (update if exists, insert if not)
	opts := options.Update().SetUpsert(true)

	// Perform the upsert operation
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to upsert notification settings: %v", err)
	}

	return nil
}

func (s *store) GetChatPriceAndRules(userID primitive.ObjectID) (*common.ChatsSettingsPayload, error) {
	collection := s.database.Collection("chat-settings")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Define the filter to locate the document by userID
	filter := bson.M{"_id": userID}

	// Define a variable to store the result
	var settings common.ChatsSettingsPayload

	// Find the document in the collection and decode it into the settings variable
	err := collection.FindOne(ctx, filter).Decode(&settings)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("no chat settings found for user ID: %v", userID)
		}
		return nil, fmt.Errorf("error retrieving chat settings: %v", err)
	}

	return &settings, nil
}

func (s *store) GetUserGemDonationsSQLAndMongo(userID primitive.ObjectID) (*TransactionsResponse, error) {
	// SQL query para obtener las transacciones desde la tabla gem_donations
	query := `
        SELECT transaction_id, sender_id, receiver_id, gem_amount, created_at
        FROM gem_donations
        WHERE sender_id = ? OR receiver_id = ?
    `

	// Convertimos el userID a string para usarlo en SQL
	userIDStr := userID.Hex()

	// Set a context timeout para la consulta SQL
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeoutDuration)
	defer cancel()

	// Ejecutar la consulta SQL para obtener las transacciones
	rows, err := s.db.QueryContext(ctx, query, userIDStr, userIDStr)
	if err != nil {
		return nil, fmt.Errorf("error querying gem donations: %v", err)
	}
	defer rows.Close()

	var transactions []Transaction

	// Iterar sobre los resultados de SQL
	for rows.Next() {
		var gemDonation struct {
			TransactionID int64
			SenderID      string
			ReceiverID    string
			GemAmount     int
			CreatedAt     time.Time
		}
		if err := rows.Scan(&gemDonation.TransactionID, &gemDonation.SenderID, &gemDonation.ReceiverID, &gemDonation.GemAmount, &gemDonation.CreatedAt); err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}

		// Determinar si la transacción es "outgoing" o "ingoing"
		var transactionType string
		var userToFetch string
		if gemDonation.SenderID == userIDStr {
			transactionType = "outgoing"
			userToFetch = gemDonation.ReceiverID
		} else {
			transactionType = "ingoing"
			userToFetch = gemDonation.SenderID
		}

		// Convertir el userToFetch a ObjectID de MongoDB
		userObjID, err := primitive.ObjectIDFromHex(userToFetch)
		if err != nil {
			return nil, fmt.Errorf("error converting userID to ObjectID: %v", err)
		}

		// Buscar el usuario en MongoDB
		var fromUser common.PublicUser
		userCollection := s.database.Collection("users")
		err = userCollection.FindOne(ctx, bson.M{"_id": userObjID}).Decode(&fromUser)
		if err != nil {
			return nil, fmt.Errorf("error fetching user from MongoDB: %v", err)
		}

		// Crear la transacción y agregarla a la lista
		transaction := Transaction{
			ID:              gemDonation.TransactionID,
			GemAmount:       gemDonation.GemAmount,
			TransactionType: "comments", // Tipo de transacción es "comentario"
			Type:            transactionType,
			From:            fromUser,
			Pending:         false,                 // Siempre false para gem_donations
			CreatedAt:       gemDonation.CreatedAt, // Tiempo desde SQL
		}

		transactions = append(transactions, transaction)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Responder con las transacciones formateadas
	response := &TransactionsResponse{
		Transactions: transactions,
	}

	return response, nil
}

func (s *store) GetUserChatTransactionsSQLAndMongo(userID primitive.ObjectID) (*TransactionsResponse, error) {
	// SQL query para obtener las transacciones desde la tabla chat_gems
	query := `
        SELECT transaction_id, sender_id, receiver_id, gem_amount, accepted, denied, created_at
        FROM chat_gems
        WHERE sender_id = ? OR receiver_id = ?
    `

	// Convertimos el userID a string para usarlo en SQL
	userIDStr := userID.Hex()

	// Set a context timeout para la consulta SQL
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeoutDuration)
	defer cancel()

	// Ejecutar la consulta SQL para obtener las transacciones
	rows, err := s.db.QueryContext(ctx, query, userIDStr, userIDStr)
	if err != nil {
		return nil, fmt.Errorf("error querying chat gems: %v", err)
	}
	defer rows.Close()

	var transactions []Transaction

	// Iterar sobre los resultados de SQL
	for rows.Next() {
		var chatGem struct {
			TransactionID int64
			SenderID      string
			ReceiverID    string
			GemAmount     int
			Accepted      bool
			Denied        bool
			CreatedAt     time.Time
		}
		if err := rows.Scan(&chatGem.TransactionID, &chatGem.SenderID, &chatGem.ReceiverID, &chatGem.GemAmount, &chatGem.Accepted, &chatGem.Denied, &chatGem.CreatedAt); err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}

		// Determinar si la transacción es "outgoing" o "ingoing"
		var transactionType string
		var userToFetch string
		if chatGem.SenderID == userIDStr {
			transactionType = "outgoing"
			userToFetch = chatGem.ReceiverID
		} else {
			transactionType = "ingoing"
			userToFetch = chatGem.SenderID
		}

		// Convertir el userToFetch a ObjectID de MongoDB
		userObjID, err := primitive.ObjectIDFromHex(userToFetch)
		if err != nil {
			return nil, fmt.Errorf("error converting userID to ObjectID: %v", err)
		}

		// Buscar el usuario en MongoDB
		var fromUser common.PublicUser
		userCollection := s.database.Collection("users")
		err = userCollection.FindOne(ctx, bson.M{"_id": userObjID}).Decode(&fromUser)
		if err != nil {
			return nil, fmt.Errorf("error fetching user from MongoDB: %v", err)
		}

		// Determinar si la transacción está pendiente
		isPending := !chatGem.Accepted && !chatGem.Denied

		// Crear la transacción y agregarla a la lista
		transaction := Transaction{
			ID:              chatGem.TransactionID,
			GemAmount:       chatGem.GemAmount,
			TransactionType: "chat_request", // Tipo de transacción es "chats"
			Type:            transactionType,
			From:            fromUser,
			Pending:         isPending,         // Pendiente depende de los campos `accepted` y `denied`
			CreatedAt:       chatGem.CreatedAt, // Tiempo desde SQL
		}

		transactions = append(transactions, transaction)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Responder con las transacciones formateadas
	response := &TransactionsResponse{
		Transactions: transactions,
	}

	return response, nil
}

func (s *store) GetUserMessageGemTransactionsSQLAndMongo(userID primitive.ObjectID) (*TransactionsResponse, error) {
	// SQL query para obtener las transacciones desde la tabla message_gems
	query := `
        SELECT transaction_id, sender_id, receiver_id, gem_amount, created_at
        FROM message_gems
        WHERE sender_id = ? OR receiver_id = ?
    `

	// Convertimos el userID a string para usarlo en SQL
	userIDStr := userID.Hex()

	// Set a context timeout para la consulta SQL
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeoutDuration)
	defer cancel()

	// Ejecutar la consulta SQL para obtener las transacciones
	rows, err := s.db.QueryContext(ctx, query, userIDStr, userIDStr)
	if err != nil {
		return nil, fmt.Errorf("error querying message gems: %v", err)
	}
	defer rows.Close()

	var transactions []Transaction

	// Iterar sobre los resultados de SQL
	for rows.Next() {
		var messageGem struct {
			TransactionID int64
			SenderID      string
			ReceiverID    string
			GemAmount     int
			CreatedAt     time.Time
		}
		if err := rows.Scan(&messageGem.TransactionID, &messageGem.SenderID, &messageGem.ReceiverID, &messageGem.GemAmount, &messageGem.CreatedAt); err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}

		// Determinar si la transacción es "outgoing" o "ingoing"
		var transactionType string
		var userToFetch string
		if messageGem.SenderID == userIDStr {
			transactionType = "outgoing"
			userToFetch = messageGem.ReceiverID
		} else {
			transactionType = "ingoing"
			userToFetch = messageGem.SenderID
		}

		// Convertir el userToFetch a ObjectID de MongoDB
		userObjID, err := primitive.ObjectIDFromHex(userToFetch)
		if err != nil {
			return nil, fmt.Errorf("error converting userID to ObjectID: %v", err)
		}

		// Buscar el usuario en MongoDB
		var fromUser common.PublicUser
		userCollection := s.database.Collection("users")
		err = userCollection.FindOne(ctx, bson.M{"_id": userObjID}).Decode(&fromUser)
		if err != nil {
			return nil, fmt.Errorf("error fetching user from MongoDB: %v", err)
		}

		// Crear la transacción y agregarla a la lista
		transaction := Transaction{
			ID:              messageGem.TransactionID,
			GemAmount:       messageGem.GemAmount,
			TransactionType: "chat_message", // Tipo de transacción es "message"
			Type:            transactionType,
			From:            fromUser,
			Pending:         false,                // Siempre false para message_gems
			CreatedAt:       messageGem.CreatedAt, // Tiempo desde SQL
		}

		transactions = append(transactions, transaction)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Responder con las transacciones formateadas
	response := &TransactionsResponse{
		Transactions: transactions,
	}

	return response, nil
}

func (s *store) GetUserGemPurchaseTransactionsSQLAndMongo(userID primitive.ObjectID) (*TransactionsResponse, error) {
	// SQL query para obtener las transacciones desde la tabla gem_purchases
	query := `
        SELECT transaction_id, buyer_id, gem_amount, created_at
        FROM gem_purchases
        WHERE buyer_id = ?
    `

	// Convertimos el userID a string para usarlo en SQL
	userIDStr := userID.Hex()

	// Set a context timeout para la consulta SQL
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeoutDuration)
	defer cancel()

	// Ejecutar la consulta SQL para obtener las transacciones
	rows, err := s.db.QueryContext(ctx, query, userIDStr)
	if err != nil {
		return nil, fmt.Errorf("error querying gem purchases: %v", err)
	}
	defer rows.Close()

	var transactions []Transaction

	// Iterar sobre los resultados de SQL
	for rows.Next() {
		var gemPurchase struct {
			TransactionID int64
			BuyerID       string
			GemAmount     int
			CreatedAt     time.Time
		}
		if err := rows.Scan(&gemPurchase.TransactionID, &gemPurchase.BuyerID, &gemPurchase.GemAmount, &gemPurchase.CreatedAt); err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}

		// Convertir el buyerID a ObjectID de MongoDB para buscar el usuario
		buyerObjID, err := primitive.ObjectIDFromHex(gemPurchase.BuyerID)
		if err != nil {
			return nil, fmt.Errorf("error converting buyerID to ObjectID: %v", err)
		}

		// Buscar el usuario en MongoDB
		var fromUser common.PublicUser
		userCollection := s.database.Collection("users")
		err = userCollection.FindOne(ctx, bson.M{"_id": buyerObjID}).Decode(&fromUser)
		if err != nil {
			return nil, fmt.Errorf("error fetching user from MongoDB: %v", err)
		}

		// Crear la transacción y agregarla a la lista
		transaction := Transaction{
			ID:              gemPurchase.TransactionID,
			GemAmount:       gemPurchase.GemAmount,
			TransactionType: "gem_purchase", // Tipo de transacción es "purchase"
			Type:            "ingoing",      // Como es una compra, siempre será "ingoing"
			From:            fromUser,
			Pending:         false,                 // Siempre false para gem_purchases
			CreatedAt:       gemPurchase.CreatedAt, // Tiempo desde SQL
		}

		transactions = append(transactions, transaction)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Responder con las transacciones formateadas
	response := &TransactionsResponse{
		Transactions: transactions,
	}

	return response, nil
}

func (s *store) GetUserSubscriptionPurchasesSQLAndMongo(userID primitive.ObjectID) (*TransactionsResponse, error) {
	// SQL query to get transactions from the sub_purchases table
	query := `
        SELECT transaction_id, buyer_id, creator_id, tier, created_at
        FROM sub_purchases
        WHERE buyer_id = ?
    `

	// Convert the userID to a string to use in the SQL query
	userIDStr := userID.Hex()

	// Set a context timeout for the SQL query
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeoutDuration)
	defer cancel()

	// Execute the SQL query to get the transactions
	rows, err := s.db.QueryContext(ctx, query, userIDStr)
	if err != nil {
		return nil, fmt.Errorf("error querying subscription purchases: %v", err)
	}
	defer rows.Close()

	var transactions []Transaction

	// Iterate over the SQL results
	for rows.Next() {
		var subPurchase struct {
			TransactionID int64
			BuyerID       string
			CreatorID     string
			Tier          int
			CreatedAt     time.Time
		}
		if err := rows.Scan(&subPurchase.TransactionID, &subPurchase.BuyerID, &subPurchase.CreatorID, &subPurchase.Tier, &subPurchase.CreatedAt); err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}

		// Convert the buyerID to ObjectID to fetch the user from MongoDB
		buyerObjID, err := primitive.ObjectIDFromHex(subPurchase.CreatorID)
		if err != nil {
			return nil, fmt.Errorf("error converting buyerID to ObjectID: %v", err)
		}

		// Convert the creatorID to ObjectID to fetch the creator from MongoDB
		creatorObjID, err := primitive.ObjectIDFromHex(subPurchase.CreatorID)
		if err != nil {
			return nil, fmt.Errorf("error converting creatorID to ObjectID: %v", err)
		}

		// Fetch the buyer's information from MongoDB
		var buyerUser common.PublicUser
		userCollection := s.database.Collection("users")
		err = userCollection.FindOne(ctx, bson.M{"_id": buyerObjID}).Decode(&buyerUser)
		if err != nil {
			return nil, fmt.Errorf("error fetching buyer from MongoDB: %v", err)
		}

		// Fetch the creator's information from MongoDB
		var creatorUser common.PublicUser
		err = userCollection.FindOne(ctx, bson.M{"_id": creatorObjID}).Decode(&creatorUser)
		if err != nil {
			return nil, fmt.Errorf("error fetching creator from MongoDB: %v", err)
		}

		// Create the transaction and add it to the list
		transaction := Transaction{
			ID:              subPurchase.TransactionID,
			GemAmount:       subPurchase.Tier,        // Using Tier to represent the transaction amount
			TransactionType: "subscription_purchase", // Transaction type
			Type:            "outgoing",              // Type is ingoing for a subscription purchase
			From:            buyerUser,               // User who made the purchase
			Pending:         false,                   // Assuming always false for purchases
			CreatedAt:       subPurchase.CreatedAt,   // Time from SQL
		}

		transactions = append(transactions, transaction)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Return the formatted transactions
	response := &TransactionsResponse{
		Transactions: transactions,
	}

	return response, nil
}

func (s *store) ConvertTransactionStructToGRPC(transactions []Transaction) *pb.TransactionsResponse {
	var grpcTransactions []*pb.Transaction

	// Itera sobre la lista de transacciones en Go y convierte cada una a su formato gRPC
	for _, t := range transactions {
		// Convierte la estructura PublicUser de Go a la estructura gRPC PublicUser
		grpcPublicUser := &pb.PublicUser{
			Id:          t.From.ID.Hex(), // Suponiendo que el ID es un ObjectID de MongoDB
			DisplayName: t.From.DisplayName,
			Username:    t.From.UserName,
			CreatedAt:   timestamppb.New(t.From.CreatedAt),
			Followers:   int64(t.From.Followers),
			Following:   int64(t.From.Following),
			Subscribers: int64(t.From.Subscribers),
			AvatarUri:   t.From.Avatar_uri,
			BannerUri:   t.From.Banner_uri,
			Description: t.From.Description,
		}

		// Convierte la estructura Transaction de Go a la estructura gRPC Transaction
		grpcTransaction := &pb.Transaction{
			Id:              t.ID,
			GemAmount:       int32(t.GemAmount),
			TransactionType: t.TransactionType,
			Type:            t.Type,
			From:            grpcPublicUser,
			Pending:         t.Pending,
			CreatedAt:       timestamppb.New(t.CreatedAt), // Convierte el campo CreatedAt a un timestamppb.Timestamp
		}

		// Añade la transacción convertida a la lista de transacciones en gRPC
		grpcTransactions = append(grpcTransactions, grpcTransaction)
	}

	// Devuelve una respuesta TransactionsResponse en gRPC
	return &pb.TransactionsResponse{
		Transactions: grpcTransactions,
	}
}

func (s *store) GetAllUserTransactions(userID primitive.ObjectID, page, pageSize int) (*pb.TransactionsResponse, error) {
	// Llama a las funciones que recuperan las diferentes transacciones
	gemDonations, err := s.GetUserGemDonationsSQLAndMongo(userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching gem donations: %v", err)
	}

	chatTransactions, err := s.GetUserChatTransactionsSQLAndMongo(userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching chat gems: %v", err)
	}

	messageGemTransactions, err := s.GetUserMessageGemTransactionsSQLAndMongo(userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching message gems: %v", err)
	}

	purchaseTransactions, err := s.GetUserGemPurchaseTransactionsSQLAndMongo(userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching gem purchases: %v", err)
	}

	subscriptionTransactions, err := s.GetUserSubscriptionPurchasesSQLAndMongo(userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching subscription purchases: %v", err)
	}

	// Junta todas las transacciones en una sola lista
	allTransactions := append(gemDonations.Transactions, chatTransactions.Transactions...)
	allTransactions = append(allTransactions, messageGemTransactions.Transactions...)
	allTransactions = append(allTransactions, purchaseTransactions.Transactions...)
	allTransactions = append(allTransactions, subscriptionTransactions.Transactions...)

	// Ordena las transacciones por fecha de más reciente a más antigua
	sort.SliceStable(allTransactions, func(i, j int) bool {
		return allTransactions[i].CreatedAt.After(allTransactions[j].CreatedAt)
	})

	// Implementar la paginación: obtener sólo las primeras 10 transacciones según la página
	startIndex := (page - 1) * pageSize
	endIndex := startIndex + pageSize

	// Verifica que los índices no excedan los límites de la lista
	if startIndex >= len(allTransactions) {
		return nil, nil // No hay más transacciones
	}
	if endIndex > len(allTransactions) {
		endIndex = len(allTransactions)
	}

	// Obtén las transacciones paginadas
	paginatedTransactions := allTransactions[startIndex:endIndex]

	// Convierte las transacciones paginadas a gRPC
	grpcResponse := s.ConvertTransactionStructToGRPC(paginatedTransactions)

	return grpcResponse, nil
}

func (s *store) GetAllValidUserTransactions(userID primitive.ObjectID, page, pageSize int) (*pb.TransactionsResponse, error) {
	// Llama a las funciones que recuperan las diferentes transacciones
	gemDonations, err := s.GetUserGemDonationsSQLAndMongo(userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching gem donations: %v", err)
	}

	chatTransactions, err := s.GetUserChatTransactionsSQLAndMongo(userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching chat gems: %v", err)
	}

	messageGemTransactions, err := s.GetUserMessageGemTransactionsSQLAndMongo(userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching message gems: %v", err)
	}

	purchaseTransactions, err := s.GetUserGemPurchaseTransactionsSQLAndMongo(userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching gem purchases: %v", err)
	}

	subscriptionTransactions, err := s.GetUserSubscriptionPurchasesSQLAndMongo(userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching subscription purchases: %v", err)
	}

	// Junta todas las transacciones en una sola lista
	allTransactions := append(gemDonations.Transactions, chatTransactions.Transactions...)
	allTransactions = append(allTransactions, messageGemTransactions.Transactions...)
	allTransactions = append(allTransactions, purchaseTransactions.Transactions...)
	allTransactions = append(allTransactions, subscriptionTransactions.Transactions...)

	// Ordena las transacciones por fecha de más reciente a más antigua
	sort.SliceStable(allTransactions, func(i, j int) bool {
		return allTransactions[i].CreatedAt.After(allTransactions[j].CreatedAt)
	})

	// Implementar la paginación: obtener sólo las primeras 10 transacciones según la página
	startIndex := (page - 1) * pageSize
	endIndex := startIndex + pageSize

	// Verifica que los índices no excedan los límites de la lista
	if startIndex >= len(allTransactions) {
		return nil, nil // No hay más transacciones
	}
	if endIndex > len(allTransactions) {
		endIndex = len(allTransactions)
	}

	// Obtén las transacciones paginadas
	paginatedTransactions := allTransactions[startIndex:endIndex]

	// Convierte las transacciones paginadas a gRPC
	grpcResponse := s.ConvertTransactionStructToGRPC(paginatedTransactions)

	return grpcResponse, nil
}

func (s *store) ConvertPBToSubscriptionTiers(pbTier *pb.SubscriptionTiers) (*common.SubscriptionTiers, error) {
	// Convert CreatorId from string to ObjectID
	creatorID, err := primitive.ObjectIDFromHex(pbTier.CreatorId)
	if err != nil {
		return nil, fmt.Errorf("invalid CreatorID: %v", err)
	}

	// Map fields from pb.SubscriptionTiers to SubscriptionTiers
	tier := &common.SubscriptionTiers{
		CreatorID: creatorID,
		Name:      pbTier.Name,
		Benefits:  pbTier.Benefits,
		Tier:      int(pbTier.Tier), // Convert int32 to int
	}

	return tier, nil
}

func (s *store) CreateSubscriptionTier(tier common.SubscriptionTiers) (primitive.ObjectID, error) {
	collection := s.database.Collection("subscription-tiers")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First, check if the subscription tier already exists
	var existingTier common.SubscriptionTiers
	filter := bson.M{
		"creator_id": tier.CreatorID,
		"tier":       tier.Tier,
	}
	err := collection.FindOne(ctx, filter).Decode(&existingTier)
	if err == nil {
		// A subscription tier with the same CreatorID and Tier already exists
		return primitive.NilObjectID, fmt.Errorf("subscription tier for creator with tier %d already exists", tier.Tier)
	}

	if err != mongo.ErrNoDocuments {
		// There was an error in the query
		return primitive.NilObjectID, fmt.Errorf("error checking for existing subscription tier: %v", err)
	}

	// No existing subscription tier, proceed with the insertion
	result, err := collection.InsertOne(ctx, tier)
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("error inserting new subscription tier: %v", err)
	}

	// Extract the inserted ID, assuming it's an ObjectID
	insertedID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return primitive.NilObjectID, fmt.Errorf("inserted document ID is not an ObjectID")
	}

	return insertedID, nil
}

func (s *store) UpdateSubscriptionTier(tier common.SubscriptionTiers) error {
	collection := s.database.Collection("subscription-tiers")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if the subscription tier exists
	filter := bson.M{
		"creator_id": tier.CreatorID,
		"tier":       tier.Tier,
	}

	update := bson.M{
		"$set": bson.M{
			"name":     tier.Name,
			"benefits": tier.Benefits,
		},
	}

	// Perform the update
	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("error updating subscription tier: %v", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("subscription tier for creator with tier %d not found", tier.Tier)
	}

	return nil
}

func (s *store) ConvertSubscriptionTiersToPB(tier *common.SubscriptionTiers) (*pb.SubscriptionTiers, error) {
	// Convert CreatorID from ObjectID to string
	creatorID := tier.CreatorID.Hex()

	// Map fields from SubscriptionTiers to pb.SubscriptionTiers
	pbTier := &pb.SubscriptionTiers{
		CreatorId: creatorID,
		Name:      tier.Name,
		Benefits:  tier.Benefits,
		Tier:      int32(tier.Tier), // Convert int to int32
	}

	return pbTier, nil
}

func (s *store) GetSubscriptionTiers(creatorID, userID primitive.ObjectID) ([]common.SubscriptionTiers, error) {
	collection := s.database.Collection("subscription-tiers")
	subCollection := s.database.Collection("user-subscriptions") // Collection where user subscriptions are stored
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Filter by creatorID
	filter := bson.M{
		"creator_id": creatorID,
	}

	// Sort by tier in ascending order
	opts := options.Find().SetSort(bson.D{{Key: "tier", Value: 1}})

	// Execute the query to get subscription tiers
	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("error fetching subscription tiers: %v", err)
	}
	defer cursor.Close(ctx)

	var tiers []common.SubscriptionTiers
	if err = cursor.All(ctx, &tiers); err != nil {
		return nil, fmt.Errorf("error decoding subscription tiers: %v", err)
	}

	// Now for each tier, check if the user has a valid subscription for that tier
	for i, tier := range tiers {
		subscriptionFilter := bson.M{
			"user_id":    userID,
			"creator_id": creatorID,
			"tier":       tier.Tier,
			"active":     true, // Only check if the subscription is active
		}

		// Check if a valid subscription exists for the user
		var subscription common.Subscription
		err := subCollection.FindOne(ctx, subscriptionFilter).Decode(&subscription)
		if err == nil {
			// If a valid subscription exists, mark the tier as subscribed
			tiers[i].Subscribed = true
		} else if err != mongo.ErrNoDocuments {
			// Handle any other error except for no documents found
			return nil, fmt.Errorf("error checking user subscription: %v", err)
		}
	}

	return tiers, nil
}

func (s *store) ConvertSubscriptionTierToPB(tier *common.SubscriptionTiers) *pb.SubscriptionTiers {
	return &pb.SubscriptionTiers{
		Id:         tier.ID.Hex(),
		CreatorId:  tier.CreatorID.Hex(),
		Name:       tier.Name,
		Benefits:   tier.Benefits,
		Tier:       int32(tier.Tier), // Convert int to int32
		Subscribed: tier.Subscribed,
	}
}

func (s *store) ConvertSubscriptionTiersListToPB(tiers []common.SubscriptionTiers) []*pb.SubscriptionTiers {
	var pbTiers []*pb.SubscriptionTiers

	// Iterate over the list and convert each SubscriptionTier to gRPC
	for _, tier := range tiers {
		pbTiers = append(pbTiers, s.ConvertSubscriptionTierToPB(&tier))
	}

	return pbTiers
}

func (s *store) GetSubscriptionTiersAsPB(creatorID, UserID primitive.ObjectID) ([]*pb.SubscriptionTiers, error) {
	tiers, err := s.GetSubscriptionTiers(creatorID, UserID)
	if err != nil {
		return nil, err
	}

	// Convert the list to gRPC format
	return s.ConvertSubscriptionTiersListToPB(tiers), nil
}

func (s *store) FindTierMembers(creatorID primitive.ObjectID, tier, page, pageSize int) ([]*pb.TierMember, error) {
	collection := s.database.Collection("user-subscriptions")
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeoutDuration)
	defer cancel()

	// Build the query based on tier value
	var filter bson.M
	if tier == 0 {
		filter = bson.M{"creator_id": creatorID, "pending": false, "active": true, "valid": true}
	} else {
		filter = bson.M{"creator_id": creatorID, "tier": tier, "pending": false, "active": true, "valid": true}
	}

	// Find options for sorting by tier descending and pagination
	opts := options.Find().
		SetSort(bson.D{{Key: "tier", Value: -1}}).      // Sort by tier in descending order
		SetProjection(bson.M{"user_id": 1, "tier": 1}). // Only include user_id and tier in the results
		SetSkip(int64((page - 1) * pageSize)).          // Skip the documents that come before the current page
		SetLimit(int64(pageSize))                       // Limit the results to the size of the page

	// Find the subscriptions
	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("error finding subscriptions: %v", err)
	}
	defer cursor.Close(ctx)

	var results []struct {
		UserID primitive.ObjectID `bson:"user_id"`
		Tier   int                `bson:"tier"`
	}
	if err = cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("error decoding subscription results: %v", err)
	}

	// Retrieve user details and prepare the TierMember list
	var members []*pb.TierMember
	for _, result := range results {
		user, err := s.GetUserByObjectID(result.UserID)
		if err != nil {
			return nil, fmt.Errorf("error retrieving user details: %v", err)
		}

		publicUser := convertUserToPublicUser(*user)
		member := &pb.TierMember{
			Tier: int32(result.Tier),
			User: convertPublicUserToProto(publicUser), // Convert common.User to pb.PublicUser
		}
		members = append(members, member)
	}

	return members, nil
}

func convertUserToPublicUser(user common.User) common.PublicUser {
	return common.PublicUser{
		ID:          user.ID,
		DisplayName: user.DisplayName,
		UserName:    user.UserName,
		CreatedAt:   user.CreatedAt,
		Followers:   user.Followers,
		Following:   user.Following,
		Subscribers: user.Subscribers,
		Avatar_uri:  user.Avatar_uri,
		Banner_uri:  user.Banner_uri,
		Description: user.Description,
		Roles:       user.Roles,
	}
}

func (s *store) UpdateUserDetails(lastName string, username string, userID primitive.ObjectID) error {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create the filter for the user by their ID
	filter := bson.M{"_id": userID}

	// Create the update object to set both lastName and username
	update := bson.M{
		"$set": bson.M{
			"lastName": lastName,
			"username": username, // Assuming the bson key for UserName is "username"
		},
	}

	// Execute the update
	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("error updating user details: %v", err)
	}

	return nil
}

func (s *store) IsAdmin(user common.User) bool {
	for _, role := range user.Roles {
		if role == "admin" {
			return true
		}
	}
	return false
}

func (s *store) BanUser(userID, adminID primitive.ObjectID) error {
	// Selecciona la colección de usuarios y configura el contexto
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Filtro para encontrar al usuario por su ID
	filter := bson.M{"_id": userID}
	update := bson.M{
		"$set": bson.M{
			"banned": true,
		},
	}

	// Actualiza el estado de "banned" del usuario
	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	// Registra el ban en la colección 'bans'
	banCollection := s.database.Collection("bans")
	banRecord := bson.M{
		"user_id":   userID,
		"admin_id":  adminID,
		"banned_at": time.Now(), // Fecha y hora del ban
	}

	_, err = banCollection.InsertOne(ctx, banRecord)
	if err != nil {
		return err
	}

	return nil
}

func (s *store) GetUserValidReceiverChatTransactionsSQLAndMongo(userID primitive.ObjectID) (*TransactionsResponse, error) {
	// SQL query para obtener las transacciones desde la tabla chat_gems donde el user es el receiver y valid es 1
	query := `
        SELECT transaction_id, sender_id, receiver_id, gem_amount, accepted, denied, created_at
        FROM chat_gems
        WHERE receiver_id = ? AND valid = 1
    `

	// Convertimos el userID a string para usarlo en SQL
	userIDStr := userID.Hex()

	// Set a context timeout para la consulta SQL
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeoutDuration)
	defer cancel()

	// Ejecutar la consulta SQL para obtener las transacciones
	rows, err := s.db.QueryContext(ctx, query, userIDStr)
	if err != nil {
		return nil, fmt.Errorf("error querying chat gems: %v", err)
	}
	defer rows.Close()

	var transactions []Transaction

	// Iterar sobre los resultados de SQL
	for rows.Next() {
		var chatGem struct {
			TransactionID int64
			SenderID      string
			ReceiverID    string
			GemAmount     int
			Accepted      bool
			Denied        bool
			CreatedAt     time.Time
		}
		if err := rows.Scan(&chatGem.TransactionID, &chatGem.SenderID, &chatGem.ReceiverID, &chatGem.GemAmount, &chatGem.Accepted, &chatGem.Denied, &chatGem.CreatedAt); err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}

		// Convertir el senderID a ObjectID de MongoDB para buscar el usuario
		senderObjID, err := primitive.ObjectIDFromHex(chatGem.SenderID)
		if err != nil {
			return nil, fmt.Errorf("error converting senderID to ObjectID: %v", err)
		}

		// Buscar el usuario en MongoDB
		var fromUser common.PublicUser
		userCollection := s.database.Collection("users")
		err = userCollection.FindOne(ctx, bson.M{"_id": senderObjID}).Decode(&fromUser)
		if err != nil {
			return nil, fmt.Errorf("error fetching user from MongoDB: %v", err)
		}

		// Determinar si la transacción está pendiente
		isPending := !chatGem.Accepted && !chatGem.Denied

		// Crear la transacción y agregarla a la lista
		transaction := Transaction{
			ID:              chatGem.TransactionID,
			GemAmount:       chatGem.GemAmount,
			TransactionType: "chat_request", // Tipo de transacción es "chats"
			Type:            "ingoing",      // Siempre es ingoing ya que el usuario es el receiver
			From:            fromUser,
			Pending:         isPending,         // Pendiente depende de los campos `accepted` y `denied`
			CreatedAt:       chatGem.CreatedAt, // Tiempo desde SQL
		}

		transactions = append(transactions, transaction)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Responder con las transacciones formateadas
	response := &TransactionsResponse{
		Transactions: transactions,
	}

	return response, nil
}

func (s *store) GetUserValidReceiverMessageGemTransactionsSQLAndMongo(userID primitive.ObjectID) (*TransactionsResponse, error) {
	// SQL query para obtener las transacciones desde la tabla message_gems donde el user es el receiver y valid es 1
	query := `
        SELECT transaction_id, sender_id, receiver_id, gem_amount, created_at
        FROM message_gems
        WHERE receiver_id = ? AND valid = 1
    `

	// Convertimos el userID a string para usarlo en SQL
	userIDStr := userID.Hex()

	// Set a context timeout para la consulta SQL
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeoutDuration)
	defer cancel()

	// Ejecutar la consulta SQL para obtener las transacciones
	rows, err := s.db.QueryContext(ctx, query, userIDStr)
	if err != nil {
		return nil, fmt.Errorf("error querying message gems: %v", err)
	}
	defer rows.Close()

	var transactions []Transaction

	// Iterar sobre los resultados de SQL
	for rows.Next() {
		var messageGem struct {
			TransactionID int64
			SenderID      string
			ReceiverID    string
			GemAmount     int
			CreatedAt     time.Time
		}
		if err := rows.Scan(&messageGem.TransactionID, &messageGem.SenderID, &messageGem.ReceiverID, &messageGem.GemAmount, &messageGem.CreatedAt); err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}

		// Convertir el senderID a ObjectID de MongoDB para buscar el usuario
		senderObjID, err := primitive.ObjectIDFromHex(messageGem.SenderID)
		if err != nil {
			return nil, fmt.Errorf("error converting senderID to ObjectID: %v", err)
		}

		// Buscar el usuario en MongoDB
		var fromUser common.PublicUser
		userCollection := s.database.Collection("users")
		err = userCollection.FindOne(ctx, bson.M{"_id": senderObjID}).Decode(&fromUser)
		if err != nil {
			return nil, fmt.Errorf("error fetching user from MongoDB: %v", err)
		}

		// Crear la transacción y agregarla a la lista
		transaction := Transaction{
			ID:              messageGem.TransactionID,
			GemAmount:       messageGem.GemAmount,
			TransactionType: "chat_message", // Tipo de transacción es "message"
			Type:            "ingoing",      // Siempre es ingoing ya que el usuario es el receiver
			From:            fromUser,
			Pending:         false,                // Siempre false para message_gems
			CreatedAt:       messageGem.CreatedAt, // Tiempo desde SQL
		}

		transactions = append(transactions, transaction)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Responder con las transacciones formateadas
	response := &TransactionsResponse{
		Transactions: transactions,
	}

	return response, nil
}

func (s *store) GetUserValidReceiverGemDonationsSQLAndMongo(userID primitive.ObjectID) (*TransactionsResponse, error) {
	// SQL query to get the transactions from gem_donations where user is the receiver and valid = 1
	query := `
        SELECT transaction_id, sender_id, receiver_id, gem_amount, created_at
        FROM gem_donations
        WHERE receiver_id = ? AND valid = 1
    `

	// Convert the userID to a string for SQL
	userIDStr := userID.Hex()

	// Set a context timeout for the SQL query
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeoutDuration)
	defer cancel()

	// Execute the SQL query to get the valid receiver gem donations
	rows, err := s.db.QueryContext(ctx, query, userIDStr)
	if err != nil {
		return nil, fmt.Errorf("error querying gem donations: %v", err)
	}
	defer rows.Close()

	var transactions []Transaction

	// Iterate over the SQL results
	for rows.Next() {
		var gemDonation struct {
			TransactionID int64
			SenderID      string
			ReceiverID    string
			GemAmount     int
			CreatedAt     time.Time
		}
		if err := rows.Scan(&gemDonation.TransactionID, &gemDonation.SenderID, &gemDonation.ReceiverID, &gemDonation.GemAmount, &gemDonation.CreatedAt); err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}

		// Convert the senderID to ObjectID for MongoDB
		senderObjID, err := primitive.ObjectIDFromHex(gemDonation.SenderID)
		if err != nil {
			return nil, fmt.Errorf("error converting senderID to ObjectID: %v", err)
		}

		// Fetch the sender user details from MongoDB
		var fromUser common.PublicUser
		userCollection := s.database.Collection("users")
		err = userCollection.FindOne(ctx, bson.M{"_id": senderObjID}).Decode(&fromUser)
		if err != nil {
			return nil, fmt.Errorf("error fetching user from MongoDB: %v", err)
		}

		// Create the transaction and add it to the list
		transaction := Transaction{
			ID:              gemDonation.TransactionID,
			GemAmount:       gemDonation.GemAmount,
			TransactionType: "donation", // Transaction type is "donation"
			Type:            "ingoing",  // This is an ingoing transaction since the user is the receiver
			From:            fromUser,
			Pending:         false,                 // Always false for gem_donations
			CreatedAt:       gemDonation.CreatedAt, // Time from SQL
		}

		transactions = append(transactions, transaction)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Return the formatted transactions
	response := &TransactionsResponse{
		Transactions: transactions,
	}

	return response, nil
}

func (s *store) GetLast30DaysEarnings(userID primitive.ObjectID) (*pb.Earnings, error) {
	// Get the current time and the time 30 days ago
	now := time.Now()
	thirtyDaysAgo := now.AddDate(0, 0, -30)

	// Retrieve chat transactions and message transactions
	chatTransactions, err := s.GetUserValidReceiverChatTransactionsSQLAndMongo(userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching chat transactions: %v", err)
	}

	messageTransactions, err := s.GetUserValidReceiverMessageGemTransactionsSQLAndMongo(userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching message transactions: %v", err)
	}

	donationTransactions, err := s.GetUserValidReceiverGemDonationsSQLAndMongo(userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching donation transactions: %v", err)
	}

	// Combine all transactions
	allTransactions := append(chatTransactions.Transactions, messageTransactions.Transactions...)
	allTransactions = append(allTransactions, donationTransactions.Transactions...)

	// Map to hold day-wise total of valid transactions
	dayTotals := make(map[string]int)

	totalGems := 0

	// Iterate through all transactions
	for _, transaction := range allTransactions {
		// Only consider transactions from the last 30 days
		if transaction.CreatedAt.After(thirtyDaysAgo) && transaction.CreatedAt.Before(now) && !transaction.Pending {
			day := transaction.CreatedAt.Truncate(24 * time.Hour).Format("2006-01-02") // Normalize date as string (YYYY-MM-DD)
			dayTotals[day] += transaction.GemAmount
			totalGems += transaction.GemAmount
		}
	}

	// Create DayTransactions slice and ensure every day has an entry
	var dayTransactions []DayTransactions
	for day := thirtyDaysAgo; !day.After(now); day = day.AddDate(0, 0, 1) {
		dayStr := day.Format("2006-01-02") // Normalize date as string (YYYY-MM-DD)
		total, exists := dayTotals[dayStr]
		if !exists {
			total = 0
		}
		dayTransactions = append(dayTransactions, DayTransactions{
			Day:   day,
			Total: total,
		})
	}

	// Sort day transactions by date (newest to oldest)
	sort.Slice(dayTransactions, func(i, j int) bool {
		return dayTransactions[i].Day.After(dayTransactions[j].Day)
	})

	// Fill the Earnings struct
	earnings := Earnings{
		TotalValid:      totalGems,
		DayTransactions: dayTransactions,
	}

	// Convert Earnings struct to gRPC equivalent
	return s.ConvertToGRPCEarnings(earnings), nil
}

func (s *store) ConvertToGRPCEarnings(earnings Earnings) *pb.Earnings {
	dayTransactions := make([]*pb.DayTransactions, len(earnings.DayTransactions))

	for i, dt := range earnings.DayTransactions {
		dayTransactions[i] = &pb.DayTransactions{
			Day:   timestamppb.New(dt.Day),
			Total: int32(dt.Total),
		}
	}

	return &pb.Earnings{
		TotalValid:      int32(earnings.TotalValid),
		DayTransactions: dayTransactions,
	}
}

func (s *store) WipeUserData(userID primitive.ObjectID) error {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create the filter for the user by their ID
	filter := bson.M{"_id": userID}

	hash, err := common.GenerateRandomString(5)
	if err != nil {
		return err
	}

	// Create the update object to set all string fields to empty and reset counts
	update := bson.M{
		"$set": bson.M{
			"displayName":          "deleted-user#" + hash,
			"firstName":            "",
			"lastName":             "",
			"username":             "deleted-user#" + hash,
			"email":                "",
			"password":             "",
			"birthDate":            "",
			"avatarUri":            "",
			"bannerUri":            "",
			"verified":             "",
			"code":                 "",
			"description":          "",
			"expoToken":            []string{}, // Empty the expo token list
			"followers":            0,
			"following":            0,
			"subscribers":          0,
			"pendingNotifications": 0,
			"gems":                 0,
			"banned":               false,
			"deleted":              true,
		},
	}

	// Execute the update
	_, err = collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("error wiping user data: %v", err)
	}

	return nil
}

func (s *store) DeleteFollowsAndDecreaseCount(userID primitive.ObjectID) error {
	collection := s.database.Collection("user-follows")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create filters to find documents where userID is either the follower or following
	filter := bson.M{
		"$or": []bson.M{
			{"follower_id": userID},
			{"following_id": userID},
		},
	}

	// Find and delete matching documents
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return fmt.Errorf("error finding user follows: %v", err)
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var follow struct {
			FollowerID  primitive.ObjectID `bson:"follower_id"`
			FollowingID primitive.ObjectID `bson:"following_id"`
		}
		if err := cursor.Decode(&follow); err != nil {
			return fmt.Errorf("error decoding follow: %v", err)
		}

		// Decrease the follow count for the other user
		if follow.FollowerID == userID {
			s.DecreaseFollowingCount(follow.FollowingID)
		} else {
			s.DecreaseFollowerCount(follow.FollowerID)
		}
	}

	// Delete all matching follow documents
	_, err = collection.DeleteMany(ctx, filter)
	if err != nil {
		return fmt.Errorf("error deleting follow records: %v", err)
	}

	return nil
}

func (s *store) DecreaseFollowerCount(userID primitive.ObjectID) error {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Decrease the follower count
	_, err := collection.UpdateOne(ctx, bson.M{"_id": userID}, bson.M{"$inc": bson.M{"followers": -1}})
	return err
}

func (s *store) DecreaseFollowingCount(userID primitive.ObjectID) error {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Decrease the following count
	_, err := collection.UpdateOne(ctx, bson.M{"_id": userID}, bson.M{"$inc": bson.M{"following": -1}})
	return err
}

func (s *store) WipeSubscriptionTiers(userID primitive.ObjectID) error {
	collection := s.database.Collection("subscription-tiers")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Delete all subscription tiers where the user is the creator
	_, err := collection.DeleteMany(ctx, bson.M{"creator_id": userID})
	if err != nil {
		return fmt.Errorf("error deleting subscription tiers: %v", err)
	}

	return nil
}

func (s *store) DeleteUserNotifications(userID primitive.ObjectID) error {
	collection := s.database.Collection("notifications")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Delete all notifications where the destinationUser matches the userID
	_, err := collection.DeleteMany(ctx, bson.M{"destinationUser.id": userID})
	if err != nil {
		return fmt.Errorf("error deleting user notifications: %v", err)
	}

	return nil
}

func (s *store) HidePostsByUser(userID primitive.ObjectID) error {
	postCollection := s.database.Collection("posts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create filter to find posts by userID
	filter := bson.M{
		"user_id": userID,
	}

	// Create update to set "hidden" to true
	update := bson.M{
		"$set": bson.M{
			"hidden": true,
		},
	}

	// Update all posts that match the filter
	_, err := postCollection.UpdateMany(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("error hiding posts: %v", err)
	}

	return nil
}

func (s *store) UpdateUserDisplayName(displayName string, userID primitive.ObjectID) error {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create the filter for the user by their ID
	filter := bson.M{"_id": userID}

	// Create the update object to set both lastName and username
	update := bson.M{
		"$set": bson.M{
			"displayName": displayName,
			// Assuming the bson key for UserName is "username"
		},
	}

	// Execute the update
	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("error updating user details: %v", err)
	}

	return nil
}

func (s *store) GetInstagramFollowersCountAndUsername(accessToken string) (int, string, error) {
	// Construct the URL with the access token
	url := fmt.Sprintf("https://graph.instagram.com/me?fields=followers_count,username&access_token=%s", accessToken)

	// Make the GET request
	resp, err := http.Get(url)
	if err != nil {
		return 0, "", fmt.Errorf("error making the request: %v", err)
	}
	defer resp.Body.Close()

	// Check for non-200 response codes
	if resp.StatusCode != http.StatusOK {
		return 0, "", fmt.Errorf("received non-200 response code: %d", resp.StatusCode)
	}

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, "", fmt.Errorf("error reading response body: %v", err)
	}

	// Parse the response JSON
	var igResponse InstagramResponse
	err = json.Unmarshal(body, &igResponse)
	if err != nil {
		return 0, "", fmt.Errorf("error parsing JSON: %v", err)
	}

	// Return the followers_count and username
	return igResponse.FollowersCount, igResponse.Username, nil
}

func (s *store) AddCreatorRole(userID primitive.ObjectID) error {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Define the filter to locate the document to update
	filter := bson.M{"_id": userID}

	update := bson.M{
		"$addToSet": bson.M{
			"roles": "creator",
		},
	}

	// Perform the update operation
	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}

func (s *store) UpsertPendingCreator(userID primitive.ObjectID, followersCount int, userName string) error {
	collection := s.database.Collection("waiting-list")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Step 1: Create filter to check if the pending creator already exists
	filter := bson.M{"user_id": userID}

	// Step 2: Create the update object
	update := bson.M{
		"$set": bson.M{
			"followers_count": followersCount,
			"username":        userName,
			"createdAt":       time.Now(),
		},
	}

	// Step 3: Perform upsert operation
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("error upserting pending creator: %v", err)
	}

	return nil
}

func (s *store) GetUserValidCreatorSubscriptionPurchasesSQLAndMongo(userID primitive.ObjectID) (*TransactionsResponse, error) {
	// SQL query to get the transactions from the sub_purchases table
	query := `
        SELECT transaction_id, buyer_id, creator_id, tier, created_at
        FROM sub_purchases
        WHERE creator_id = ? AND valid = 1
    `

	// Convert the userID to a string for use in the SQL query
	userIDStr := userID.Hex()

	// Set a context timeout for the SQL query
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeoutDuration)
	defer cancel()

	// Execute the SQL query to get the transactions
	rows, err := s.db.QueryContext(ctx, query, userIDStr)
	if err != nil {
		return nil, fmt.Errorf("error querying subscription purchases: %v", err)
	}
	defer rows.Close()

	// Define the map for tier to gemAmount conversion
	tierToGemAmount := map[int]int{
		1: 5,
		2: 10,
		3: 25,
		4: 50,
		5: 100,
	}

	var transactions []Transaction

	// Iterate over the SQL results
	for rows.Next() {
		var subPurchase struct {
			TransactionID int64
			BuyerID       string
			CreatorID     string
			Tier          int
			CreatedAt     time.Time
		}
		if err := rows.Scan(&subPurchase.TransactionID, &subPurchase.BuyerID, &subPurchase.CreatorID, &subPurchase.Tier, &subPurchase.CreatedAt); err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}

		// Convert the buyerID to ObjectID to fetch the user from MongoDB
		buyerObjID, err := primitive.ObjectIDFromHex(subPurchase.BuyerID)
		if err != nil {
			return nil, fmt.Errorf("error converting buyerID to ObjectID: %v", err)
		}

		// Fetch the buyer's information from MongoDB
		var buyerUser common.PublicUser
		userCollection := s.database.Collection("users")
		err = userCollection.FindOne(ctx, bson.M{"_id": buyerObjID}).Decode(&buyerUser)
		if err != nil {
			return nil, fmt.Errorf("error fetching buyer from MongoDB: %v", err)
		}

		// Get the gemAmount from the tierToGemAmount map
		gemAmount, exists := tierToGemAmount[subPurchase.Tier]
		if !exists {
			return nil, fmt.Errorf("invalid tier value: %d", subPurchase.Tier)
		}

		// Create the transaction and add it to the list
		transaction := Transaction{
			ID:              subPurchase.TransactionID,
			GemAmount:       gemAmount,               // Gem amount is based on the tier
			TransactionType: "subscription_purchase", // Transaction type is "subscription purchase"
			Type:            "ingoing",               // Type is ingoing for the creator
			From:            buyerUser,               // The buyer who made the purchase
			Pending:         false,                   // Assuming no pending state for purchases
			CreatedAt:       subPurchase.CreatedAt,   // The time of the purchase
		}

		transactions = append(transactions, transaction)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Return the formatted transactions
	response := &TransactionsResponse{
		Transactions: transactions,
	}

	return response, nil
}

func (s *store) GetLast30DaysSubEarnings(userID primitive.ObjectID) (*pb.Earnings, error) {
	// Get the current time and the time 30 days ago
	now := time.Now()
	thirtyDaysAgo := now.AddDate(0, 0, -30)

	// Retrieve subscription transactions for the user where they are the creator
	subscriptionTransactions, err := s.GetUserValidCreatorSubscriptionPurchasesSQLAndMongo(userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching subscription transactions: %v", err)
	}

	// Map to hold day-wise total of valid transactions
	dayTotals := make(map[string]int)

	totalGems := 0

	// Iterate through all subscription transactions
	for _, transaction := range subscriptionTransactions.Transactions {
		// Only consider transactions from the last 30 days
		if transaction.CreatedAt.After(thirtyDaysAgo) && transaction.CreatedAt.Before(now) && !transaction.Pending {
			day := transaction.CreatedAt.Truncate(24 * time.Hour).Format("2006-01-02") // Normalize date as string (YYYY-MM-DD)
			dayTotals[day] += transaction.GemAmount
			totalGems += transaction.GemAmount
		}
	}

	// Create DayTransactions slice and ensure every day has an entry
	var dayTransactions []DayTransactions
	for day := thirtyDaysAgo; !day.After(now); day = day.AddDate(0, 0, 1) {
		dayStr := day.Format("2006-01-02") // Normalize date as string (YYYY-MM-DD)
		total, exists := dayTotals[dayStr]
		if !exists {
			total = 0
		}
		dayTransactions = append(dayTransactions, DayTransactions{
			Day:   day,
			Total: total,
		})
	}

	// Sort day transactions by date (newest to oldest)
	sort.Slice(dayTransactions, func(i, j int) bool {
		return dayTransactions[i].Day.After(dayTransactions[j].Day)
	})

	// Fill the Earnings struct
	earnings := Earnings{
		TotalValid:      totalGems,
		DayTransactions: dayTransactions,
	}

	// Convert Earnings struct to gRPC equivalent
	return s.ConvertToGRPCEarnings(earnings), nil
}

func (s *store) RetrieveUserSubscriptions(userID primitive.ObjectID, page int, pageSize int) (*pb.RetrieveSubResponse, error) {
	collection := s.database.Collection("user-subscriptions")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Define the filter to find active, valid subscriptions for the given userID
	filter := bson.M{
		"user_id": userID,
		"pending": false,
		"active":  true,
	}

	// Calculate how many documents to skip based on the page number
	skip := (page - 1) * pageSize

	// Define the FindOptions with pagination (skip and limit)
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(pageSize))

	// Perform the MongoDB query with pagination
	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("error finding subscriptions: %v", err)
	}
	defer cursor.Close(ctx)

	// Create a slice to hold the Sub protobuf messages
	var subs []*pb.Sub

	// Iterate over the cursor and decode each subscription
	for cursor.Next(ctx) {
		var result common.Subscription
		if err := cursor.Decode(&result); err != nil {
			return nil, fmt.Errorf("error decoding subscription: %v", err)
		}

		// Fetch creator's public information from MongoDB
		creatorUser, err := s.GetUserByObjectID(result.CreatorID)
		if err != nil {
			return nil, fmt.Errorf("error fetching creator user details: %v", err)
		}

		publicUser := convertUserToPublicUser(*creatorUser)

		// Convert PublicUser Go struct to protobuf
		creatorProto := convertPublicUserToProto(publicUser)

		// Convert the result to the Sub protobuf message
		subProto := &pb.Sub{
			SubId:   result.ID.Hex(), // Convert ObjectID to string
			Creator: creatorProto,
			Tier:    int32(result.Tier), // Tier as int32
		}

		// Append the Sub message to the list
		subs = append(subs, subProto)
	}

	// Check if there was an error with the cursor iteration
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("error during cursor iteration: %v", err)
	}

	// Create the final response
	response := &pb.RetrieveSubResponse{
		Subscriptions: subs,
	}

	return response, nil
}

func (s *store) IsEmailInWaitingCreators(email string) (bool, error) {
	waitingCreatorsCollection := s.database.Collection("waiting-creators")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Step 1: Check if the given email is in the "waiting-creators" list
	filter := bson.M{
		"emails": bson.M{"$in": []string{email}}, // Check if the email exists in the array
	}

	// Try to find the document with the email
	err := waitingCreatorsCollection.FindOne(ctx, filter).Err()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			// Email is not in the waiting-creators list
			return false, nil
		}
		// Some other error occurred
		return false, fmt.Errorf("error checking email in waiting creators list: %v", err)
	}

	// Email found in the waiting-creators list
	return true, nil
}

func (s *store) CreateTopCreatorsList() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Step 1: Get the impressions in the last hour from the post-impression collection
	collection := s.database.Collection("post-impression")
	lastHour := time.Now().Add(-1 * time.Hour)

	// Filter for impressions created in the last hour
	impressionFilter := bson.M{
		"createdAt": bson.M{"$gte": lastHour},
	}

	// Find the impressions created within the last hour
	opts := options.Find()
	cursor, err := collection.Find(ctx, impressionFilter, opts)
	if err != nil {
		return fmt.Errorf("error finding post impressions: %v", err)
	}
	defer cursor.Close(ctx)

	// Step 2: Track impressions per user
	userImpressions := make(map[primitive.ObjectID]int)

	for cursor.Next(ctx) {
		var impression struct {
			UserID primitive.ObjectID `bson:"user_id"`
			PostID primitive.ObjectID `bson:"post_id"`
		}
		if err := cursor.Decode(&impression); err != nil {
			return fmt.Errorf("error decoding impression: %v", err)
		}

		// Get the user_id from the post associated with the impression
		post, err := s.GetPostByID(impression.PostID)
		if err != nil {
			log.Printf("error getting post %s: %v", impression.PostID.Hex(), err)
			continue
		}

		// Increment the impression count for the post owner (user_id)
		userImpressions[post.User_id]++
	}

	// Step 3: Sort users by impressions and take the top 5
	type UserImpression struct {
		UserID     primitive.ObjectID
		Impression int
	}

	var sortedUsers []UserImpression
	for userID, impressions := range userImpressions {
		sortedUsers = append(sortedUsers, UserImpression{UserID: userID, Impression: impressions})
	}

	// Sort by number of impressions in descending order
	sort.Slice(sortedUsers, func(i, j int) bool {
		return sortedUsers[i].Impression > sortedUsers[j].Impression
	})

	// Get the top 5 users
	topUsers := sortedUsers
	if len(sortedUsers) > 5 {
		topUsers = sortedUsers[:5]
	}

	// Step 4: Fetch the PublicUser data for each top creator
	var topCreators []*pb.PublicUser
	alreadyAddedUsers := make(map[primitive.ObjectID]bool) // Track users already added to prevent duplicates
	for _, user := range topUsers {
		creator, err := s.GetUserByObjectID(user.UserID)
		if err != nil {
			log.Printf("error getting user %s: %v", user.UserID.Hex(), err)
			continue
		}
		publicUser := s.ConvertToPublicUserProto(creator)
		topCreators = append(topCreators, publicUser)
		alreadyAddedUsers[user.UserID] = true
	}

	// Step 5: If fewer than 10 creators, fill with creators with the most general impressions
	if len(topCreators) < 10 {
		remainingSlots := 10 - len(topCreators)

		// Fetch the creators with the most general impressions (regardless of time)
		generalTopCreators, err := s.GetTopCreatorsByGeneralImpressions(remainingSlots, alreadyAddedUsers)
		if err != nil {
			log.Printf("error fetching general top creators: %v", err)
			return err
		}

		// Add these to the top creators list
		topCreators = append(topCreators, generalTopCreators...)
	}

	// Ensure that the final list does not exceed 10 creators
	if len(topCreators) > 10 {
		topCreators = topCreators[:10]
	}

	// Step 6: Store the list in gRPC format
	err = s.StoreTopCreators(topCreators)
	if err != nil {
		return fmt.Errorf("error storing top creators: %v", err)
	}

	log.Println("Top creators list created successfully")
	return nil
}

func (s *store) GetTopCreatorsByGeneralImpressions(limit int, alreadyAddedUsers map[primitive.ObjectID]bool) ([]*pb.PublicUser, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	collection := s.database.Collection("post-impression")

	// Aggregate impressions by user
	pipeline := mongo.Pipeline{
		{{
			Key: "$group",
			Value: bson.D{
				{Key: "_id", Value: "$user_id"},
				{Key: "totalImpressions", Value: bson.D{{Key: "$sum", Value: 1}}},
			},
		}},
		{{
			Key:   "$sort",
			Value: bson.D{{Key: "totalImpressions", Value: -1}},
		}},
		{{
			Key:   "$limit",
			Value: limit,
		}},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("error aggregating impressions: %v", err)
	}
	defer cursor.Close(ctx)

	var users []struct {
		UserID           primitive.ObjectID `bson:"_id"`
		TotalImpressions int                `bson:"totalImpressions"`
	}

	if err := cursor.All(ctx, &users); err != nil {
		return nil, fmt.Errorf("error decoding impressions: %v", err)
	}

	// Fetch the PublicUser data for each creator
	var topCreators []*pb.PublicUser
	for _, user := range users {
		// Skip users who are already added
		if alreadyAddedUsers[user.UserID] {
			continue
		}
		creator, err := s.GetUserByObjectID(user.UserID)
		if err != nil {
			log.Printf("error getting user %s: %v", user.UserID.Hex(), err)
			continue
		}
		publicUser := s.ConvertToPublicUserProto(creator)
		topCreators = append(topCreators, publicUser)
	}

	return topCreators, nil
}

func (s *store) StoreTopCreators(creators []*pb.PublicUser) error {
	// Store the list of top creators in the global variable
	topCreatorsList = creators
	return nil
}

func (s *store) GetPostByID(postID primitive.ObjectID) (*common.Post, error) {
	collection := s.database.Collection("posts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var post common.Post
	err := collection.FindOne(ctx, bson.M{"_id": postID}).Decode(&post)
	if err != nil {
		return nil, err
	}

	return &post, nil
}

// Function to convert a common.User struct to a gRPC PublicUser message
func (s *store) ConvertToPublicUserProto(user *common.User) *pb.PublicUser {
	return &pb.PublicUser{
		Id:          user.ID.Hex(),
		DisplayName: user.DisplayName,
		Username:    user.UserName,
		CreatedAt:   timestamppb.New(user.CreatedAt),
		AvatarUri:   user.Avatar_uri,
		BannerUri:   user.Banner_uri,
		Description: user.Description,
		Followers:   int64(user.Followers),
		Following:   int64(user.Following),
		Subscribers: int64(user.Subscribers),
	}
}

func (s *store) UpdateTopCreators() error {
	// Fetch top creators from the database based on post impressions
	err := s.CreateTopCreatorsList()
	if err != nil {
		return fmt.Errorf("error fetching top creators: %v", err)
	}

	return nil
}

func (s *store) IsCreator(user common.User) bool {
	for _, role := range user.Roles {
		if role == "creator" {
			return true
		}
	}
	return false
}
