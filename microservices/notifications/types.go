package main

import (
	"context"
	"time"

	common "github.com/Eskiwi-Organization/infra/commons"
	pb "github.com/Eskiwi-Organization/infra/commons/api"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type NotificationService interface {
	SendUserCode(context.Context, *pb.SendUserCodeRequest) error
	CreatePostNotification(ctx context.Context, post common.Post) error
	CreatePostLikeNotification(ctx context.Context, payload common.LikeNotificationPayload) error
	CreatePostCommentNotification(ctx context.Context, payload common.LikeNotificationPayload) error
	CreateUserFollowNotification(ctx context.Context, payload common.FollowNotificationPayload) error
	CreateChatmsgNotification(ctx context.Context, payload common.ChatNotificationPayload) error

	GetLatestNotifications(context.Context) error
}

type NotificationStore interface {
	Create(context.Context) error
	UpdateUserCode(email string, newCode string) error
	CreatePostNotifications(postID primitive.ObjectID, userID primitive.ObjectID, notificationType string) error
	GetFollowersByUserID(userID primitive.ObjectID) ([]primitive.ObjectID, error)
	GetUserByID(userID primitive.ObjectID) (*common.PublicUser, error)
	GetPostByID(postID primitive.ObjectID) (*common.Post, error)
	GetPushTokenByUserID(userID primitive.ObjectID) ([]string, error)
	GetLatestsNotifications(page int, pageSize int, userID primitive.ObjectID) ([]NotificationMongo, error)
	CreatePostLikeNotifications(postID primitive.ObjectID, userID primitive.ObjectID, notificationType string) error
	IncrementPendingNotifications(userID primitive.ObjectID) error
	CreatePostCommentNotifications(postID primitive.ObjectID, commentID primitive.ObjectID, userID primitive.ObjectID, notificationType string) error
	GetUserNotificationSettings(userID primitive.ObjectID) (*common.NotificationsSettingsPayload, error)
	CreateUserFollowNotification(CreatorID primitive.ObjectID, userID primitive.ObjectID, notificationType string) error
	CreateChatmsgNotification(ChatID primitive.ObjectID, userID primitive.ObjectID, text string) error
	UpdateNotificationCounter(userID primitive.ObjectID) error
}

type PostWithUser struct {
	Post common.Post `json:"post"`
	User common.User `json:"user"`
}

type Notification struct {
	User            common.User     `bson:"user"`
	Post            common.Post     `bson:"post"`
	CreatedAt       time.Time       `bson:"createdAt"`
	DestinationUser DestinationUser `bson:"destinationUser"`
	Type            string          `bson:"type"`
}

type DestinationUser struct {
	ID     primitive.ObjectID `bson:"id"`
	IsRead bool               `bson:"isRead"`
}

type UserPushToken struct {
	ID        primitive.ObjectID `bson:"_id"`
	PushToken []string           `bson:"expoToken"`
}

type NotificationMongo struct {
	ID              primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	User            common.User        `json:"user" bson:"user"`
	Post            common.Post        `json:"post" bson:"post"`
	Comment         common.Comment     `json:"comment" bson:"comment"`
	CreatedAt       time.Time          `json:"createdAt" bson:"createdAt"`
	DestinationUser DestinationUser    `json:"destinationUser" bson:"destinationUser"`
	IsRead          bool               `json:"isRead" bson:"isRead"`
	Type            string             `json:"type" bson:"type"` // Notification type: "Follow", "Like", "Comment", "Message", "newPost"
}

type Chat struct {
	ID          primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	CreatorID   primitive.ObjectID `json:"creator_id"    bson:"creator_id"`
	UserID      primitive.ObjectID `json:"user_id"       bson:"user_id"`
	CreatedAt   time.Time          `json:"createdAt"  bson:"createdAt"`
	Request     bool               `json:"request"  bson:"request"`
	LastMessage string             `json:"last_message"  bson:"last_message"`
	LastSentAt  time.Time          `json:"lastSentAt"  bson:"lastSentAt"`
}
