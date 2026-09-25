package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"mime/multipart"
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
	client       *mongo.Client
	database     *mongo.Database
	postS3Client *aws_s3.S3Client
}

func NewStore(client *mongo.Client, postS3Client *aws_s3.S3Client) *store {
	return &store{client: client, database: client.Database(mongoDatabaseName), postS3Client: postS3Client}
}

func (s *store) Create(context.Context) error {
	return nil
}

func (s *store) CreatePosts(post common.Post) (primitive.ObjectID, error) {
	collection := s.database.Collection("posts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := collection.InsertOne(ctx, post)
	if err != nil {
		return primitive.ObjectID{}, err
	}

	id, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return primitive.ObjectID{}, fmt.Errorf("inserted ID is not of type ObjectID")
	}

	return id, nil
}

func (s *store) DeletePost(postID primitive.ObjectID) error {
	collection := s.database.Collection("posts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Define the update to set "hidden" to true
	update := bson.M{"$set": bson.M{"hidden": true}}
	_, err := collection.UpdateOne(ctx, bson.M{"_id": postID}, update)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) CreatePostLike(postLike PostLike) error {
	collection := s.database.Collection("posts-like")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if the PostLike already exists
	filter := bson.M{
		"user_id": postLike.User_id,
		"post_id": postLike.Post_id,
	}
	var existingPostLike PostLike
	err := collection.FindOne(ctx, filter).Decode(&existingPostLike)
	if err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	if existingPostLike.ID != primitive.NilObjectID {
		return fmt.Errorf("post like already exists")
	}

	_, err = collection.InsertOne(ctx, postLike)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) DeletePostLike(userID primitive.ObjectID, postID primitive.ObjectID) error {
	collection := s.database.Collection("posts-like")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Perform the delete operation
	result, err := collection.DeleteOne(ctx, bson.M{"user_id": userID, "post_id": postID})
	if err != nil {
		return err
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("no post like found for user ID %s and post ID %s", userID.Hex(), postID.Hex())
	}

	return nil
}

func (s *store) IncrementPostLike(postID primitive.ObjectID) error {
	collection := s.database.Collection("posts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": postID}
	update := bson.M{"$inc": bson.M{"likes": 1}}

	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) DecrementPostLike(postID primitive.ObjectID) error {
	collection := s.database.Collection("posts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": postID}
	update := bson.M{"$inc": bson.M{"likes": -1}}

	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) CreateComments(comment common.Comment) (primitive.ObjectID, error) {
	collection := s.database.Collection("posts-comment")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := collection.InsertOne(ctx, comment)
	if err != nil {
		return primitive.NilObjectID, err
	}

	// Assuming the ID is an ObjectID and the insert was successful, extract the ID
	insertedID := result.InsertedID.(primitive.ObjectID)

	return insertedID, nil
}

func (s *store) DeleteCommentByID(commentID primitive.ObjectID) error {
	collection := s.database.Collection("posts-comment")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Update the "hidden" field to true instead of deleting the comment
	update := bson.M{
		"$set": bson.M{
			"hidden": true,
		},
	}

	// Perform the update operation
	result, err := collection.UpdateOne(ctx, bson.M{"_id": commentID}, update)
	if err != nil {
		return fmt.Errorf("error updating comment: %v", err)
	}

	// Check if a document was modified
	if result.MatchedCount == 0 {
		return fmt.Errorf("comment not found")
	}

	return nil
}

func (s *store) CreateCommentLike(commentLike CommentLike) error {
	collection := s.database.Collection("comment-like")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if the PostLike already exists
	filter := bson.M{
		"user_id":    commentLike.User_id,
		"comment_id": commentLike.Comment_id,
	}
	var existingcommentLike CommentLike
	err := collection.FindOne(ctx, filter).Decode(&existingcommentLike)
	if err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	if existingcommentLike.ID != primitive.NilObjectID {
		return fmt.Errorf("post like already exists")
	}

	_, err = collection.InsertOne(ctx, commentLike)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) DeleteCommentLike(userID primitive.ObjectID, commentID primitive.ObjectID) error {
	collection := s.database.Collection("comment-like")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Perform the delete operation
	result, err := collection.DeleteOne(ctx, bson.M{"user_id": userID, "comment_id": commentID})
	if err != nil {
		return err
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("no comment like found for user ID %s and comment ID %s", userID.Hex(), commentID.Hex())
	}

	return nil
}

func (s *store) UpdateCommentLikeCounter(commentID primitive.ObjectID, increaseDecrease int) error {
	collection := s.database.Collection("posts-comment")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": commentID}
	update := bson.M{"$inc": bson.M{"likes": increaseDecrease}}

	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) CheckCommentLike(commentLike CommentLike) (bool, error) {
	collection := s.database.Collection("comment-like")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if the PostLike already exists
	filter := bson.M{
		"user_id":    commentLike.User_id,
		"comment_id": commentLike.Comment_id,
	}
	var existingCommentLike CommentLike
	err := collection.FindOne(ctx, filter).Decode(&existingCommentLike)
	if err != nil && err != mongo.ErrNoDocuments {
		return false, err
	}

	if existingCommentLike.ID != primitive.NilObjectID {
		return true, nil
	}

	return false, nil
}

func (s *store) UpdateCommentCounter(postID primitive.ObjectID, increaseDecrease int) error {
	collection := s.database.Collection("posts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": postID}
	update := bson.M{"$inc": bson.M{"comments": increaseDecrease}}

	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) DecreaseCommentCounterByCommentID(commentID primitive.ObjectID, increaseDecrease int) error {
	// Collection where comments are stored
	commentCollection := s.database.Collection("posts-comment")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Find the comment by commentID
	var comment struct {
		PostID primitive.ObjectID `bson:"post_id"`
	}
	err := commentCollection.FindOne(ctx, bson.M{"_id": commentID}).Decode(&comment)
	if err != nil {
		return fmt.Errorf("failed to find comment: %v", err)
	}

	// Use the postID to update the comment counter
	err = s.UpdateCommentCounter(comment.PostID, increaseDecrease)
	if err != nil {
		return fmt.Errorf("failed to update comment counter: %v", err)
	}

	return nil
}

func (s *store) CreateReplies(reply common.Reply) error {
	collection := s.database.Collection("comments-reply")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := collection.InsertOne(ctx, reply)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) DeleteRepliesByID(replyID primitive.ObjectID) error {
	collection := s.database.Collection("comments-reply")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Perform the delete operation
	result, err := collection.DeleteOne(ctx, bson.M{"_id": replyID})
	if err != nil {
		return err
	}

	// Check if a document was deleted
	if result.DeletedCount == 0 {
		return fmt.Errorf("reply not found")
	}

	return nil
}

func (s *store) UpdateReplyCounter(commentID primitive.ObjectID, increaseDecrease int) error {
	collection := s.database.Collection("posts-comment")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": commentID}
	update := bson.M{"$inc": bson.M{"replies": increaseDecrease}}

	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) UpdateLikeCounter(commentID primitive.ObjectID, increaseDecrease int) error {
	collection := s.database.Collection("posts-comment")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": commentID}
	update := bson.M{"$inc": bson.M{"likes": increaseDecrease}}

	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) GetLatestReplies(commentID primitive.ObjectID, page int, pageSize int) ([]ReplyWithUser, error) {
	// Define the collection for replies
	replyCollection := s.database.Collection("comments-reply")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Define find options for pagination and sorting
	opts := options.Find()
	opts.SetSort(bson.D{primitive.E{Key: "createdAt", Value: -1}})
	opts.SetSkip(int64((page - 1) * pageSize))
	opts.SetLimit(int64(pageSize))

	// Query for replies associated with the given commentID
	filter := bson.M{"comment_id": commentID}
	cursor, err := replyCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	// Decode the results into a slice of Reply objects
	var replies []common.Reply
	if err = cursor.All(ctx, &replies); err != nil {
		return nil, err
	}

	// Define the collection for users
	userCollection := s.database.Collection("users")

	var repliesWithUsers []ReplyWithUser

	// Iterate over each reply and fetch the corresponding user
	for _, reply := range replies {
		var user common.User
		err := userCollection.FindOne(ctx, bson.M{"_id": reply.UserID}).Decode(&user)
		if err != nil {
			return nil, err
		}
		replyWithUser := ReplyWithUser{
			Reply: reply,
			User:  user,
		}
		repliesWithUsers = append(repliesWithUsers, replyWithUser)
	}

	return repliesWithUsers, nil
}

func (s *store) GetLatestComments(postID primitive.ObjectID, userID primitive.ObjectID, page int, pageSize int) ([]CommentWithUser, error) {
	postCollection := s.database.Collection("posts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Retrieve the post to get the userID of the owner
	var post common.Post
	err := postCollection.FindOne(ctx, bson.M{"_id": postID, "hidden": false}).Decode(&post)
	if err != nil {
		return nil, fmt.Errorf("post not found: %v", err)
	}

	userCollection := s.database.Collection("users")
	commentCollection := s.database.Collection("posts-comment")

	var commentsWithUsers []CommentWithUser

	// Step 1: Get comments from the requesting user, filtered by gems
	userCommentsCursor, err := commentCollection.Find(ctx, bson.M{
		"post_id": postID,
		"user_id": userID,
		"hidden":  false,
	}, options.Find().SetSort(bson.D{{Key: "gems", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer userCommentsCursor.Close(ctx)

	var userComments []common.Comment
	if err = userCommentsCursor.All(ctx, &userComments); err != nil {
		return nil, err
	}

	for _, comment := range userComments {
		var user common.User
		err := userCollection.FindOne(ctx, bson.M{"_id": comment.UserID}).Decode(&user)
		if err != nil {
			return nil, err
		}
		commentsWithUsers = append(commentsWithUsers, CommentWithUser{
			Comment: comment,
			User:    user,
		})
	}

	// Step 2: Get comments from other users, filtered by gems
	otherCommentsCursor, err := commentCollection.Find(ctx, bson.M{
		"post_id": postID,
		"user_id": bson.M{"$ne": userID},
		"hidden":  false,
	}, options.Find().SetSort(bson.D{{Key: "gems", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer otherCommentsCursor.Close(ctx)

	var otherComments []common.Comment
	if err = otherCommentsCursor.All(ctx, &otherComments); err != nil {
		return nil, err
	}

	for _, comment := range otherComments {
		var user common.User
		err := userCollection.FindOne(ctx, bson.M{"_id": comment.UserID}).Decode(&user)
		if err != nil {
			return nil, err
		}
		commentsWithUsers = append(commentsWithUsers, CommentWithUser{
			Comment: comment,
			User:    user,
		})
	}

	// Step 3: Apply pagination on combined results without further sorting
	totalComments := len(commentsWithUsers)

	// Calculate start and end index for pagination
	start := (page - 1) * pageSize
	end := start + pageSize

	// Handle cases where pagination exceeds bounds
	if start > totalComments {
		return nil, nil // No comments on this page
	}
	if end > totalComments {
		end = totalComments
	}

	// Return the paginated subset of comments
	return commentsWithUsers[start:end], nil
}

// S3
func (s *store) UploadContent(encodedContent []string) ([]string, error) {

	var contentUrls []string
	//Decode

	for _, encodedImage := range encodedContent {
		// Decode the base64 encoded image
		decodedImage, err := base64.StdEncoding.DecodeString(encodedImage)
		if err != nil {
			return nil, fmt.Errorf("failed to decode image: %v", err)
		}
		//Generar nombre random
		filename, err := common.GenerateRandomImageName("jpg")
		if err != nil {
			return nil, err
		}
		//Subir al bucket
		err = s.postS3Client.Upload(decodedImage, filename)
		if err != nil {
			return nil, err
		}
		//Colocar el url en el array
		url := "https://" + CloudFlare_Link + "/" + filename
		log.Println(url)

		// Append the decoded image to the array
		contentUrls = append(contentUrls, url)
	}

	return contentUrls, nil
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
		err = s.postS3Client.Upload(fileBytes, filename)
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

func (s *store) GetLatestsPosts(UserID primitive.ObjectID, page int, pageSize int) ([]PostWithUser, error) {
	postCollection := s.database.Collection("posts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"hidden": false}

	opts := options.Find()
	opts.SetSort(bson.D{primitive.E{Key: "createdAt", Value: -1}})
	opts.SetSkip(int64((page - 1) * pageSize))
	opts.SetLimit(int64(pageSize))

	cursor, err := postCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var posts []common.Post
	if err = cursor.All(ctx, &posts); err != nil {
		return nil, err
	}

	userCollection := s.database.Collection("users")

	var postsWithUsers []PostWithUser

	for _, post := range posts {
		var user common.User
		err := userCollection.FindOne(ctx, bson.M{"_id": post.User_id}).Decode(&user)
		if err != nil {
			return nil, err
		}

		//si post tier 0 ignorar resto
		//		verificar tier del post
		//		verificar tier del user
		//		comparar y meter blur o no blur
		//		post.Content = post.BluredContent

		var subbed = true
		if post.User_id != UserID {

			if post.Tier != 0 {
				userTIer, err := s.GetSubscriptionTier(UserID, post.User_id)
				if err != nil {
					return nil, err
				}

				if userTIer < post.Tier {
					post.Content = post.BluredContent
					subbed = false
				}

			}
		}

		postWithUser := PostWithUser{
			Post:       post,
			User:       user,
			Subscribed: subbed,
		}
		postsWithUsers = append(postsWithUsers, postWithUser)
	}

	return postsWithUsers, nil
}

func (s *store) GetLatestsUserPosts(UserID primitive.ObjectID, CreatorID primitive.ObjectID, page int, pageSize int) ([]PostWithUser, error) {
	postCollection := s.database.Collection("posts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"user_id": CreatorID, "hidden": false}

	opts := options.Find()
	opts.SetSort(bson.D{primitive.E{Key: "createdAt", Value: -1}})
	opts.SetSkip(int64((page - 1) * pageSize))
	opts.SetLimit(int64(pageSize))

	cursor, err := postCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var posts []common.Post
	if err = cursor.All(ctx, &posts); err != nil {
		return nil, err
	}

	userCollection := s.database.Collection("users")

	var postsWithUsers []PostWithUser

	for _, post := range posts {
		var postUser common.User
		err := userCollection.FindOne(ctx, bson.M{"_id": post.User_id}).Decode(&postUser)
		if err != nil {
			return nil, err
		}

		var subbed = true

		if post.User_id != UserID {

			if post.Tier != 0 {
				userTIer, err := s.GetSubscriptionTier(UserID, post.User_id)
				if err != nil {
					return nil, err
				}

				user, err := s.GetUserByID(UserID.Hex())
				if err != nil {
					return nil, err
				}

				if userTIer < post.Tier && !s.IsAdmin(*user) {
					post.Content = post.BluredContent
					subbed = false
				}

			}
		}

		///mandar impresion

		postWithUser := PostWithUser{
			Post:       post,
			User:       postUser,
			Subscribed: subbed,
		}
		postsWithUsers = append(postsWithUsers, postWithUser)
	}

	return postsWithUsers, nil
}

func (s *store) ConvertToGrpcPost(post *common.Post) *pb.Post {
	return &pb.Post{
		Id:          post.ID.Hex(),
		Title:       post.Title,
		Description: post.Description,
		UserId:      post.User_id.Hex(),
		Content:     post.Content,
		Tags:        post.Tags,
		Gems:        int32(post.Gems),
		Likes:       int32(post.Likes),
		Comments:    int32(post.Comments),
		Shares:      int32(post.Shares),
		Saves:       int32(post.Saves),
		Tier:        int32(post.Tier),
		Created_At:  timestamppb.New(post.CreatedAt),
		Updated_At:  timestamppb.New(post.UpdatedAt),
		Impression:  int64(post.Impression),
	}
}

func (s *store) ConvertToGrpcComment(comment *common.Comment) *pb.Comment {
	return &pb.Comment{
		Id:         comment.ID.Hex(),
		UserId:     comment.UserID.Hex(),
		PostId:     comment.PostID.Hex(),
		Text:       comment.Text,
		Likes:      comment.Likes,
		Gems:       comment.Gems,
		Replies:    comment.Replies,
		Created_At: timestamppb.New(comment.CreatedAt),
		Updated_At: timestamppb.New(comment.UpdatedAt),
	}
}

func (s *store) ConvertToGrpcUser(user *common.User) *pb.User {
	return &pb.User{
		Id:          user.ID.Hex(),
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		DisplayName: user.DisplayName,
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
		Subscribers: int64(user.Subscribers),
		Description: user.Description,
	}
}

func (s *store) ConvertToGrpcPublicUser(user *common.User) *pb.User {
	return &pb.User{
		Id:          user.ID.Hex(),
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		DisplayName: user.DisplayName,
		Username:    user.UserName,
		CreatedAt:   user.CreatedAt.String(),
		AvatarUri:   user.Avatar_uri,
		BannerUri:   user.Banner_uri,
		Followers:   int64(user.Followers),
		Following:   int64(user.Following),
		Subscribers: int64(user.Subscribers),
		Description: user.Description,
	}
}

func (s *store) ConvertToGrpcReply(reply *common.Reply) *pb.Reply {
	return &pb.Reply{
		Id:         reply.ID.Hex(),
		UserId:     reply.UserID.Hex(),
		CommentId:  reply.CommentID.Hex(),
		Text:       reply.Text,
		Created_At: timestamppb.New(reply.CreatedAt),
		Updated_At: timestamppb.New(reply.UpdatedAt),
	}
}

func (s *store) CheckPostLike(postLike PostLike) (bool, error) {
	collection := s.database.Collection("posts-like")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if the PostLike already exists
	filter := bson.M{
		"user_id": postLike.User_id,
		"post_id": postLike.Post_id,
	}
	var existingPostLike PostLike
	err := collection.FindOne(ctx, filter).Decode(&existingPostLike)
	if err != nil && err != mongo.ErrNoDocuments {
		return false, err
	}

	if existingPostLike.ID != primitive.NilObjectID {
		return true, nil
	}

	return false, nil
}

func (s *store) GetPost(postID, UserID primitive.ObjectID) ([]PostWithUser, error) {
	postCollection := s.database.Collection("posts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": postID, "hidden": false}

	cursor, err := postCollection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var posts []common.Post
	if err = cursor.All(ctx, &posts); err != nil {
		return nil, err
	}

	userCollection := s.database.Collection("users")

	var postsWithUsers []PostWithUser

	for _, post := range posts {
		var postUser common.User
		err := userCollection.FindOne(ctx, bson.M{"_id": post.User_id}).Decode(&postUser)
		if err != nil {
			return nil, err
		}

		var subbed = true

		if post.User_id != UserID {

			if post.Tier != 0 {
				userTIer, err := s.GetSubscriptionTier(UserID, post.User_id)
				if err != nil {
					return nil, err
				}

				user, err := s.GetUserByID(UserID.Hex())
				if err != nil {
					return nil, err
				}

				if userTIer < post.Tier && !s.IsAdmin(*user) {
					post.Content = post.BluredContent
					subbed = false
				}

			}
		}

		///mandar impresion

		postWithUser := PostWithUser{
			Post:       post,
			User:       postUser,
			Subscribed: subbed,
		}
		postsWithUsers = append(postsWithUsers, postWithUser)
	}

	return postsWithUsers, nil
}

func (s *store) GetPostByID(postID primitive.ObjectID) (*common.Post, error) {
	collection := s.database.Collection("posts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": postID, "hidden": false}
	var post common.Post

	err := collection.FindOne(ctx, filter).Decode(&post)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("post not found: id %s", postID)
		}
		return nil, err
	}
	return &post, nil
}

func (s *store) GetSubscriptionTier(userID, creatorID primitive.ObjectID) (int, error) {
	collection := s.database.Collection("user-subscriptions")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Filter by userID and creatorID
	filter := bson.M{
		"user_id":    userID,
		"creator_id": creatorID,
	}

	// Define a variable to store the result
	var subscription common.Subscription

	// Query MongoDB to find the subscription
	err := collection.FindOne(ctx, filter).Decode(&subscription)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return 0, nil // Return 0 if no subscription is found
		}
		return 0, fmt.Errorf("error finding subscription: %v", err)
	}

	// Return the tier
	return subscription.Tier, nil
}

func (s *store) GetUserIDFromComment(commentID primitive.ObjectID) (primitive.ObjectID, error) {
	collection := s.database.Collection("posts-comment")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Filter to find the comment by its ID
	filter := bson.M{
		"_id": commentID,
	}

	// Define a variable to store the result
	var comment common.Comment

	// Query MongoDB to find the comment
	err := collection.FindOne(ctx, filter).Decode(&comment)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return primitive.NilObjectID, fmt.Errorf("comment with ID %s not found", commentID.Hex())
		}
		return primitive.NilObjectID, fmt.Errorf("error finding comment: %v", err)
	}

	// Return the UserID
	return comment.UserID, nil
}

func (s *store) CountCommentsByUserAndPost(userID, postID primitive.ObjectID) (int64, error) {
	// Acceder a la colección de comentarios
	collection := s.database.Collection("posts-comment")

	// Contexto con timeout para la operación de base de datos
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Filtro para buscar los documentos por user_id y post_id
	filter := bson.M{
		"user_id": userID,
		"post_id": postID,
	}

	// Contar los documentos que coinciden con el filtro
	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("error counting documents: %v", err)
	}

	// Devolver el número de documentos encontrados
	return count, nil
}

func (s *store) GetFollowingByUserID(userID primitive.ObjectID) ([]primitive.ObjectID, error) {
	collection := s.database.Collection("user-follows")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a filter to find all documents where the follower_id matches userID
	filter := bson.M{"follower_id": userID}
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("error querying following: %v", err)
	}
	defer cursor.Close(ctx)

	var following []primitive.ObjectID
	for cursor.Next(ctx) {
		var result struct {
			FollowingID primitive.ObjectID `bson:"following_id"`
		}
		if err := cursor.Decode(&result); err != nil {
			return nil, fmt.Errorf("error decoding following: %v", err)
		}
		// Append the following_id to the following list
		following = append(following, result.FollowingID)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %v", err)
	}

	return following, nil
}

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

func (s *store) IsAdmin(user common.User) bool {
	for _, role := range user.Roles {
		if role == "admin" {
			return true
		}
	}
	return false
}

func (s *store) IsCreator(user common.User) bool {
	for _, role := range user.Roles {
		if role == "creator" {
			return true
		}
	}
	return false
}

func (s *store) GetPostOwnerFromComment(commentID primitive.ObjectID) (primitive.ObjectID, error) {
	// Collection for comments
	commentCollection := s.database.Collection("posts-comment")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Step 1: Find the comment by its ID
	var comment common.Comment
	commentFilter := bson.M{
		"_id": commentID,
	}

	err := commentCollection.FindOne(ctx, commentFilter).Decode(&comment)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return primitive.NilObjectID, fmt.Errorf("comment with ID %s not found", commentID.Hex())
		}
		return primitive.NilObjectID, fmt.Errorf("error finding comment: %v", err)
	}

	// Now we have the PostID from the comment
	postID := comment.PostID

	// Step 2: Find the post by its ID
	postCollection := s.database.Collection("posts")
	var post common.Post
	postFilter := bson.M{
		"_id": postID,
	}

	err = postCollection.FindOne(ctx, postFilter).Decode(&post)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return primitive.NilObjectID, fmt.Errorf("post with ID %s not found", postID.Hex())
		}
		return primitive.NilObjectID, fmt.Errorf("error finding post: %v", err)
	}

	// Return the UserID (owner of the post)
	return post.User_id, nil
}

func (s *store) UpsertPostImpression(impression PostImpression) error {
	collection := s.database.Collection("post-impression")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Filter for the upsert operation
	filter := bson.M{
		"post_id": impression.PostID,
		"user_id": impression.UserID,
	}

	// Define the update with the upsert flag
	update := bson.M{
		"$set": bson.M{
			"createdAt": time.Now(), // Update the timestamp to the current time
		},
		"$setOnInsert": bson.M{
			"post_id": impression.PostID,
			"user_id": impression.UserID,
		},
	}

	// Perform the upsert operation
	_, err := collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("error upserting post impression: %v", err)
	}

	return nil
}

func (s *store) SharePost(userID, postID primitive.ObjectID) error {
	shareCollection := s.database.Collection("post-share")
	postCollection := s.database.Collection("posts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Step 1: Check if a PostShare already exists
	filter := bson.M{
		"user_id": userID,
		"post_id": postID,
	}

	// Try to find the PostShare
	var existingShare common.PostShare
	err := shareCollection.FindOne(ctx, filter).Decode(&existingShare)
	if err == nil {
		// PostShare already exists, return without error (no need to increase the count)
		return nil
	}

	// If the error is not "no documents found," return the error
	if err != mongo.ErrNoDocuments {
		return fmt.Errorf("error checking for existing PostShare: %v", err)
	}

	// Step 2: Insert the new PostShare
	newShare := common.PostShare{
		UserID:    userID,
		PostID:    postID,
		CreatedAt: time.Now(),
	}

	_, err = shareCollection.InsertOne(ctx, newShare)
	if err != nil {
		return fmt.Errorf("error inserting new PostShare: %v", err)
	}

	// Step 3: Increase the "shares" count in the post
	update := bson.M{
		"$inc": bson.M{"shares": 1},
	}

	_, err = postCollection.UpdateOne(ctx, bson.M{"_id": postID}, update)
	if err != nil {
		return fmt.Errorf("error updating post shares count: %v", err)
	}

	return nil
}

func (s *store) GetRecommended_(UserID primitive.ObjectID) ([]PostWithUser, error) {
	// Step 1: Get the list of creators the user is following
	creatorsIDs, err := s.GetFollowingByUserID(UserID)
	if err != nil {
		return nil, err
	}

	if len(creatorsIDs) == 0 {
		return nil, fmt.Errorf("no creators found for user")
	}

	postCollection := s.database.Collection("posts")
	impresionCollection := s.database.Collection("post-impression")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Step 2: Fetch distinct post IDs where the user has already seen the posts
	distinctPostIDs, err := impresionCollection.Distinct(ctx, "post_id", bson.M{"user_id": UserID})
	if err != nil {
		return nil, fmt.Errorf("error fetching distinct post IDs: %v", err)
	}

	// Step 3: Build the filter
	filter := bson.M{
		"user_id": bson.M{"$in": creatorsIDs}, // Get posts from followed creators
		"hidden":  false,                      // Filter out hidden posts
	}

	// If there are any distinctPostIDs, exclude them
	if len(distinctPostIDs) > 0 {
		filter["_id"] = bson.M{
			"$nin": distinctPostIDs, // Exclude posts the user has already seen
		}
	}

	opts := options.Find()
	opts.SetSort(bson.D{primitive.E{Key: "createdAt", Value: -1}}) // Sort by newest first
	opts.SetLimit(int64(10))

	cursor, err := postCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var posts []common.Post
	if err = cursor.All(ctx, &posts); err != nil {
		return nil, err
	}

	// Step 4: Process posts and create impressions
	userCollection := s.database.Collection("users")
	var postsWithUsers []PostWithUser

	for _, post := range posts {
		// Create a new impression (upsert)
		impresion := PostImpression{
			PostID:    post.ID,
			UserID:    UserID,
			CreatedAt: time.Now(),
		}
		err = s.UpsertPostImpression(impresion)
		if err != nil {
			return nil, err
		}

		// Increment the post's impression count
		_, err = postCollection.UpdateOne(
			ctx,
			bson.M{"_id": post.ID},
			bson.M{"$inc": bson.M{"impression": 1}}, // Increment the "impression" counter by 1
		)
		if err != nil {
			return nil, fmt.Errorf("error updating post impression counter: %v", err)
		}

		// Retrieve the user who created the post
		var postUser common.User
		err = userCollection.FindOne(ctx, bson.M{"_id": post.User_id}).Decode(&postUser)
		if err != nil {
			return nil, err
		}

		// Handle subscription and blur logic
		var subbed = true
		if post.User_id != UserID && post.Tier != 0 {
			userTier, err := s.GetSubscriptionTier(UserID, post.User_id)
			if err != nil {
				return nil, err
			}

			user, err := s.GetUserByID(UserID.Hex())
			if err != nil {
				return nil, err
			}

			if userTier < post.Tier && !s.IsAdmin(*user) {
				post.Content = post.BluredContent
				subbed = false
			}
		}

		// Combine post with user data
		postWithUser := PostWithUser{
			Post:       post,
			User:       postUser,
			Subscribed: subbed,
		}

		postsWithUsers = append(postsWithUsers, postWithUser)
	}

	// Step 5: If not enough posts, get posts with most impressions in the last 24 hours
	if len(postsWithUsers) < 10 {
		remainingSlots := 10 - len(postsWithUsers)
		topPostsWithMostImpressions, err := s.GetTopPostsWithMostImpressionsInLast24Hrs(UserID, remainingSlots, distinctPostIDs)
		if err != nil {
			return nil, err
		}

		postsWithUsers = append(postsWithUsers, topPostsWithMostImpressions...)
	}

	return postsWithUsers, nil
}

func (s *store) GetRecommended(UserID primitive.ObjectID) ([]PostWithUser, error) {
	// Step 1: Get the list of creators the user is following
	creatorsIDs, err := s.GetFollowingByUserID(UserID)
	if err != nil {
		return nil, err
	}

	// Step 2: Continue regardless of whether creatorsIDs is empty.
	postCollection := s.database.Collection("posts")
	impresionCollection := s.database.Collection("post-impression")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Step 3: Fetch distinct post IDs where the user has already seen the posts
	distinctPostIDs, err := impresionCollection.Distinct(ctx, "post_id", bson.M{"user_id": UserID})
	if err != nil {
		return nil, fmt.Errorf("error fetching distinct post IDs: %v", err)
	}
	log.Print("posts encontrados con impresiones")
	log.Print(distinctPostIDs)

	// Step 4: Build the filter (only apply it if creatorsIDs is not empty)
	filter := bson.M{
		"hidden": false, // Filter out hidden posts
	}
	if len(creatorsIDs) > 0 {
		filter["user_id"] = bson.M{"$in": creatorsIDs} // Get posts from followed creators
	}

	// If there are any distinctPostIDs, exclude them
	if len(distinctPostIDs) > 0 {
		filter["_id"] = bson.M{
			"$nin": distinctPostIDs, // Exclude posts the user has already seen
		}
	}

	opts := options.Find()
	opts.SetSort(bson.D{primitive.E{Key: "createdAt", Value: -1}}) // Sort by newest first
	opts.SetLimit(int64(10))

	cursor, err := postCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var posts []common.Post
	if err = cursor.All(ctx, &posts); err != nil {
		return nil, err
	}

	// Step 5: Process posts and create impressions
	userCollection := s.database.Collection("users")
	var postsWithUsers []PostWithUser

	for _, post := range posts {
		// Create a new impression (upsert)
		impresion := PostImpression{
			PostID:    post.ID,
			UserID:    UserID,
			CreatedAt: time.Now(),
		}
		err = s.UpsertPostImpression(impresion)
		if err != nil {
			return nil, err
		}

		// Increment the post's impression count
		_, err = postCollection.UpdateOne(
			ctx,
			bson.M{"_id": post.ID},
			bson.M{"$inc": bson.M{"impression": 1}}, // Increment the "impression" counter by 1
		)
		if err != nil {
			return nil, fmt.Errorf("error updating post impression counter: %v", err)
		}

		// Retrieve the user who created the post
		var postUser common.User
		err = userCollection.FindOne(ctx, bson.M{"_id": post.User_id}).Decode(&postUser)
		if err != nil {
			return nil, err
		}

		// Handle subscription and blur logic
		var subbed = true
		if post.User_id != UserID && post.Tier != 0 {
			userTier, err := s.GetSubscriptionTier(UserID, post.User_id)
			if err != nil {
				return nil, err
			}

			user, err := s.GetUserByID(UserID.Hex())
			if err != nil {
				return nil, err
			}

			if userTier < post.Tier && !s.IsAdmin(*user) {
				post.Content = post.BluredContent
				subbed = false
			}
		}

		// Combine post with user data
		postWithUser := PostWithUser{
			Post:       post,
			User:       postUser,
			Subscribed: subbed,
		}

		postsWithUsers = append(postsWithUsers, postWithUser)
	}

	// Step 6: If not enough posts, get posts with most impressions in the last 24 hours
	log.Print("posts selecionados, toco entrar al metodo si es <10")
	log.Print(len(postsWithUsers))
	if len(postsWithUsers) < 10 {
		remainingSlots := 10 - len(postsWithUsers)
		topPostsWithMostImpressions, err := s.GetTopPostsWithMostImpressionsInLast24Hrs(UserID, remainingSlots, distinctPostIDs)
		if err != nil {
			return nil, err
		}

		postsWithUsers = append(postsWithUsers, topPostsWithMostImpressions...)
	}

	return postsWithUsers, nil
}

func (s *store) GetTopPostsWithMostImpressionsInLast24Hrs_(userID primitive.ObjectID, limit int) ([]PostWithUser, error) {
	impresionCollection := s.database.Collection("post-impression")
	postCollection := s.database.Collection("posts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Calculate the time 24 hours ago
	last24Hours := time.Now().Add(-24 * time.Hour)

	// Part 1: Aggregate posts with impressions from the last 24 hours
	pipeline := mongo.Pipeline{
		// Match impressions created within the last 24 hours
		{{
			Key: "$match",
			Value: bson.D{
				{Key: "createdAt", Value: bson.D{{Key: "$gte", Value: last24Hours}}},
			},
		}},
		// Group by post_id and count the impressions for each post
		{{
			Key: "$group",
			Value: bson.D{
				{Key: "_id", Value: "$post_id"},
				{Key: "impressionCount", Value: bson.D{{Key: "$sum", Value: 1}}},
			},
		}},
		// Sort by impressionCount in descending order
		{{
			Key:   "$sort",
			Value: bson.D{{Key: "impressionCount", Value: -1}},
		}},
		// Limit the number of results
		{{
			Key:   "$limit",
			Value: int64(limit),
		}},
	}

	cursor, err := impresionCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("error fetching impressions: %v", err)
	}
	defer cursor.Close(ctx)

	var topPostsWithImpressions []struct {
		PostID          primitive.ObjectID `bson:"_id"`
		ImpressionCount int                `bson:"impressionCount"`
	}
	if err := cursor.All(ctx, &topPostsWithImpressions); err != nil {
		return nil, fmt.Errorf("error decoding impressions: %v", err)
	}

	// If no impressions are found, log it
	if len(topPostsWithImpressions) == 0 {
		log.Println("No impressions found in the last 24 hours")
	}

	// Step 3: Fetch the actual posts using the post IDs
	var postIDs []primitive.ObjectID
	for _, record := range topPostsWithImpressions {
		postIDs = append(postIDs, record.PostID)
	}

	// Fetch posts that have impressions first
	postFilter := bson.M{
		"_id":     bson.M{"$in": postIDs},
		"hidden":  false,
		"user_id": bson.M{"$ne": userID}, // Ensure we're not fetching the user's own posts
	}
	postsCursor, err := postCollection.Find(ctx, postFilter)
	if err != nil {
		return nil, fmt.Errorf("error fetching posts: %v", err)
	}
	defer postsCursor.Close(ctx)

	var posts []common.Post
	if err = postsCursor.All(ctx, &posts); err != nil {
		return nil, fmt.Errorf("error decoding posts: %v", err)
	}

	// Step 4: Prepare posts with user data and create impressions
	var postsWithUsers []PostWithUser
	userCollection := s.database.Collection("users")

	for _, post := range posts {
		// Check if the user has already seen this post by checking impressions
		impresionFilter := bson.M{
			"post_id": post.ID,
			"user_id": userID,
		}
		count, err := impresionCollection.CountDocuments(ctx, impresionFilter)
		if err != nil {
			return nil, err
		}

		// If the user has seen this post, skip it
		if count > 0 {
			continue
		}

		// Create a new impression (upsert)
		impresion := PostImpression{
			PostID:    post.ID,
			UserID:    userID,
			CreatedAt: time.Now(),
		}
		err = s.UpsertPostImpression(impresion)
		if err != nil {
			return nil, err
		}

		// Increment the post's impression count
		_, err = postCollection.UpdateOne(
			ctx,
			bson.M{"_id": post.ID},
			bson.M{"$inc": bson.M{"impression": 1}}, // Increment the "impression" counter by 1
		)
		if err != nil {
			return nil, fmt.Errorf("error updating post impression counter: %v", err)
		}

		// Retrieve the user who created the post
		var postUser common.User
		err = userCollection.FindOne(ctx, bson.M{"_id": post.User_id}).Decode(&postUser)
		if err != nil {
			return nil, err
		}

		// Handle subscription and blur logic
		var subbed = true
		if post.User_id != userID {
			if post.Tier != 0 {
				userTier, err := s.GetSubscriptionTier(userID, post.User_id)
				if err != nil {
					return nil, err
				}

				user, err := s.GetUserByID(userID.Hex())
				if err != nil {
					return nil, err
				}

				if userTier < post.Tier && !s.IsAdmin(*user) {
					post.Content = post.BluredContent
					subbed = false
				}
			}
		}

		// Combine post with postUser data
		postWithUser := PostWithUser{
			Post:       post,
			User:       postUser,
			Subscribed: subbed,
		}

		postsWithUsers = append(postsWithUsers, postWithUser)
	}

	// Part 2: Fetch additional posts that do not have impressions
	if len(postsWithUsers) < limit {
		remainingLimit := limit - len(postsWithUsers)

		noImpressionsFilter := bson.M{
			"_id":     bson.M{"$nin": postIDs}, // Exclude posts with impressions
			"hidden":  false,
			"user_id": bson.M{"$ne": userID}, // Exclude the user's own posts
		}

		// Fetch posts without impressions
		remainingPostsCursor, err := postCollection.Find(ctx, noImpressionsFilter, options.Find().SetLimit(int64(remainingLimit)))
		if err != nil {
			return nil, fmt.Errorf("error fetching posts without impressions: %v", err)
		}
		defer remainingPostsCursor.Close(ctx)

		var remainingPosts []common.Post
		if err = remainingPostsCursor.All(ctx, &remainingPosts); err != nil {
			return nil, fmt.Errorf("error decoding remaining posts: %v", err)
		}

		for _, post := range remainingPosts {
			// Check if the user has already seen this post by checking impressions
			impresionFilter := bson.M{
				"post_id": post.ID,
				"user_id": userID,
			}
			count, err := impresionCollection.CountDocuments(ctx, impresionFilter)
			if err != nil {
				return nil, err
			}

			// If the user has seen this post, skip it
			if count > 0 {
				continue
			}

			// Create a new impression (upsert)
			impresion := PostImpression{
				PostID:    post.ID,
				UserID:    userID,
				CreatedAt: time.Now(),
			}
			err = s.UpsertPostImpression(impresion)
			if err != nil {
				return nil, err
			}

			// Increment the post's impression count
			_, err = postCollection.UpdateOne(
				ctx,
				bson.M{"_id": post.ID},
				bson.M{"$inc": bson.M{"impression": 1}}, // Increment the "impression" counter by 1
			)
			if err != nil {
				return nil, fmt.Errorf("error updating post impression counter: %v", err)
			}

			// Retrieve the user who created the post
			var user common.User
			err = userCollection.FindOne(ctx, bson.M{"_id": post.User_id}).Decode(&user)
			if err != nil {
				return nil, err
			}

			// Handle subscription and blur logic
			var subbed = true
			if post.User_id != userID {
				if post.Tier != 0 {
					userTier, err := s.GetSubscriptionTier(userID, post.User_id)
					if err != nil {
						return nil, err
					}
					if userTier < post.Tier && !s.IsAdmin(user) {
						post.Content = post.BluredContent
						subbed = false
					}
				}
			}

			// Combine post with user data
			postWithUser := PostWithUser{
				Post:       post,
				User:       user,
				Subscribed: subbed,
			}

			postsWithUsers = append(postsWithUsers, postWithUser)
		}
	}

	return postsWithUsers, nil
}

func (s *store) GetTopPostsWithMostImpressionsInLast24Hrs__(userID primitive.ObjectID, limit int) ([]PostWithUser, error) {
	impresionCollection := s.database.Collection("post-impression")
	postCollection := s.database.Collection("posts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Calculate the time 24 hours ago
	last24Hours := time.Now().Add(-24 * time.Hour)

	// Part 1: Aggregate posts with impressions from the last 24 hours but not seen by this user
	pipeline := mongo.Pipeline{
		// Match impressions created within the last 24 hours
		{{
			Key: "$match",
			Value: bson.D{
				{Key: "createdAt", Value: bson.D{{Key: "$gte", Value: last24Hours}}},
				{Key: "user_id", Value: bson.M{"$ne": userID}}, // Exclude impressions by this user
			},
		}},
		// Group by post_id and count the impressions for each post
		{{
			Key: "$group",
			Value: bson.D{
				{Key: "_id", Value: "$post_id"},
				{Key: "impressionCount", Value: bson.D{{Key: "$sum", Value: 1}}},
			},
		}},
		// Sort by impressionCount in descending order
		{{
			Key:   "$sort",
			Value: bson.D{{Key: "impressionCount", Value: -1}},
		}},
		// Limit the number of results
		{{
			Key:   "$limit",
			Value: int64(limit),
		}},
	}

	cursor, err := impresionCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("error fetching impressions: %v", err)
	}
	defer cursor.Close(ctx)

	var topPostsWithImpressions []struct {
		PostID          primitive.ObjectID `bson:"_id"`
		ImpressionCount int                `bson:"impressionCount"`
	}
	if err := cursor.All(ctx, &topPostsWithImpressions); err != nil {
		return nil, fmt.Errorf("error decoding impressions: %v", err)
	}

	// If no impressions are found, log it
	if len(topPostsWithImpressions) == 0 {
		log.Println("No impressions found in the last 24 hours")
	}

	// Step 2: Fetch the actual posts using the post IDs
	var postIDs []primitive.ObjectID
	for _, record := range topPostsWithImpressions {
		postIDs = append(postIDs, record.PostID)
	}

	var posts []common.Post

	if len(postIDs) > 0 {
		// Fetch posts that have impressions but exclude user's own posts
		postFilter := bson.M{
			"_id":     bson.M{"$in": postIDs},
			"hidden":  false,
			"user_id": bson.M{"$ne": userID}, // Ensure we're not fetching the user's own posts
		}
		postsCursor, err := postCollection.Find(ctx, postFilter)
		if err != nil {
			return nil, fmt.Errorf("error fetching posts: %v", err)
		}
		defer postsCursor.Close(ctx)

		if err = postsCursor.All(ctx, &posts); err != nil {
			return nil, fmt.Errorf("error decoding posts: %v", err)
		}
	}

	// Step 3: Prepare posts with user data
	var postsWithUsers []PostWithUser
	userCollection := s.database.Collection("users")

	for _, post := range posts {
		// Retrieve the user who created the post
		var postUser common.User
		err = userCollection.FindOne(ctx, bson.M{"_id": post.User_id}).Decode(&postUser)
		if err != nil {
			return nil, err
		}

		// Handle subscription and blur logic
		var subbed = true
		if post.User_id != userID {
			if post.Tier != 0 {
				userTier, err := s.GetSubscriptionTier(userID, post.User_id)
				if err != nil {
					return nil, err
				}

				user, err := s.GetUserByID(userID.Hex())
				if err != nil {
					return nil, err
				}

				if userTier < post.Tier && !s.IsAdmin(*user) {
					post.Content = post.BluredContent
					subbed = false
				}
			}
		}

		// Combine post with postUser data
		postWithUser := PostWithUser{
			Post:       post,
			User:       postUser,
			Subscribed: subbed,
		}

		postsWithUsers = append(postsWithUsers, postWithUser)
	}

	// Step 4: Fetch additional posts without impressions (if not enough posts were found)
	if len(postsWithUsers) < limit {
		remainingLimit := limit - len(postsWithUsers)

		noImpressionsFilter := bson.M{
			"hidden":  false,
			"user_id": bson.M{"$ne": userID}, // Exclude the user's own posts
		}

		// Fetch posts without impressions
		remainingPostsCursor, err := postCollection.Find(ctx, noImpressionsFilter, options.Find().SetLimit(int64(remainingLimit)))
		if err != nil {
			return nil, fmt.Errorf("error fetching posts without impressions: %v", err)
		}
		defer remainingPostsCursor.Close(ctx)

		var remainingPosts []common.Post
		if err = remainingPostsCursor.All(ctx, &remainingPosts); err != nil {
			return nil, fmt.Errorf("error decoding remaining posts: %v", err)
		}

		for _, post := range remainingPosts {
			// Retrieve the user who created the post
			var user common.User
			err = userCollection.FindOne(ctx, bson.M{"_id": post.User_id}).Decode(&user)
			if err != nil {
				return nil, err
			}

			// Handle subscription and blur logic
			var subbed = true
			if post.User_id != userID {
				if post.Tier != 0 {
					userTier, err := s.GetSubscriptionTier(userID, post.User_id)
					if err != nil {
						return nil, err
					}
					if userTier < post.Tier && !s.IsAdmin(user) {
						post.Content = post.BluredContent
						subbed = false
					}
				}
			}

			// Combine post with user data
			postWithUser := PostWithUser{
				Post:       post,
				User:       user,
				Subscribed: subbed,
			}

			postsWithUsers = append(postsWithUsers, postWithUser)
		}
	}

	return postsWithUsers, nil
}

func (s *store) GetTopPostsWithMostImpressionsInLast24Hrs___(userID primitive.ObjectID, limit int, distinctPostIDs []primitive.ObjectID) ([]PostWithUser, error) {
	impresionCollection := s.database.Collection("post-impression")
	postCollection := s.database.Collection("posts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Calculate the time 24 hours ago
	last24Hours := time.Now().Add(-24 * time.Hour)

	// Part 1: Aggregate posts with impressions from the last 24 hours but not seen by this user
	pipeline := mongo.Pipeline{
		// Match impressions created within the last 24 hours
		{{
			Key: "$match",
			Value: bson.D{
				{Key: "createdAt", Value: bson.D{{Key: "$gte", Value: last24Hours}}},
				{Key: "user_id", Value: bson.M{"$ne": userID}}, // Exclude impressions by this user
			},
		}},
		// Group by post_id and count the impressions for each post
		{{
			Key: "$group",
			Value: bson.D{
				{Key: "_id", Value: "$post_id"},
				{Key: "impressionCount", Value: bson.D{{Key: "$sum", Value: 1}}},
			},
		}},
		// Sort by impressionCount in descending order
		{{
			Key:   "$sort",
			Value: bson.D{{Key: "impressionCount", Value: -1}},
		}},
		// Limit the number of results
		{{
			Key:   "$limit",
			Value: int64(limit),
		}},
	}

	cursor, err := impresionCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("error fetching impressions: %v", err)
	}
	defer cursor.Close(ctx)

	var topPostsWithImpressions []struct {
		PostID          primitive.ObjectID `bson:"_id"`
		ImpressionCount int                `bson:"impressionCount"`
	}
	if err := cursor.All(ctx, &topPostsWithImpressions); err != nil {
		return nil, fmt.Errorf("error decoding impressions: %v", err)
	}

	// If no impressions are found, log it
	if len(topPostsWithImpressions) == 0 {
		log.Println("No impressions found in the last 24 hours")
	}

	// Step 2: Fetch the actual posts using the post IDs
	var postIDs []primitive.ObjectID
	for _, record := range topPostsWithImpressions {
		postIDs = append(postIDs, record.PostID)
	}

	// Ensure we exclude posts the user has already seen (distinctPostIDs)
	if len(distinctPostIDs) > 0 {
		postIDs = append(postIDs, distinctPostIDs...)
	}

	var posts []common.Post

	if len(postIDs) > 0 {
		// Fetch posts that have impressions but exclude user's own posts and already seen posts
		postFilter := bson.M{
			"_id":     bson.M{"$in": postIDs},
			"hidden":  false,
			"user_id": bson.M{"$ne": userID}, // Ensure we're not fetching the user's own posts // Exclude already seen posts
		}
		postsCursor, err := postCollection.Find(ctx, postFilter)
		if err != nil {
			return nil, fmt.Errorf("error fetching posts: %v", err)
		}
		defer postsCursor.Close(ctx)

		if err = postsCursor.All(ctx, &posts); err != nil {
			return nil, fmt.Errorf("error decoding posts: %v", err)
		}
	}

	// Step 3: Prepare posts with user data
	var postsWithUsers []PostWithUser
	userCollection := s.database.Collection("users")

	for _, post := range posts {
		// Retrieve the user who created the post
		var postUser common.User
		err = userCollection.FindOne(ctx, bson.M{"_id": post.User_id}).Decode(&postUser)
		if err != nil {
			return nil, err
		}

		// Handle subscription and blur logic
		var subbed = true
		if post.User_id != userID {
			if post.Tier != 0 {
				userTier, err := s.GetSubscriptionTier(userID, post.User_id)
				if err != nil {
					return nil, err
				}

				user, err := s.GetUserByID(userID.Hex())
				if err != nil {
					return nil, err
				}

				if userTier < post.Tier && !s.IsAdmin(*user) {
					post.Content = post.BluredContent
					subbed = false
				}
			}
		}

		// Combine post with postUser data
		postWithUser := PostWithUser{
			Post:       post,
			User:       postUser,
			Subscribed: subbed,
		}

		postsWithUsers = append(postsWithUsers, postWithUser)
	}

	// Step 4: Fetch additional posts without impressions (if not enough posts were found)
	if len(postsWithUsers) < limit {
		remainingLimit := limit - len(postsWithUsers)

		noImpressionsFilter := bson.M{
			"hidden":  false,
			"user_id": bson.M{"$ne": userID},           // Exclude the user's own posts
			"_id":     bson.M{"$nin": distinctPostIDs}, // Exclude already seen posts
		}

		// Fetch posts without impressions
		remainingPostsCursor, err := postCollection.Find(ctx, noImpressionsFilter, options.Find().SetLimit(int64(remainingLimit)))
		if err != nil {
			return nil, fmt.Errorf("error fetching posts without impressions: %v", err)
		}
		defer remainingPostsCursor.Close(ctx)

		var remainingPosts []common.Post
		if err = remainingPostsCursor.All(ctx, &remainingPosts); err != nil {
			return nil, fmt.Errorf("error decoding remaining posts: %v", err)
		}

		for _, post := range remainingPosts {
			// Retrieve the user who created the post
			var user common.User
			err = userCollection.FindOne(ctx, bson.M{"_id": post.User_id}).Decode(&user)
			if err != nil {
				return nil, err
			}

			// Handle subscription and blur logic
			var subbed = true
			if post.User_id != userID {
				if post.Tier != 0 {
					userTier, err := s.GetSubscriptionTier(userID, post.User_id)
					if err != nil {
						return nil, err
					}
					if userTier < post.Tier && !s.IsAdmin(user) {
						post.Content = post.BluredContent
						subbed = false
					}
				}
			}

			// Combine post with user data
			postWithUser := PostWithUser{
				Post:       post,
				User:       user,
				Subscribed: subbed,
			}

			postsWithUsers = append(postsWithUsers, postWithUser)
		}
	}

	return postsWithUsers, nil
}

func (s *store) GetTopPostsWithMostImpressionsInLast24Hrs(userID primitive.ObjectID, limit int, distinctPostIDs []interface{}) ([]PostWithUser, error) {
	impresionCollection := s.database.Collection("post-impression")
	postCollection := s.database.Collection("posts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	convertedDistinctIDs, err := convertToObjectIDArray(distinctPostIDs)
	if err != nil {
		return nil, fmt.Errorf("error converting distinctPostIDs: %v", err)
	}

	// Calculate the time 24 hours ago
	last24Hours := time.Now().Add(-24 * time.Hour)

	// Part 1: Aggregate posts with impressions from the last 24 hours but not seen by this user
	pipeline := mongo.Pipeline{
		// Match impressions created within the last 24 hours
		{{
			Key: "$match",
			Value: bson.D{
				{Key: "createdAt", Value: bson.D{{Key: "$gte", Value: last24Hours}}},
				{Key: "user_id", Value: bson.M{"$ne": userID}}, // Exclude impressions by this user
			},
		}},
		// Group by post_id and count the impressions for each post
		{{
			Key: "$group",
			Value: bson.D{
				{Key: "_id", Value: "$post_id"},
				{Key: "impressionCount", Value: bson.D{{Key: "$sum", Value: 1}}},
			},
		}},
		// Sort by impressionCount in descending order
		{{
			Key:   "$sort",
			Value: bson.D{{Key: "impressionCount", Value: -1}},
		}},
		// Limit the number of results
		{{
			Key:   "$limit",
			Value: int64(limit),
		}},
	}

	cursor, err := impresionCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("error fetching impressions: %v", err)
	}
	defer cursor.Close(ctx)

	var topPostsWithImpressions []struct {
		PostID          primitive.ObjectID `bson:"_id"`
		ImpressionCount int                `bson:"impressionCount"`
	}
	if err := cursor.All(ctx, &topPostsWithImpressions); err != nil {
		return nil, fmt.Errorf("error decoding impressions: %v", err)
	}

	// If no impressions are found, log it
	if len(topPostsWithImpressions) == 0 {
		log.Println("No impressions found in the last 24 hours")
	}

	// Step 2: Fetch the actual posts using the post IDs
	var postIDs []primitive.ObjectID
	for _, record := range topPostsWithImpressions {
		postIDs = append(postIDs, record.PostID)
	}

	var posts []common.Post

	if len(postIDs) > 0 {
		// Fetch posts that have impressions but exclude user's own posts and already seen posts
		postFilter := bson.M{
			"_id": bson.M{
				"$in":  postIDs,              // Include only posts with IDs in postIDs
				"$nin": convertedDistinctIDs, // Exclude posts the user has already seen
			},
			"hidden":  false,                 // Exclude hidden posts
			"user_id": bson.M{"$ne": userID}, // Exclude the user's own posts
		}

		postsCursor, err := postCollection.Find(ctx, postFilter)
		if err != nil {
			return nil, fmt.Errorf("error fetching posts: %v", err)
		}
		defer postsCursor.Close(ctx)

		if err = postsCursor.All(ctx, &posts); err != nil {
			return nil, fmt.Errorf("error decoding posts: %v", err)
		}
	}

	// Step 3: Prepare posts with user data
	var postsWithUsers []PostWithUser
	userCollection := s.database.Collection("users")

	for _, post := range posts {
		// Retrieve the user who created the post
		var postUser common.User
		err = userCollection.FindOne(ctx, bson.M{"_id": post.User_id}).Decode(&postUser)
		if err != nil {
			return nil, err
		}

		impresion := PostImpression{
			PostID:    post.ID,
			UserID:    userID,
			CreatedAt: time.Now(),
		}
		err = s.UpsertPostImpression(impresion)
		if err != nil {
			return nil, err
		}

		// Increment the post's impression count
		_, err = postCollection.UpdateOne(
			ctx,
			bson.M{"_id": post.ID},
			bson.M{"$inc": bson.M{"impression": 1}}, // Increment the "impression" counter by 1
		)
		if err != nil {
			return nil, fmt.Errorf("error updating post impression counter: %v", err)
		}

		// Handle subscription and blur logic
		var subbed = true
		if post.User_id != userID {
			if post.Tier != 0 {
				userTier, err := s.GetSubscriptionTier(userID, post.User_id)
				if err != nil {
					return nil, err
				}

				user, err := s.GetUserByID(userID.Hex())
				if err != nil {
					return nil, err
				}

				if userTier < post.Tier && !s.IsAdmin(*user) {
					post.Content = post.BluredContent
					subbed = false
				}
			}
		}

		// Combine post with postUser data
		postWithUser := PostWithUser{
			Post:       post,
			User:       postUser,
			Subscribed: subbed,
		}

		postsWithUsers = append(postsWithUsers, postWithUser)
	}

	distinctPostIDs, err = impresionCollection.Distinct(ctx, "post_id", bson.M{"user_id": userID})
	if err != nil {
		return nil, fmt.Errorf("error fetching distinct post IDs: %v", err)
	}

	// Step 4: Fetch additional posts without impressions (if not enough posts were found)
	if len(postsWithUsers) < limit {
		remainingLimit := limit - len(postsWithUsers)

		noImpressionsFilter := bson.M{
			"hidden":  false,
			"user_id": bson.M{"$ne": userID},           // Exclude the user's own posts
			"_id":     bson.M{"$nin": distinctPostIDs}, // Exclude already seen posts
		}

		// Fetch posts without impressions
		remainingPostsCursor, err := postCollection.Find(ctx, noImpressionsFilter, options.Find().SetLimit(int64(remainingLimit)))
		if err != nil {
			return nil, fmt.Errorf("error fetching posts without impressions: %v", err)
		}
		defer remainingPostsCursor.Close(ctx)

		var remainingPosts []common.Post
		if err = remainingPostsCursor.All(ctx, &remainingPosts); err != nil {
			return nil, fmt.Errorf("error decoding remaining posts: %v", err)
		}

		for _, post := range remainingPosts {
			// Retrieve the user who created the post
			var user common.User
			err = userCollection.FindOne(ctx, bson.M{"_id": post.User_id}).Decode(&user)
			if err != nil {
				return nil, err
			}

			impresion := PostImpression{
				PostID:    post.ID,
				UserID:    userID,
				CreatedAt: time.Now(),
			}
			err = s.UpsertPostImpression(impresion)
			if err != nil {
				return nil, err
			}

			// Increment the post's impression count
			_, err = postCollection.UpdateOne(
				ctx,
				bson.M{"_id": post.ID},
				bson.M{"$inc": bson.M{"impression": 1}}, // Increment the "impression" counter by 1
			)
			if err != nil {
				return nil, fmt.Errorf("error updating post impression counter: %v", err)
			}

			// Handle subscription and blur logic
			var subbed = true
			if post.User_id != userID {
				if post.Tier != 0 {
					userTier, err := s.GetSubscriptionTier(userID, post.User_id)
					if err != nil {
						return nil, err
					}
					if userTier < post.Tier && !s.IsAdmin(user) {
						post.Content = post.BluredContent
						subbed = false
					}
				}
			}

			// Combine post with user data
			postWithUser := PostWithUser{
				Post:       post,
				User:       user,
				Subscribed: subbed,
			}

			postsWithUsers = append(postsWithUsers, postWithUser)
		}
	}

	return postsWithUsers, nil
}

func convertToObjectIDArray(ids []interface{}) ([]primitive.ObjectID, error) {
	var objectIDs []primitive.ObjectID
	for _, id := range ids {
		// Ensure the id is of type primitive.ObjectID before appending
		oid, ok := id.(primitive.ObjectID)
		if !ok {
			return nil, fmt.Errorf("invalid ID type, expected primitive.ObjectID")
		}
		objectIDs = append(objectIDs, oid)
	}
	return objectIDs, nil
}
