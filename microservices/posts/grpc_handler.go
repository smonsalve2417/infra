package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	common "github.com/Eskiwi-Organization/infra/commons"
	pb "github.com/Eskiwi-Organization/infra/commons/api"
	broker "github.com/Eskiwi-Organization/infra/commons/broker"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc"
)

type grpcHandler struct {
	pb.UnimplementedPostServiceServer

	service PostService
	store   PostStore
	channel *amqp.Channel
}

func NewGRPCHandler(grpcServer *grpc.Server, service PostService, store PostStore, channel *amqp.Channel) {
	handler := &grpcHandler{
		service: service,
		store:   store,
		channel: channel,
	}
	pb.RegisterPostServiceServer(grpcServer, handler)

}

func (h *grpcHandler) CreatePost(ctx context.Context, p *pb.CreatePostRequest) (*pb.CreatePostResponse, error) {
	log.Printf("New post received! Post from: %v", p.UserID)

	user := p.GetUserID()
	objID, err := primitive.ObjectIDFromHex(user)
	if err != nil {
		o := &pb.CreatePostResponse{Status: "error creating post process: " + err.Error()}
		return o, err
	}

	contentUrls, err := h.store.UploadContent(p.Post.Content)
	if err != nil {
		o := &pb.CreatePostResponse{Status: "error creating post process: " + err.Error()}
		return o, err
	}

	_, err = h.store.CreatePosts(common.Post{
		Title:       p.Post.Title,
		Description: p.Post.Description,
		User_id:     objID,
		Content:     contentUrls,
		Tags:        p.Post.Tags,
		Gems:        0,
		Likes:       0,
		Comments:    0,
		Shares:      0,
		Saves:       0,
		CreatedAt:   time.Now(),
	})

	if err != nil {
		o := &pb.CreatePostResponse{Status: "error creating post process: " + err.Error()}
		return o, err
	}

	o := &pb.CreatePostResponse{Status: "successfull post creation"}
	return o, nil
}

func (h *grpcHandler) DeletePost(ctx context.Context, p *pb.DeletePostRequest) (*pb.DeletePostResponse, error) {
	log.Printf("New post deletion received! Post from: %v", p.PostID)

	postObjID, err := primitive.ObjectIDFromHex(p.PostID)
	if err != nil {
		o := &pb.DeletePostResponse{Status: "error creating post process: " + err.Error()}
		return o, err
	}

	userObjID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		o := &pb.DeletePostResponse{Status: "error creating post process: " + err.Error()}
		return o, err
	}

	post, err := h.store.GetPostByID(postObjID)
	if err != nil {
		o := &pb.DeletePostResponse{Status: "error getting post: " + err.Error()}
		return o, err
	}

	if post.User_id != userObjID {
		o := &pb.DeletePostResponse{Status: "no authorization to delete"}
		return o, nil
	}

	err = h.store.DeletePost(postObjID)
	if err != nil {
		o := &pb.DeletePostResponse{Status: "error deleting comment process: " + err.Error()}
		return o, err
	}

	o := &pb.DeletePostResponse{Status: "successfull post deletion"}
	return o, nil
}

func (h *grpcHandler) LikePost(ctx context.Context, p *pb.LikePostRequest) (*pb.LikePostResponse, error) {
	log.Printf("New like received! Post from: %v", p.PostID)

	objUserID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		o := &pb.LikePostResponse{Status: "error on User objectID convertion process: " + err.Error()}
		return o, err
	}

	objPostID, err := primitive.ObjectIDFromHex(p.PostID)
	if err != nil {
		o := &pb.LikePostResponse{Status: "error on Post objectID convertion process: " + err.Error()}
		return o, err
	}

	err = h.store.CreatePostLike(PostLike{
		User_id:   objUserID,
		Post_id:   objPostID,
		CreatedAt: time.Now(),
	})
	if err != nil {
		o := &pb.LikePostResponse{Status: "error uploading like to mongo like-post: " + err.Error()}
		return o, err
	}

	err = h.store.IncrementPostLike(objPostID)
	if err != nil {
		o := &pb.LikePostResponse{Status: "error uploading like increment to mongo post: " + err.Error()}
		return o, err
	}

	var payload common.LikeNotificationPayload

	payload.Post_id = objPostID
	payload.User_id = objUserID

	post, err := h.store.GetPostByID(objPostID)
	if err != nil {
		log.Printf("Post not found: %v", err)
		return nil, err
	}

	if objUserID != post.User_id {

		marshalledRequest, err := json.Marshal(payload)
		if err != nil {
			log.Printf("Internal server error: %v", err)
			return nil, err
		}

		q, err := h.channel.QueueDeclare(broker.SendLikeNotificationCreatedEvent, true, false, false, false, nil)
		if err != nil {
			log.Printf("Internal server error: %v", err)
			return nil, err
		}

		h.channel.PublishWithContext(ctx, "", q.Name, false, false, amqp.Publishing{
			ContentType:  "application/json",
			Body:         marshalledRequest,
			DeliveryMode: amqp.Persistent,
		})
	}

	o := &pb.LikePostResponse{Status: "successfull post like"}
	return o, nil
}

func (h *grpcHandler) UnlikePost(ctx context.Context, p *pb.LikePostRequest) (*pb.LikePostResponse, error) {
	log.Printf("New unlike received! Post from: %v", p.PostID)

	objUserID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		o := &pb.LikePostResponse{Status: "error on User objectID convertion process: " + err.Error()}
		return o, err
	}

	objPostID, err := primitive.ObjectIDFromHex(p.PostID)
	if err != nil {
		o := &pb.LikePostResponse{Status: "error on Post objectID convertion process: " + err.Error()}
		return o, err
	}

	err = h.store.DeletePostLike(objUserID, objPostID)
	if err != nil {
		o := &pb.LikePostResponse{Status: "error deleting unlike from mongo like-post: " + err.Error()}
		return o, err
	}

	err = h.store.DecrementPostLike(objPostID)
	if err != nil {
		o := &pb.LikePostResponse{Status: "error uploading like decrement to mongo post: " + err.Error()}
		return o, err
	}

	o := &pb.LikePostResponse{Status: "successfull unlike"}
	return o, nil
}

func (h *grpcHandler) CreateComment(ctx context.Context, p *pb.CreateCommentRequest) (*pb.CreateCommentResponse, error) {
	log.Printf("New comment received! Post from: %v", p.Comment.UserID)

	commentID := primitive.NewObjectID()

	if p.Comment.Gems == 0 {
		return h.CreateCommentConsumer(ctx, p, commentID)
	}

	userObjID, err := primitive.ObjectIDFromHex(p.Comment.UserID)
	if err != nil {
		o := &pb.CreateCommentResponse{Status: "error creating post process: " + err.Error()}
		return o, err
	}

	postObjID, err := primitive.ObjectIDFromHex(p.Comment.PostID)
	if err != nil {
		o := &pb.CreateCommentResponse{Status: "error creating post process: " + err.Error()}
		return o, err
	}

	//all this after the confirmation

	var payload1 common.CreateCommentDonationPayload

	payload1.Post_id = postObjID
	payload1.User_id = userObjID
	payload1.Comment_id = commentID
	payload1.Gems = int(p.Comment.Gems)

	log.Printf("amount of gems in post create comment: %v", payload1.Gems)

	marshalledRequest, err := json.Marshal(payload1)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		return nil, err
	}
	//queue to send to payments to process the creation of a comment
	q, err := h.channel.QueueDeclare(broker.SendDonationCommentCreatedEvent, true, false, false, false, nil)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		return nil, err
	}

	// Declare the callback queue for receiving comment confirmation
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
					// Proceed to create the comment
					return h.CreateCommentConsumer(ctx, p, commentID)
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

func (h *grpcHandler) CreateCommentConsumer(ctx context.Context, p *pb.CreateCommentRequest, commentID primitive.ObjectID) (*pb.CreateCommentResponse, error) {
	log.Printf("New comment received! Post from: %v", p.Comment.UserID)

	userObjID, err := primitive.ObjectIDFromHex(p.Comment.UserID)
	if err != nil {
		o := &pb.CreateCommentResponse{Status: "error creating post process: " + err.Error()}
		return o, err
	}

	postObjID, err := primitive.ObjectIDFromHex(p.Comment.PostID)
	if err != nil {
		o := &pb.CreateCommentResponse{Status: "error creating post process: " + err.Error()}
		return o, err
	}

	//all this after the confirmation
	commentObjtID, err := h.store.CreateComments(common.Comment{
		ID:        commentID,
		UserID:    userObjID,
		PostID:    postObjID,
		Text:      p.Comment.Text,
		Gems:      p.Comment.Gems,
		Likes:     0,
		Replies:   0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	if err != nil {
		o := &pb.CreateCommentResponse{Status: "error creating post process: " + err.Error()}
		return o, err
	}
	err = h.store.UpdateCommentCounter(postObjID, 1)
	if err != nil {
		o := &pb.CreateCommentResponse{Status: "error creating post process: " + err.Error()}
		return o, err
	}

	var payload common.LikeNotificationPayload

	payload.Post_id = postObjID
	payload.User_id = userObjID
	payload.Comment_id = commentObjtID

	post, err := h.store.GetPostByID(postObjID)
	if err != nil {
		log.Printf("Post not found: %v", err)
		return nil, err
	}

	if userObjID != post.User_id {

		marshalledRequest, err := json.Marshal(payload)
		if err != nil {
			log.Printf("Internal server error: %v", err)
			return nil, err
		}

		q, err := h.channel.QueueDeclare(broker.SendCommentNotificationCreatedEvent, true, false, false, false, nil)
		if err != nil {
			log.Printf("Internal server error: %v", err)
			return nil, err
		}

		h.channel.PublishWithContext(ctx, "", q.Name, false, false, amqp.Publishing{
			ContentType:  "application/json",
			Body:         marshalledRequest,
			DeliveryMode: amqp.Persistent,
		})
	}

	o := &pb.CreateCommentResponse{Status: "successfull comment creation"}
	return o, nil
}

func (h *grpcHandler) DeleteComment(ctx context.Context, p *pb.DeleteCommentRequest) (*pb.DeleteCommentResponse, error) {
	log.Printf("New delete comment received! Post from: %v", p.CommentID)

	commentObjID, err := primitive.ObjectIDFromHex(p.CommentID)
	if err != nil {
		o := &pb.DeleteCommentResponse{Status: "error creating post process: " + err.Error()}
		return o, err
	}

	err = h.store.DeleteCommentByID(commentObjID)
	if err != nil {
		o := &pb.DeleteCommentResponse{Status: "error deleting comment process: " + err.Error()}
		return o, err
	}
	err = h.store.DecreaseCommentCounterByCommentID(commentObjID, -1)
	if err != nil {
		o := &pb.DeleteCommentResponse{Status: "error deleting comment process: " + err.Error()}
		return o, err
	}

	o := &pb.DeleteCommentResponse{Status: "successfull comment deletion"}
	return o, nil
}

func (h *grpcHandler) LikeComment(ctx context.Context, p *pb.LikeCommentRequest) (*pb.LikeCommentResponse, error) {
	log.Printf("New like received! Comment from: %v", p.CommentID)

	objUserID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		o := &pb.LikeCommentResponse{Status: "error on User objectID convertion process: " + err.Error()}
		return o, err
	}

	objCommentID, err := primitive.ObjectIDFromHex(p.CommentID)
	if err != nil {
		o := &pb.LikeCommentResponse{Status: "error on Post objectID convertion process: " + err.Error()}
		return o, err
	}

	err = h.store.CreateCommentLike(CommentLike{
		User_id:    objUserID,
		Comment_id: objCommentID,
		CreatedAt:  time.Now(),
	})
	if err != nil {
		o := &pb.LikeCommentResponse{Status: "error uploading like to mongo like-post: " + err.Error()}
		return o, err
	}

	err = h.store.UpdateCommentLikeCounter(objCommentID, 1)
	if err != nil {
		o := &pb.LikeCommentResponse{Status: "error uploading like increment to mongo post: " + err.Error()}
		return o, err
	}

	o := &pb.LikeCommentResponse{Status: "successfull post like"}
	return o, nil
}

func (h *grpcHandler) UnLikeComment(ctx context.Context, p *pb.LikeCommentRequest) (*pb.LikeCommentResponse, error) {
	log.Printf("New unlike received! comment from: %v", p.CommentID)

	objUserID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		o := &pb.LikeCommentResponse{Status: "error on User objectID convertion process: " + err.Error()}
		return o, err
	}

	objCommentID, err := primitive.ObjectIDFromHex(p.CommentID)
	if err != nil {
		o := &pb.LikeCommentResponse{Status: "error on Post objectID convertion process: " + err.Error()}
		return o, err
	}

	err = h.store.DeleteCommentLike(objUserID, objCommentID)
	if err != nil {
		o := &pb.LikeCommentResponse{Status: "error deleting unlike from mongo like-post: " + err.Error()}
		return o, err
	}

	err = h.store.UpdateCommentLikeCounter(objCommentID, -1)
	if err != nil {
		o := &pb.LikeCommentResponse{Status: "error uploading like decrement to mongo post: " + err.Error()}
		return o, err
	}

	o := &pb.LikeCommentResponse{Status: "successfull unlike"}
	return o, nil
}

func (h *grpcHandler) GetUserCommentLike(ctx context.Context, p *pb.GetUserCommentLikeRequest) (*pb.GetUserCommentLikeResponse, error) {
	log.Printf("New GetUserLike received!")

	objUserID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		log.Printf("error en user")
		return nil, err
	}

	objCommentID, err := primitive.ObjectIDFromHex(p.CommentID)
	if err != nil {
		log.Printf("error en comment")
		return nil, err
	}

	var commentLike CommentLike

	commentLike.Comment_id = objCommentID
	commentLike.User_id = objUserID

	liked, err := h.store.CheckCommentLike(commentLike)
	if err != nil {
		log.Printf("error en comment")
		return nil, err
	}

	o := &pb.GetUserCommentLikeResponse{
		Liked: liked,
	}

	return o, nil
}

func (h *grpcHandler) CreateReply(ctx context.Context, p *pb.CreateReplyRequest) (*pb.CreateReplyResponse, error) {
	log.Printf("New reply received! Post from: %v", p.Reply.UserID)

	userObjID, err := primitive.ObjectIDFromHex(p.Reply.UserID)
	if err != nil {
		o := &pb.CreateReplyResponse{Status: "error creating post process: " + err.Error()}
		return o, err
	}

	commentObjID, err := primitive.ObjectIDFromHex(p.Reply.CommentID)
	if err != nil {
		o := &pb.CreateReplyResponse{Status: "error creating post process: " + err.Error()}
		return o, err
	}

	err = h.store.CreateReplies(common.Reply{
		UserID:    userObjID,
		CommentID: commentObjID,
		Text:      p.Reply.Text,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	if err != nil {
		o := &pb.CreateReplyResponse{Status: "error creating post process: " + err.Error()}
		return o, err
	}

	//subir numero de replies del comentario
	err = h.store.UpdateReplyCounter(commentObjID, 1)
	if err != nil {
		o := &pb.CreateReplyResponse{Status: "error creating post process: " + err.Error()}
		return o, err
	}

	o := &pb.CreateReplyResponse{Status: "successfull comment creation"}
	return o, nil
}

func (h *grpcHandler) DeleteReply(ctx context.Context, p *pb.DeleteReplyRequest) (*pb.DeleteReplyResponse, error) {
	log.Printf("New delete reply received! reply from: %v", p.ReplyID)

	replyObjID, err := primitive.ObjectIDFromHex(p.ReplyID)
	if err != nil {
		o := &pb.DeleteReplyResponse{Status: "error creating post process: " + err.Error()}
		return o, err
	}

	commentObjID, err := primitive.ObjectIDFromHex(p.CommentID)
	if err != nil {
		o := &pb.DeleteReplyResponse{Status: "error creating post process: " + err.Error()}
		return o, err
	}

	err = h.store.DeleteRepliesByID(replyObjID)
	if err != nil {
		o := &pb.DeleteReplyResponse{Status: "error deleting comment process: " + err.Error()}
		return o, err
	}

	err = h.store.UpdateReplyCounter(commentObjID, -1)
	if err != nil {
		o := &pb.DeleteReplyResponse{Status: "error deleting comment process: " + err.Error()}
		return o, err
	}

	o := &pb.DeleteReplyResponse{Status: "successfull comment deletion"}
	return o, nil
}

func (h *grpcHandler) GetLatestPosts(ctx context.Context, p *pb.GetLatestPostsRequest) (*pb.GetLatestPostsResponse, error) {
	log.Printf("New GetLatestPosts received!")

	list, err := h.store.GetLatestsPosts(int(p.Page), 10)
	if err != nil {
		o := &pb.GetLatestPostsResponse{}
		return o, err
	}

	var response []*pb.PostWithUser

	for _, postwithuser := range list {
		grpcPost := h.store.ConvertToGrpcPost(&postwithuser.Post)
		grpcUser := h.store.ConvertToGrpcUser(&postwithuser.User)
		o := &pb.PostWithUser{
			Post: grpcPost,
			User: grpcUser,
		}
		response = append(response, o)
	}

	o := &pb.GetLatestPostsResponse{
		Posts: response,
	}

	return o, nil
}

func (h *grpcHandler) GetLatestUserPosts(ctx context.Context, p *pb.GetLatestUserPostsRequest) (*pb.GetLatestUserPostsResponse, error) {
	log.Printf("New GetLatestUserPosts received!")

	objUserID, err := primitive.ObjectIDFromHex(p.TargetID)
	if err != nil {
		o := &pb.GetLatestUserPostsResponse{}
		return o, err
	}

	list, err := h.store.GetLatestsUserPosts(objUserID, int(p.Page), 10)
	if err != nil {
		o := &pb.GetLatestUserPostsResponse{}
		return o, err
	}

	var response []*pb.PostWithUser

	for _, postwithuser := range list {
		grpcPost := h.store.ConvertToGrpcPost(&postwithuser.Post)
		grpcUser := h.store.ConvertToGrpcUser(&postwithuser.User)
		o := &pb.PostWithUser{
			Post: grpcPost,
			User: grpcUser,
		}
		response = append(response, o)
	}

	o := &pb.GetLatestUserPostsResponse{
		Posts: response,
	}

	return o, nil
}

func (h *grpcHandler) GetUserLike(ctx context.Context, p *pb.GetUserLikeRequest) (*pb.GetUserLikeResponse, error) {
	log.Printf("New GetUserLike received!")

	objUserID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		return nil, err
	}

	objPostID, err := primitive.ObjectIDFromHex(p.PostID)
	if err != nil {
		return nil, err
	}

	var postLike PostLike

	postLike.Post_id = objPostID
	postLike.User_id = objUserID

	liked, err := h.store.CheckPostLike(postLike)
	if err != nil {
		return nil, err
	}

	o := &pb.GetUserLikeResponse{
		Liked: liked,
	}

	return o, nil
}

func (h *grpcHandler) GetLatestComments(ctx context.Context, p *pb.GetLatestCommentsRequest) (*pb.GetLatestCommentsResponse, error) {
	log.Printf("New GetLatestComments received!")

	postObjID, err := primitive.ObjectIDFromHex(p.PostID)
	if err != nil {
		o := &pb.GetLatestCommentsResponse{}
		return o, err
	}

	userObjID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		o := &pb.GetLatestCommentsResponse{}
		return o, err
	}

	list, err := h.store.GetLatestComments(postObjID, userObjID, int(p.Page), 10)
	if err != nil {
		o := &pb.GetLatestCommentsResponse{}
		return o, err
	}

	var response []*pb.CommentWithUser

	for _, commentwithuser := range list {
		grpcComment := h.store.ConvertToGrpcComment(&commentwithuser.Comment)
		grpcUser := h.store.ConvertToGrpcUser(&commentwithuser.User)
		o := &pb.CommentWithUser{
			Comment: grpcComment,
			User:    grpcUser,
		}
		response = append(response, o)
	}

	o := &pb.GetLatestCommentsResponse{
		Comments: response,
	}

	return o, nil
}

func (h *grpcHandler) GetLatestReplies(ctx context.Context, p *pb.GetLatestRepliesRequest) (*pb.GetLatestRepliesResponse, error) {
	log.Printf("New GetLatestPosts received!")

	commentObjID, err := primitive.ObjectIDFromHex(p.CommentID)
	if err != nil {
		o := &pb.GetLatestRepliesResponse{}
		return o, err
	}

	list, err := h.store.GetLatestReplies(commentObjID, int(p.Page), 3)
	if err != nil {
		o := &pb.GetLatestRepliesResponse{}
		return o, err
	}

	var response []*pb.ReplyWithUser

	for _, replywithuser := range list {
		grpcReply := h.store.ConvertToGrpcReply(&replywithuser.Reply)
		grpcUser := h.store.ConvertToGrpcUser(&replywithuser.User)
		o := &pb.ReplyWithUser{
			Reply: grpcReply,
			User:  grpcUser,
		}
		response = append(response, o)
	}

	o := &pb.GetLatestRepliesResponse{
		Replies: response,
	}

	return o, nil
}

func (h *grpcHandler) GetPost(ctx context.Context, p *pb.GetPostRequest) (*pb.GetLatestPostsResponse, error) {
	log.Printf("New GetLatestPosts received!")

	postObjID, err := primitive.ObjectIDFromHex(p.PostID)
	if err != nil {
		o := &pb.GetLatestPostsResponse{}
		return o, err
	}

	list, err := h.store.GetPost(postObjID)
	if err != nil {
		o := &pb.GetLatestPostsResponse{}
		return o, err
	}

	var response []*pb.PostWithUser

	for _, postwithuser := range list {
		grpcPost := h.store.ConvertToGrpcPost(&postwithuser.Post)
		grpcUser := h.store.ConvertToGrpcUser(&postwithuser.User)
		o := &pb.PostWithUser{
			Post: grpcPost,
			User: grpcUser,
		}
		response = append(response, o)
	}

	o := &pb.GetLatestPostsResponse{
		Posts: response,
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
