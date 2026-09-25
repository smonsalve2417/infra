package main

import (
	"context"
	"mime/multipart"
	"time"

	common "github.com/Eskiwi-Organization/infra/commons"
	pb "github.com/Eskiwi-Organization/infra/commons/api"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PostService interface {
	CreatePost(context.Context) error
}

type PostStore interface {
	CreatePosts(post common.Post) (primitive.ObjectID, error)
	DeletePost(postID primitive.ObjectID) error
	CreatePostLike(postLike PostLike) error
	DeletePostLike(userID primitive.ObjectID, postID primitive.ObjectID) error
	IncrementPostLike(postID primitive.ObjectID) error
	DecrementPostLike(postID primitive.ObjectID) error
	CreateComments(comment common.Comment) (primitive.ObjectID, error)
	DeleteCommentByID(commentID primitive.ObjectID) error
	CreateCommentLike(commentLike CommentLike) error
	UpdateCommentLikeCounter(commentID primitive.ObjectID, increaseDecrease int) error
	CreateReplies(reply common.Reply) error
	DeleteRepliesByID(replyID primitive.ObjectID) error
	UpdateReplyCounter(commentID primitive.ObjectID, increaseDecrease int) error
	UpdateCommentCounter(postID primitive.ObjectID, increaseDecrease int) error
	DecreaseCommentCounterByCommentID(commentID primitive.ObjectID, increaseDecrease int) error
	UpdateLikeCounter(commentID primitive.ObjectID, increaseDecrease int) error
	GetLatestComments(postID primitive.ObjectID, userID primitive.ObjectID, page int, pageSize int) ([]CommentWithUser, error)
	UploadContent(encodedContent []string) ([]string, error)
	UploadMultiPartContent(files []*multipart.FileHeader) ([]string, error)
	GetLatestsPosts(page int, pageSize int) ([]PostWithUser, error)
	GetLatestsUserPosts(userID primitive.ObjectID, page int, pageSize int) ([]PostWithUser, error)
	ConvertToGrpcPost(post *common.Post) *pb.Post
	ConvertToGrpcComment(comment *common.Comment) *pb.Comment
	ConvertToGrpcReply(reply *common.Reply) *pb.Reply
	ConvertToGrpcUser(user *common.User) *pb.User
	ConvertToGrpcPublicUser(user *common.User) *pb.User
	CheckPostLike(postLike PostLike) (bool, error)
	GetLatestReplies(commentID primitive.ObjectID, page int, pageSize int) ([]ReplyWithUser, error)
	CheckCommentLike(commentLike CommentLike) (bool, error)
	DeleteCommentLike(userID primitive.ObjectID, commentID primitive.ObjectID) error
	GetPost(postID primitive.ObjectID) ([]PostWithUser, error)
	GetPostByID(postID primitive.ObjectID) (*common.Post, error)
}

type PostLike struct {
	ID        primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	User_id   primitive.ObjectID `json:"user_id" bson:"user_id"`
	Post_id   primitive.ObjectID `json:"post_id" bson:"post_id"`
	CreatedAt time.Time          `json:"createdAt"`
}

type CommentLike struct {
	ID         primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	User_id    primitive.ObjectID `json:"user_id" bson:"user_id"`
	Comment_id primitive.ObjectID `json:"comment_id" bson:"comment_id"`
	CreatedAt  time.Time          `json:"createdAt"`
}

type PostWithUser struct {
	Post common.Post `json:"post"`
	User common.User `json:"user"`
}

type CommentWithUser struct {
	Comment common.Comment `json:"comment"`
	User    common.User    `json:"user"`
}

type ReplyWithUser struct {
	Reply common.Reply `json:"reply"`
	User  common.User  `json:"user"`
}
