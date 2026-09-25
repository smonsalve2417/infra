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

	// Perform the delete operation
	result, err := collection.DeleteOne(ctx, bson.M{"_id": commentID})
	if err != nil {
		return err
	}

	// Check if a document was deleted
	if result.DeletedCount == 0 {
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
	err := postCollection.FindOne(ctx, bson.M{"_id": postID}).Decode(&post)
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

func (s *store) GetLatestsPosts(page int, pageSize int) ([]PostWithUser, error) {
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
		postWithUser := PostWithUser{
			Post: post,
			User: user,
		}
		postsWithUsers = append(postsWithUsers, postWithUser)
	}

	return postsWithUsers, nil
}

func (s *store) GetLatestsUserPosts(userID primitive.ObjectID, page int, pageSize int) ([]PostWithUser, error) {
	postCollection := s.database.Collection("posts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"user_id": userID, "hidden": false}

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
		postWithUser := PostWithUser{
			Post: post,
			User: user,
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
		UserXid:     post.User_id.Hex(),
		Content:     post.Content,
		Tags:        post.Tags,
		Gems:        int32(post.Gems),
		Likes:       int32(post.Likes),
		Comments:    int32(post.Comments),
		Shares:      int32(post.Shares),
		Saves:       int32(post.Saves),
		Created_At:  timestamppb.New(post.CreatedAt),
		Updated_At:  timestamppb.New(post.UpdatedAt),
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

func (s *store) ConvertToGrpcPublicUser(user *common.User) *pb.User {
	return &pb.User{
		Id:          user.ID.Hex(),
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		Username:    user.UserName,
		CreatedAt:   user.CreatedAt.String(),
		AvatarUri:   user.Avatar_uri,
		BannerUri:   user.Banner_uri,
		Followers:   int64(user.Followers),
		Following:   int64(user.Following),
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

func (s *store) GetPost(postID primitive.ObjectID) ([]PostWithUser, error) {
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
		var user common.User
		err := userCollection.FindOne(ctx, bson.M{"_id": post.User_id}).Decode(&user)
		if err != nil {
			return nil, err
		}
		postWithUser := PostWithUser{
			Post: post,
			User: user,
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
