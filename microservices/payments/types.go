package main

import (
	"context"
	"time"

	common "github.com/Eskiwi-Organization/infra/commons"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PaymentsService interface {
	RegisterComment(ctx context.Context, comment common.Comment) error
	RegisterChat(ctx context.Context, ChatCreation common.CreateChatDonationPayload) error
	AcceptChat(ctx context.Context, ChatCreation common.AcceptChatPayload) error
	DenyChat(ctx context.Context, ChatCreation common.AcceptChatPayload) error
	RegisterChatMessage(ctx context.Context, chatMessage common.GemsOnChatPayload) error
	//Grpc
	GetAvailableSubscriptionGroup(context.Context) error
	CreateSubscribeToCreator(context.Context) error
}

type PaymentsStore interface {
	CreateUser(user *common.User) error
	UserExists(email string) (bool, error)
	CreateGemDonation(donation *GemDonation) error
	GetPostByID(postID primitive.ObjectID) (*common.Post, error)
	GetUserByIDsql(userID string) (*common.User, error)
	UpdateUserGems(email string, newGems int) error
	GetUserByID(id string) (*common.User, error)
	CreateChatGem(chatGem *ChatGem) error
	ChatGemExists(senderID string, receiverID string) (bool, error)
	UpdateUserGemsMongo(userID primitive.ObjectID, gems int) error
	GetParticipantsByChatID(chatID primitive.ObjectID) (primitive.ObjectID, primitive.ObjectID, error)
	FindAndAcceptChatGem(senderID, receiverID string) (int, error)
	CreateMessageGem(ctx context.Context, messageGem *MessageGem) error
	CreateGemPurchase(purchase *GemPurchase) error
	UpdatePostGemsMongo(postID primitive.ObjectID, gems int) error
	GetChatGemAmount(senderID, receiverID string) (int, error)
	DenyChatGem(chatID string) error
	GetProductIDsByUserID(userID primitive.ObjectID) ([]string, error)
	ExtractGroupNumber(word string) (string, error)
	FindSmallestMissingNumberFromProductIDs(productIDs []string) (int, error)
	GetSubscriptionTierByCreatorIDAndTier(creatorID primitive.ObjectID, tier int) (*common.SubscriptionTiers, error)
	CreateSubscriptionFromTier(userID, creatorID primitive.ObjectID, tier common.SubscriptionTiers, productID string) (primitive.ObjectID, error)
	ActivateSubscription(userID primitive.ObjectID, productID string) error
	RenewActivateSubscription(userID primitive.ObjectID, productID string) error
	CancelActivateSubscription(userID primitive.ObjectID, productID string) error
	CreateSubPurchase(purchase *SubPurchase) error
	GetSubscription(userID primitive.ObjectID, productID string) (*common.Subscription, error)
}

type PostWithUser struct {
	Post common.Post `json:"post"`
	User common.User `json:"user"`
}

type GemDonation struct {
	TransactionID int64     `json:"transaction_id"`
	SenderID      string    `json:"sender_id"`
	ReceiverID    string    `json:"receiver_id"`
	PostID        string    `json:"post_id"`
	CommentID     string    `json:"comment_id"`
	CreatedAt     time.Time `json:"created_at"`
	Valid         bool      `json:"valid"`
	GemAmount     int       `json:"gem_amount"`
}

type ChatGem struct {
	TransactionID int64     `bson:"_id" json:"transaction_id"` // MongoDB will assign this automatically
	ChatID        string    `json:"chat_id" db:"chat_id"`
	SenderID      string    `bson:"sender_id" json:"sender_id"`
	ReceiverID    string    `bson:"receiver_id" json:"receiver_id"`
	CreatedAt     time.Time `bson:"created_at" json:"created_at"`
	Valid         bool      `bson:"valid" json:"valid"`
	Denied        bool      `bson:"denied" json:"denied"`
	GemAmount     int       `bson:"gem_amount" json:"gem_amount"`
	Accepted      bool      `bson:"accepted" json:"accepted"`
}

type Config struct {
	PublicHost string
	Port       string
	DBUser     string
	DBPassword string
	DBAddress  string
	DBName     string
}

type MessageGem struct {
	TransactionID int64     `json:"transaction_id" db:"transaction_id"` // Auto-incremented in the DB
	ChatID        string    `json:"chat_id" db:"chat_id"`               // New field for chat ID
	SenderID      string    `json:"sender_id" db:"sender_id"`
	ReceiverID    string    `json:"receiver_id" db:"receiver_id"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"` // Automatically handled by DB
	Valid         bool      `json:"valid" db:"valid"`           // Default to true
	GemAmount     int       `json:"gem_amount" db:"gem_amount"`
}

type GemPurchase struct {
	TransactionID   int64     `json:"transaction_id"`              // SQL auto-incrementing primary key
	BuyerID         string    `json:"buyer_id"`                    // Buyer’s unique identifier
	CreatedAt       time.Time `json:"created_at"`                  // Timestamp when the purchase was created
	RcUserID        string    `json:"rc_user_id,omitempty"`        // Remote user ID (optional)
	RcTransactionID string    `json:"rc_transaction_id,omitempty"` // Remote transaction ID (optional)
	RcProductID     string    `json:"rc_product_id,omitempty"`     // Remote product ID (optional)
	Platform        string    `json:"platform,omitempty"`          // Platform where the purchase was made
	GemAmount       int       `json:"gem_amount"`                  // The number of gems in the purchase
	Valid           bool      `json:"valid"`                       // Whether the purchase is valid
}

type SubPurchase struct {
	TransactionID   int64     `json:"transaction_id"`
	BuyerID         string    `json:"buyer_id"`
	CreatorID       string    `json:"creator_id"`
	CreatedAt       time.Time `json:"created_at"`
	RcTransactionID string    `json:"rc_transaction_id"`
	RcProductID     string    `json:"rc_product_id"`
	Platform        string    `json:"platform"`
	Tier            int       `json:"tier"`
	Valid           bool      `json:"valid"`
}

type Event struct {
	OriginalAppUserID     string `json:"original_app_user_id"`
	OriginalTransactionID string `json:"original_transaction_id"`
	ProductID             string `json:"product_id"`
	AppUserID             string `json:"app_user_id"`
	Store                 string `json:"store"`
}

type RCJSON struct {
	ApiVersion string `json:"api_version"`
	Event      Event  `json:"event"`
}
