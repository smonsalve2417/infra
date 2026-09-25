package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"mime/multipart"
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

	// Define the update operation to add the new token to the expoToken array
	update := bson.M{
		"$push": bson.M{
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
	userID, err := primitive.ObjectIDFromHex(grpcMsg.UserXid)
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
		UserXid:          userID,
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
	filter := bson.M{"username": bson.M{"$regex": username, "$options": "i"}}

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
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		Username:    user.UserName,
		CreatedAt:   timestamppb.New(user.CreatedAt),
		AvatarUri:   user.Avatar_uri,
		BannerUri:   user.Banner_uri,
		Description: user.Description,
		Followers:   int64(user.Followers),
		Following:   int64(user.Following),
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
			FirstName:   t.From.FirstName,
			LastName:    t.From.LastName,
			Username:    t.From.UserName,
			CreatedAt:   timestamppb.New(t.From.CreatedAt),
			Followers:   int64(t.From.Followers),
			Following:   int64(t.From.Following),
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
		Tier:      int(pbTier.Tier),  // Convert int32 to int
		Price:     int(pbTier.Price), // Convert int32 to int
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
			"price":    tier.Price,
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

	if result.ModifiedCount == 0 {
		return fmt.Errorf("subscription tier found, but no changes were made")
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
		Tier:      int32(tier.Tier),  // Convert int to int32
		Price:     int32(tier.Price), // Convert int to int32
	}

	return pbTier, nil
}

func (s *store) GetSubscriptionTiers(creatorID primitive.ObjectID) ([]common.SubscriptionTiers, error) {
	collection := s.database.Collection("subscription-tiers")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Filter by creatorID
	filter := bson.M{
		"creator_id": creatorID,
	}

	// Sort by tier in ascending order
	opts := options.Find().SetSort(bson.D{{Key: "tier", Value: 1}})

	// Execute the query
	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("error fetching subscription tiers: %v", err)
	}
	defer cursor.Close(ctx)

	var tiers []common.SubscriptionTiers
	if err = cursor.All(ctx, &tiers); err != nil {
		return nil, fmt.Errorf("error decoding subscription tiers: %v", err)
	}

	return tiers, nil
}

func (s *store) ConvertSubscriptionTierToPB(tier *common.SubscriptionTiers) *pb.SubscriptionTiers {
	return &pb.SubscriptionTiers{
		XId:       tier.ID.Hex(),
		CreatorId: tier.CreatorID.Hex(),
		Name:      tier.Name,
		Benefits:  tier.Benefits,
		Tier:      int32(tier.Tier),  // Convert int to int32
		Price:     int32(tier.Price), // Convert int to int32
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

func (s *store) GetSubscriptionTiersAsPB(creatorID primitive.ObjectID) ([]*pb.SubscriptionTiers, error) {
	tiers, err := s.GetSubscriptionTiers(creatorID)
	if err != nil {
		return nil, err
	}

	// Convert the list to gRPC format
	return s.ConvertSubscriptionTiersListToPB(tiers), nil
}
