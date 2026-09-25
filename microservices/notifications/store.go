package main

import (
	"context"
	"fmt"
	"time"

	common "github.com/Eskiwi-Organization/infra/commons"
	pb "github.com/Eskiwi-Organization/infra/commons/api"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type store struct {
	client   *mongo.Client
	database *mongo.Database
}

func NewStore(client *mongo.Client) *store {
	return &store{client: client, database: client.Database(mongoDatabaseName)}
}
func (s *store) Create(context.Context) error {
	return nil
}

func (s *store) UpdateUserCode(email string, newCode string) error {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Perform the update operation
	update := bson.M{
		"$set": bson.M{
			"code": newCode,
		},
	}
	_, err := collection.UpdateOne(ctx, bson.M{"email": email}, update)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) GetFollowersByUserID(userID primitive.ObjectID) ([]primitive.ObjectID, error) {
	collection := s.database.Collection("user-follows")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a filter to find all documents where the following_id matches userID
	filter := bson.M{"following_id": userID}
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("error querying followers: %v", err)
	}
	defer cursor.Close(ctx)

	var followers []primitive.ObjectID
	for cursor.Next(ctx) {
		var result struct {
			FollowerID primitive.ObjectID `bson:"follower_id"`
		}
		if err := cursor.Decode(&result); err != nil {
			return nil, fmt.Errorf("error decoding follower: %v", err)
		}
		// Convert ObjectID to string and append to the followers list
		followers = append(followers, result.FollowerID)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %v", err)
	}

	return followers, nil
}

func (s *store) CreatePostNotifications(postID primitive.ObjectID, userID primitive.ObjectID, notificationType string) error {
	// Step 1: Retrieve the list of followers for the given userID
	followers, err := s.GetFollowersByUserID(userID)
	if err != nil {
		return fmt.Errorf("error getting followers: %v", err)
	}

	// Step 2: Retrieve post details
	post, err := s.GetPostByID(postID)
	if err != nil {
		return fmt.Errorf("error getting post: %v", err)
	}

	// Retrieve user details
	userCreatingPost, err := s.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("error getting user: %v", err)
	}

	// Prepare notifications for each follower
	notifications := []interface{}{}
	for _, followerID := range followers {

		pushToken, err := s.GetPushTokenByUserID(followerID)
		if err != nil {
			return fmt.Errorf("error getting push token for follower: %v", err)
		}

		notification := bson.M{
			"user": bson.M{
				"_id":         userCreatingPost.ID,          // Store the ObjectID directly
				"firstName":   userCreatingPost.FirstName,   // Store user's first name
				"lastName":    userCreatingPost.LastName,    // Store user's last name
				"username":    userCreatingPost.UserName,    // Store user's username
				"avatarUri":   userCreatingPost.Avatar_uri,  // Store user's avatar URI
				"bannerUri":   userCreatingPost.Banner_uri,  // Store user's banner URI
				"description": userCreatingPost.Description, // Store user's description
				"createdAt":   userCreatingPost.CreatedAt,   // Store user's creation time
				"followers":   userCreatingPost.Followers,
				"following":   userCreatingPost.Following,
			},
			"post": bson.M{
				"_id":         post.ID,          // Store the ObjectID directly
				"title":       post.Title,       // Store the post title
				"description": post.Description, // Store the post description
				"user_id":     post.User_id,     // Store the ID of the user who created the post
				"content":     post.Content,     // Store the post content
				"tags":        post.Tags,        // Store the post tags
				"gems":        post.Gems,        // Store the number of gems
				"likes":       post.Likes,       // Store the number of likes
				"comments":    post.Comments,    // Store the number of comments
				"shares":      post.Shares,      // Store the number of shares
				"saves":       post.Saves,       // Store the number of saves
				"createdAt":   post.CreatedAt,   // Store the post creation time
				"updatedAt":   post.UpdatedAt,   // Store the post update time
			},
			"createdAt": time.Now(), // Notification creation time
			"destinationUser": bson.M{
				"id": followerID, // Store the ObjectID directly
				// Initially set as unread
			},
			"isRead": false,
			"type":   notificationType, // Notification type: "Follow", "Like", "Comment", "Message", "newPost"
		}

		err = s.IncrementPendingNotifications(followerID)
		if err != nil {
			return fmt.Errorf("error updating notification counter: %v", err)
		}

		notifications = append(notifications, notification)

		//Se debe ver si tiene la noti activada
		settings, err := s.GetUserNotificationSettings(followerID)
		if err != nil {
			//return fmt.Errorf("error searching user settings: %v", err)
		}

		if settings.NewPost {
			err = sendPushNotification(pushToken, userCreatingPost.UserName, "Ha subido un nuevo post!")
			if err != nil {
				//return fmt.Errorf("error sending push notification: %v", err)
			}
		}

	}

	// Step 3: Insert notifications into the "post-notifications" collection
	collection := s.database.Collection("notifications")
	_, err = collection.InsertMany(context.Background(), notifications)
	if err != nil {
		return fmt.Errorf("error inserting notifications: %v", err)
	}

	return nil

}

// Example GetUserByID function
func (s *store) GetUserByID(userID primitive.ObjectID) (*common.PublicUser, error) {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": userID}
	var user common.PublicUser
	err := collection.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("user not found: id %s", userID.Hex())
		}
		return nil, err
	}

	return &user, nil
}

func (s *store) GetPostByID(postID primitive.ObjectID) (*common.Post, error) {
	collection := s.database.Collection("posts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": postID}
	var post common.Post
	err := collection.FindOne(ctx, filter).Decode(&post)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("post not found: id %s", postID.Hex())
		}
		return nil, err
	}

	return &post, nil
}

func (s *store) GetCommentByID(commentID primitive.ObjectID) (*common.Comment, error) {
	collection := s.database.Collection("posts-comment")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": commentID}
	var comment common.Comment
	err := collection.FindOne(ctx, filter).Decode(&comment)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("comment not found: id %s", commentID.Hex())
		}
		return nil, err
	}

	return &comment, nil
}

func (s *store) GetChatByID(chatID primitive.ObjectID) (*Chat, error) {
	collection := s.database.Collection("chats")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": chatID}
	var chat Chat
	err := collection.FindOne(ctx, filter).Decode(&chat)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("chat not found: id %s", chatID.Hex())
		}
		return nil, err
	}

	return &chat, nil
}

func (s *store) GetPushTokenByUserID(userID primitive.ObjectID) ([]string, error) {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": userID}
	var userPushToken UserPushToken
	err := collection.FindOne(ctx, filter).Decode(&userPushToken)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("push token not found for user: id %s", userID.Hex())
		}
		return nil, err
	}

	return userPushToken.PushToken, nil
}

func (s *store) GetLatestsNotifications(page int, pageSize int, userID primitive.ObjectID) ([]NotificationMongo, error) {
	notificationCollection := s.database.Collection("notifications")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"destinationUser.id": userID}

	opts := options.Find()
	opts.SetSort(bson.D{primitive.E{Key: "createdAt", Value: -1}})
	opts.SetSkip(int64((page - 1) * pageSize))
	opts.SetLimit(int64(pageSize))

	cursor, err := notificationCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var Notifications []NotificationMongo
	if err = cursor.All(ctx, &Notifications); err != nil {
		return nil, err
	}

	postCollection := s.database.Collection("posts")
	userCollection := s.database.Collection("users")
	// Iterate over notifications to fetch and update post info
	for i, notif := range Notifications {
		// Fetch and update post info if necessary
		if notif.Type == "Like" || notif.Type == "Comment" || notif.Type == "newPost" {
			postID := notif.Post.ID
			var post common.Post
			if err := postCollection.FindOne(ctx, bson.M{"_id": postID}).Decode(&post); err != nil {
				return nil, err // Handle error properly depending on your use case
			}
			Notifications[i].Post = post // Update the post in the notification
		}

		// Fetch and update user info
		userID := notif.User.ID
		var user common.User
		if err := userCollection.FindOne(ctx, bson.M{"_id": userID}).Decode(&user); err != nil {
			return nil, err // Handle error properly depending on your use case
		}
		Notifications[i].User = user // Update the user in the notification
	}

	// Prepare the bulk write operations to update unread notifications
	var bulkOps []mongo.WriteModel
	for _, notification := range Notifications {
		if !notification.IsRead { // Assuming "IsRead" is a field in your NotificationMongo struct
			updateFilter := bson.M{"_id": notification.ID}
			update := bson.M{"$set": bson.M{"isRead": true}}
			bulkOps = append(bulkOps, mongo.NewUpdateOneModel().SetFilter(updateFilter).SetUpdate(update))
		}
	}

	// Execute the bulk write operations to mark notifications as read
	if len(bulkOps) > 0 {
		_, err := notificationCollection.BulkWrite(ctx, bulkOps)
		if err != nil {
			return nil, err
		}
	}

	return Notifications, nil
}

func ConvertToGrpcNotification(notificationMongo NotificationMongo) *pb.Notification {
	return &pb.Notification{
		Id: notificationMongo.ID.Hex(),
		User: &pb.PublicUser{
			Id:          notificationMongo.User.ID.Hex(), // Convert ObjectID to string
			FirstName:   notificationMongo.User.FirstName,
			LastName:    notificationMongo.User.LastName,
			Username:    notificationMongo.User.UserName,
			AvatarUri:   notificationMongo.User.Avatar_uri,
			BannerUri:   notificationMongo.User.Banner_uri,
			Description: notificationMongo.User.Description,
			CreatedAt:   timestamppb.New(notificationMongo.User.CreatedAt), // Format time to ISO 8601 string
			Followers:   int64(notificationMongo.User.Followers),
			Following:   int64(notificationMongo.User.Following),
		},
		Post: &pb.Post{
			Id:          notificationMongo.Post.ID.Hex(), // Convert ObjectID to string
			Title:       notificationMongo.Post.Title,
			Description: notificationMongo.Post.Description,
			UserXid:     notificationMongo.Post.User_id.Hex(), // Convert ObjectID to string
			Content:     notificationMongo.Post.Content,
			Tags:        notificationMongo.Post.Tags,
			Gems:        int32(notificationMongo.Post.Gems),
			Likes:       int32(notificationMongo.Post.Likes),
			Comments:    int32(notificationMongo.Post.Comments),
			Shares:      int32(notificationMongo.Post.Shares),
			Saves:       int32(notificationMongo.Post.Saves),
			Created_At:  timestamppb.New(notificationMongo.Post.CreatedAt), // Format time to ISO 8601 string
			Updated_At:  timestamppb.New(notificationMongo.Post.UpdatedAt), // Format time to ISO 8601 string
		},
		Comment: &pb.Comment{
			Id:         notificationMongo.Comment.ID.Hex(),
			UserId:     notificationMongo.Comment.UserID.Hex(),
			PostId:     notificationMongo.Comment.PostID.Hex(),
			Text:       notificationMongo.Comment.Text,
			Likes:      int32(notificationMongo.Comment.Likes),
			Replies:    int32(notificationMongo.Comment.Replies),
			Created_At: timestamppb.New(notificationMongo.Comment.CreatedAt),
			Updated_At: timestamppb.New(notificationMongo.Comment.UpdatedAt),
		},
		CreatedAt:         timestamppb.New(notificationMongo.CreatedAt), // Format time to ISO 8601 string
		DestinationUserId: notificationMongo.DestinationUser.ID.Hex(),   // Convert ObjectID to string
		IsRead:            notificationMongo.IsRead,
		Type:              notificationMongo.Type,
	}
}

func (s *store) CreatePostLikeNotifications(postID primitive.ObjectID, userID primitive.ObjectID, notificationType string) error {

	// Step 2: Retrieve post details
	post, err := s.GetPostByID(postID)
	if err != nil {
		return fmt.Errorf("error getting post: %v", err)
	}

	// Retrieve user of post details
	postOwnerUser, err := s.GetUserByID(post.User_id)
	if err != nil {
		return fmt.Errorf("error getting user: %v", err)
	}

	// Retrieve user of post details
	likingUser, err := s.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("error getting user: %v", err)
	}

	// Prepare notifications for each follower
	notifications := []interface{}{}

	pushToken, err := s.GetPushTokenByUserID(postOwnerUser.ID)
	if err != nil {
		return fmt.Errorf("error getting push token for follower: %v", err)
	}

	notification := bson.M{
		"user": bson.M{
			"_id":         likingUser.ID,          // Store the ObjectID directly
			"firstName":   likingUser.FirstName,   // Store user's first name
			"lastName":    likingUser.LastName,    // Store user's last name
			"username":    likingUser.UserName,    // Store user's username
			"avatarUri":   likingUser.Avatar_uri,  // Store user's avatar URI
			"bannerUri":   likingUser.Banner_uri,  // Store user's banner URI
			"description": likingUser.Description, // Store user's description
			"createdAt":   likingUser.CreatedAt,   // Store user's creation time
			"followers":   likingUser.Followers,
			"following":   likingUser.Following,
		},
		"post": bson.M{
			"_id":         post.ID,          // Store the ObjectID directly
			"title":       post.Title,       // Store the post title
			"description": post.Description, // Store the post description
			"user_id":     post.User_id,     // Store the ID of the user who created the post
			"content":     post.Content,     // Store the post content
			"tags":        post.Tags,        // Store the post tags
			"gems":        post.Gems,        // Store the number of gems
			"likes":       post.Likes,       // Store the number of likes
			"comments":    post.Comments,    // Store the number of comments
			"shares":      post.Shares,      // Store the number of shares
			"saves":       post.Saves,       // Store the number of saves
			"createdAt":   post.CreatedAt,   // Store the post creation time
			"updatedAt":   post.UpdatedAt,   // Store the post update time
		},
		"createdAt": time.Now(), // Notification creation time
		"destinationUser": bson.M{
			"id": post.User_id, // Store the ObjectID directly
			// Initially set as unread
		},
		"isRead": false,
		"type":   notificationType, // Notification type: "Follow", "Like", "Comment", "Message", "newPost"
	}

	notifications = append(notifications, notification)

	err = s.IncrementPendingNotifications(post.User_id)
	if err != nil {
		return fmt.Errorf("error updating notification counter: %v", err)
	}

	// Step 3: Insert notifications into the "post-notifications" collection
	collection := s.database.Collection("notifications")
	_, err = collection.InsertMany(context.Background(), notifications)
	if err != nil {
		return fmt.Errorf("error inserting notifications: %v", err)
	}

	settings, err := s.GetUserNotificationSettings(post.User_id)
	if err != nil {
		return fmt.Errorf("error searching user settings: %v", err)
	}

	if settings.NewLike && userID != post.User_id {
		err = sendPushNotification(pushToken, likingUser.UserName, "Le ha dado like a tu publicación!")
		if err != nil {
			return fmt.Errorf("error sending push notification: %v", err)
		}
	}

	return nil
}

func (s *store) IncrementPendingNotifications(userID primitive.ObjectID) error {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Define the filter to find the user by ID
	filter := bson.M{"_id": userID}

	// Define the update to increment the pendingNotifications field by 1
	update := bson.M{"$inc": bson.M{"pendingNotifications": 1}}

	// Perform the update operation
	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update user: %v", err)
	}

	// Check if any document was modified
	if result.MatchedCount == 0 {
		return fmt.Errorf("user not found: id %s", userID.Hex())
	}

	return nil
}

func (s *store) CreatePostCommentNotifications(postID primitive.ObjectID, commentID primitive.ObjectID, userID primitive.ObjectID, notificationType string) error {

	// Step 2: Retrieve post details
	post, err := s.GetPostByID(postID)
	if err != nil {
		return fmt.Errorf("error getting post: %v", err)
	}

	// Retrieve user of post details
	//user, err := s.GetUserByID(post.User_id)
	//if err != nil {
	//	return fmt.Errorf("error getting user: %v", err)
	//}

	comment, err := s.GetCommentByID(commentID)
	if err != nil {
		return fmt.Errorf("error getting comment: %v", err)
	}

	// Retrieve user of post details
	commentingUser, err := s.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("error getting user: %v", err)
	}

	// Prepare notifications for each follower
	notifications := []interface{}{}

	pushToken, err := s.GetPushTokenByUserID(post.User_id)
	if err != nil {
		return fmt.Errorf("error getting push token for follower: %v", err)
	}

	notification := bson.M{
		"user": bson.M{
			"_id":         commentingUser.ID,          // Store the ObjectID directly
			"firstName":   commentingUser.FirstName,   // Store user's first name
			"lastName":    commentingUser.LastName,    // Store user's last name
			"username":    commentingUser.UserName,    // Store user's username
			"avatarUri":   commentingUser.Avatar_uri,  // Store user's avatar URI
			"bannerUri":   commentingUser.Banner_uri,  // Store user's banner URI
			"description": commentingUser.Description, // Store user's description
			"createdAt":   commentingUser.CreatedAt,   // Store user's creation time
			"followers":   commentingUser.Followers,
			"following":   commentingUser.Following,
		},
		"post": bson.M{
			"_id":         post.ID,          // Store the ObjectID directly
			"title":       post.Title,       // Store the post title
			"description": post.Description, // Store the post description
			"user_id":     post.User_id,     // Store the ID of the user who created the post
			"content":     post.Content,     // Store the post content
			"tags":        post.Tags,        // Store the post tags
			"gems":        post.Gems,        // Store the number of gems
			"likes":       post.Likes,       // Store the number of likes
			"comments":    post.Comments,    // Store the number of comments
			"shares":      post.Shares,      // Store the number of shares
			"saves":       post.Saves,       // Store the number of saves
			"createdAt":   post.CreatedAt,   // Store the post creation time
			"updatedAt":   post.UpdatedAt,   // Store the post update time
		},
		"comment": bson.M{
			"_id":       comment.ID,
			"user_id":   comment.UserID,
			"post_id":   comment.PostID,
			"text":      comment.Text,
			"likes":     comment.Likes,
			"replies":   comment.Replies,
			"createdAt": comment.CreatedAt,
			"updatedAt": comment.UpdatedAt,
		},
		"createdAt": time.Now(), // Notification creation time
		"destinationUser": bson.M{
			"id": post.User_id, // Store the ObjectID directly
			// Initially set as unread
		},
		"isRead": false,
		"type":   notificationType, // Notification type: "Follow", "Like", "Comment", "Message", "newPost"
	}

	notifications = append(notifications, notification)

	err = s.IncrementPendingNotifications(post.User_id)
	if err != nil {
		return fmt.Errorf("error updating notification counter: %v", err)
	}

	// Step 3: Insert notifications into the "post-notifications" collection
	collection := s.database.Collection("notifications")
	_, err = collection.InsertMany(context.Background(), notifications)
	if err != nil {
		return fmt.Errorf("error inserting notifications: %v", err)
	}

	settings, err := s.GetUserNotificationSettings(post.User_id)
	if err != nil {
		return fmt.Errorf("error searching user settings: %v", err)
	}

	if settings.NewComment && (userID != post.User_id) {
		err = sendPushNotification(pushToken, commentingUser.UserName, "Ha comentado tu publicación!")
		if err != nil {
			return fmt.Errorf("error sending push notification: %v", err)
		}
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

func (s *store) CreateUserFollowNotification(CreatorID primitive.ObjectID, userID primitive.ObjectID, notificationType string) error {

	// Retrieve user of post details
	//creatorUser, err := s.GetUserByID(CreatorID)
	//if err != nil {
	//	return fmt.Errorf("error getting user: %v", err)
	//}

	// Retrieve user of post details
	followingUser, err := s.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("error getting user: %v", err)
	}

	// Prepare notifications for each follower
	notifications := []interface{}{}

	pushToken, err := s.GetPushTokenByUserID(CreatorID)
	if err != nil {
		return fmt.Errorf("error getting push token for follower: %v", err)
	}

	notification := bson.M{
		"user": bson.M{
			"_id":         followingUser.ID,          // Store the ObjectID directly
			"firstName":   followingUser.FirstName,   // Store user's first name
			"lastName":    followingUser.LastName,    // Store user's last name
			"username":    followingUser.UserName,    // Store user's username
			"avatarUri":   followingUser.Avatar_uri,  // Store user's avatar URI
			"bannerUri":   followingUser.Banner_uri,  // Store user's banner URI
			"description": followingUser.Description, // Store user's description
			"createdAt":   followingUser.CreatedAt,   // Store user's creation time
			"followers":   followingUser.Followers,
			"following":   followingUser.Following,
		},
		"createdAt": time.Now(), // Notification creation time
		"destinationUser": bson.M{
			"id": CreatorID, // Store the ObjectID directly
			// Initially set as unread
		},
		"isRead": false,
		"type":   notificationType, // Notification type: "Follow", "Like", "Comment", "Message", "newPost"
	}

	notifications = append(notifications, notification)

	err = s.IncrementPendingNotifications(CreatorID)
	if err != nil {
		return fmt.Errorf("error updating notification counter: %v", err)
	}

	// Step 3: Insert notifications into the "post-notifications" collection
	collection := s.database.Collection("notifications")
	_, err = collection.InsertMany(context.Background(), notifications)
	if err != nil {
		return fmt.Errorf("error inserting notifications: %v", err)
	}

	settings, err := s.GetUserNotificationSettings(CreatorID)
	if err != nil {
		return fmt.Errorf("error searching user settings: %v", err)
	}

	if settings.NewComment {
		err = sendPushNotification(pushToken, followingUser.UserName, "Te ha comenzado a seguir!")
		if err != nil {
			return fmt.Errorf("error sending push notification: %v", err)
		}
	}

	return nil
}

func (s *store) CreateChatmsgNotification(ChatID primitive.ObjectID, userID primitive.ObjectID, text string) error {

	chat, err := s.GetChatByID(ChatID)
	if err != nil {
		return fmt.Errorf("error getting user: %v", err)
	}

	var recipientID primitive.ObjectID
	if userID == chat.CreatorID {
		recipientID = chat.UserID
	} else {
		recipientID = chat.CreatorID
	}
	// Retrieve user of post details
	sendingUser, err := s.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("error getting user: %v", err)
	}

	pushToken, err := s.GetPushTokenByUserID(recipientID)
	if err != nil {
		return fmt.Errorf("error getting push token for follower: %v", err)
	}

	settings, err := s.GetUserNotificationSettings(recipientID)
	if err != nil {
		return fmt.Errorf("error searching user settings: %v", err)
	}

	if settings.UnreadMessages {
		err = sendPushNotification(pushToken, sendingUser.UserName, text)
		if err != nil {
			return fmt.Errorf("error sending push notification: %v", err)
		}
	}

	return nil
}

func (s *store) UpdateNotificationCounter(userID primitive.ObjectID) error {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Define the filter to locate the document to update
	filter := bson.M{"_id": userID}

	// Define the update operation to set the "request" field to false
	update := bson.M{
		"$set": bson.M{
			"pendingNotifications": 0,
		},
	}

	// Perform the update operation
	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}
