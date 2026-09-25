package main

import (
	"context"
	"mime/multipart"
	"time"

	common "github.com/Eskiwi-Organization/infra/commons"
	pb "github.com/Eskiwi-Organization/infra/commons/api"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UserService interface {
	LoginUser(context.Context) error
	ValidateUser(context.Context) error
	RegisterUser(context.Context) error
	GetUser(context.Context) error
	UpdateAvatar(context.Context) error
	UpdateBanner(context.Context) error
	AddExpoToken(context.Context) error
	DeleteExpoToken(context.Context) error
	UpdateNotificationsSettings(context.Context) error
	GetUserNotificationSettings(context.Context) error
	SearchCreators(context.Context) error
	GetTopCreator(context.Context) error
	GetChatSettings(context.Context) error
	GetTransactions(context.Context) error
	CreateSubscriptionTier(context.Context) error
	GetSubscriptionTier(context.Context) error
	GetTierMembers(context.Context) error
	UpdateName(context.Context) error
}

type UserFollow struct {
	ID          primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	FollowingID primitive.ObjectID `json:"following_id" bson:"following_id"` //account that is going to get a follower aka targetID
	FollowerID  primitive.ObjectID `json:"follower_id" bson:"follower_id"`   //account that is going to follow the account aka userID
	CreatedAt   time.Time          `json:"createdAt"`
}

type UserStore interface {
	Create(context.Context) error
	CreateUser(user common.User) (primitive.ObjectID, error)
	GetUserByEmail(email string) (*common.User, error)
	GetUserByID(id string) (*common.User, error)
	GetUserByObjectID(primitive.ObjectID) (*common.User, error)
	GetUserByUsername(email string) (*common.User, error)
	UpdateAvatar(primitive.ObjectID, string) error
	UpdateBanner(primitive.ObjectID, string) error
	UpdateUserCode(string, string) error
	UpdateValidationStatus(email string) error
	CheckAndRetrieveAvatar(primitive.ObjectID) (string, error)
	CheckAndRetrieveBanner(primitive.ObjectID) (string, error)
	CreateUserFollow(follow UserFollow) error
	DeleteUserFollow(follow UserFollow) error
	UserFollowerCount(userID primitive.ObjectID, increase_decrease int) error
	UserFollowingCount(userID primitive.ObjectID, increase_decrease int) error
	CheckUserFollowLike(userFollow UserFollow) (bool, error)
	UpdateUserDescription(description string, userID primitive.ObjectID) error
	AddExpoToken(userID primitive.ObjectID, token string) error
	RemoveExpoToken(userID primitive.ObjectID, token string) error
	UpsertNotificationSettings(settings common.NotificationsSettingsPayload) error
	GetUserNotificationSettings(userID primitive.ObjectID) (*common.NotificationsSettingsPayload, error)
	GetUsersByUsername(username string) ([]common.PublicUser, error)
	convertToSearchCreatorsResponse(users []common.PublicUser) (*pb.SearchCreatorsResponse, error)
	GetTopUsersByFollowers() ([]common.PublicUser, error)
	convertToSearchTopCreatorsResponse(users []common.PublicUser) (*pb.GetTopCreatorResponse, error)
	UploadMultiPartContent(files []*multipart.FileHeader) ([]string, error)
	UpdateUserPassword(email string, newPassword string) error
	UpdateUsername(username string, userID primitive.ObjectID) error
	CountUnreadMessages(chatID primitive.ObjectID, userID primitive.ObjectID) (int64, error)
	TotalUnreadMessages(userID primitive.ObjectID) (int64, error)
	UpsertChatPriceAndRules(settings common.ChatsSettingsPayload) error
	GetChatPriceAndRules(userID primitive.ObjectID) (*common.ChatsSettingsPayload, error)
	GetAllUserTransactions(userID primitive.ObjectID, page, pageSize int) (*pb.TransactionsResponse, error)
	ConvertPBToSubscriptionTiers(pbTier *pb.SubscriptionTiers) (*common.SubscriptionTiers, error)
	CreateSubscriptionTier(tier common.SubscriptionTiers) (primitive.ObjectID, error)
	UpdateSubscriptionTier(tier common.SubscriptionTiers) error
	ConvertSubscriptionTiersToPB(tier *common.SubscriptionTiers) (*pb.SubscriptionTiers, error)
	GetSubscriptionTiers(creatorID, userID primitive.ObjectID) ([]common.SubscriptionTiers, error)
	ConvertSubscriptionTierToPB(tier *common.SubscriptionTiers) *pb.SubscriptionTiers
	GetSubscriptionTiersAsPB(creatorID, UserID primitive.ObjectID) ([]*pb.SubscriptionTiers, error)
	ConvertSubscriptionTiersListToPB(tiers []common.SubscriptionTiers) []*pb.SubscriptionTiers
	FindTierMembers(creatorID primitive.ObjectID, tier, page, pageSize int) ([]*pb.TierMember, error)
	UpdateUserDetails(lastName string, username string, userID primitive.ObjectID) error
	IsAdmin(user common.User) bool
	BanUser(userID, adminID primitive.ObjectID) error
	ConvertToGRPCEarnings(earnings Earnings) *pb.Earnings
	GetLast30DaysEarnings(userID primitive.ObjectID) (*pb.Earnings, error)
	WipeUserData(userID primitive.ObjectID) error
	DeleteFollowsAndDecreaseCount(userID primitive.ObjectID) error
	WipeSubscriptionTiers(userID primitive.ObjectID) error
	DeleteUserNotifications(userID primitive.ObjectID) error
	HidePostsByUser(userID primitive.ObjectID) error
	UpdateUserDisplayName(displayName string, userID primitive.ObjectID) error
	GetInstagramFollowersCountAndUsername(accessToken string) (int, string, error)
	AddCreatorRole(userID primitive.ObjectID) error
	UpsertPendingCreator(userID primitive.ObjectID, followersCount int, userName string) error
	GetLast30DaysSubEarnings(userID primitive.ObjectID) (*pb.Earnings, error)
	RetrieveUserSubscriptions(userID primitive.ObjectID, page int, pageSize int) (*pb.RetrieveSubResponse, error)
	IsEmailInWaitingCreators(email string) (bool, error)
	UpdateTopCreators() error
	IsCreator(user common.User) bool
}

type Transaction struct {
	ID              int64             `json:"id"`              // transaction_id
	GemAmount       int               `json:"gemAmount"`       // gem_amount
	TransactionType string            `json:"transactionType"` // tipo de transacción: "comentario"
	Type            string            `json:"type"`            // outgoing o ingoing
	From            common.PublicUser `json:"from"`            // Usuario involucrado en la transacción
	Pending         bool              `json:"pending"`         // Siempre false para gem_donations
	CreatedAt       time.Time         `json:"createdAt"`
}

type TransactionsResponse struct {
	Transactions []Transaction `json:"transactions"`
}

type TierMember struct {
	Tier int32
	User common.PublicUser
}

type DayTransactions struct {
	Day   time.Time `json:"day"`
	Total int       `json:"total"`
}

type Earnings struct {
	TotalValid      int               `json:"totalValid"`
	DayTransactions []DayTransactions `json:"DayTransactions"`
}

type InstagramResponse struct {
	FollowersCount int    `json:"followers_count"`
	ID             string `json:"id"`
	Username       string `json:"username"`
}

type PendingCreator struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	UserID         primitive.ObjectID `bson:"user_id" json:"user_id"`
	FollowersCount int                `json:"followers_count"`
	CreatedAt      time.Time          `bson:"createdAt" json:"createdAt"`
}
