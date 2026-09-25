package main

import (
	"context"
	"mime/multipart"
	"time"

	common "github.com/Eskiwi-Organization/infra/commons"
	pb "github.com/Eskiwi-Organization/infra/commons/api"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type WebStore interface {
	CreateChat(chat Chat) (error, bool)
	ChatExists(chat Chat) (bool, error)
	GetChatID(userID, creatorID primitive.ObjectID) (primitive.ObjectID, error)
	GetLatestsChats(page int, pageSize int, userID primitive.ObjectID, typeRequest bool) ([]ChatWithUser, error)
	GetLatestsChatsRequest(page int, pageSize int, userID primitive.ObjectID) ([]ChatWithUser, error)
	GetChat(chatID primitive.ObjectID, userID primitive.ObjectID) ([]ChatWithUser, error)
	CreateMessage(message MongoMessage) (primitive.ObjectID, error)
	UpdateMessageStatus(chatID primitive.ObjectID, lastMessage string) error
	CountUnreadMessages(chatID primitive.ObjectID, userID primitive.ObjectID) (int64, error)
	UpdateAndRetrievePaginatedMessages(chatID primitive.ObjectID, page int, pageSize int) ([]Message, error)
	ConvertToGrpcChat(chat *Chat) *pb.Chat
	ConvertToGrpcUser(user *common.User) *pb.User
	ConvertToGrpcMessage(message *Message) *pb.Message
	UploadMultiPartContent(files []*multipart.FileHeader) ([]string, error)
	UpdateChatRequestStatus(chatID primitive.ObjectID) error
	DeleteChat(chatID primitive.ObjectID) error
	UpdateMessageReadStatus(messageID primitive.ObjectID) error
	SendMessageNotification(UserID primitive.ObjectID, ChatID primitive.ObjectID, text string) error
	GetChatPriceAndRules(userID primitive.ObjectID) (*common.ChatsSettingsPayload, error)
	IsChatRequestTrueAndUserID(chatID primitive.ObjectID) (bool, primitive.ObjectID, primitive.ObjectID, error)
	CountUserMessagesInChat(chatID, userID primitive.ObjectID) (int64, error)
}

type WebService interface {
	CreateChat(context.Context) error
	CreateRoom(context.Context) error
	GetLatestsChats(context.Context) error
	GetLatestsRequestChats(context.Context) error
	GetChat(context.Context) error
	GetLatestsMessages(context.Context) error
	AcceptChatReq(context.Context) error
	RejectChatReq(context.Context) error
}

type CreateChatPayload struct {
	CreatorID primitive.ObjectID `json:"creator_id"   bson:"creator_id"`
}

type CreateRoomPayload struct {
	ID      primitive.ObjectID `json:"id"`
	Userid  primitive.ObjectID `json:"user_id"`
	Request bool               `json:"request"`
}

type JoinRoomReq struct {
	ClientID string `json:"client_id"`
	Username string `json:"username"`
}

type CreateChatResponse struct {
	ChatID primitive.ObjectID `json:"chat_id"`
}

type CreateRoomResponse struct {
	Created bool `json:"created"`
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

type MessagePayload struct {
	ID       primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	ChatID   primitive.ObjectID `json:"chat_id"   bson:"chat_id"`
	Gems     int                `json:"gems"`
	Text     string             `json:"text"`
	Contents []string           `json:"contents"`
	Error    bool               `json:"error"`
}

type ChatWithUser struct {
	Chat   Chat        `json:"chat"`
	User   common.User `json:"user"`
	Unread int         `json:"unread"`
}

type MongoMessage struct {
	ID        primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Text      string             `json:"text"` //text
	Chatid    primitive.ObjectID `json:"chat_id" bson:"chat_id"`
	Userid    primitive.ObjectID `json:"user_id" bson:"user_id"`
	Gems      int                `json:"gems"`
	CreatedAt time.Time          `json:"createdAt"  bson:"createdAt"`
	Read      bool               `json:"read" bson:"read"`
	Contents  []string           `json:"contents"`
}

type ChatImagesURLs struct {
	URLs []string `json:"Urls"`
}
