package common

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID                   primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	DisplayName          string             `json:"displayName"`
	FirstName            string             `json:"firstName" bson:"firstName"`
	LastName             string             `json:"lastName" bson:"lastName"`
	UserName             string             `json:"username"`
	Email                string             `json:"email"`
	Password             string             `json:"-"`
	BirthDate            string             `json:"birthDate" bson:"birthDate"`
	CreatedAt            time.Time          `json:"createdAt" bson:"createdAt"`
	Followers            int                `json:"followers"`
	Following            int                `json:"following"`
	Subscribers          int                `json:"subscribers"`
	Avatar_uri           string             `json:"avatarUri" bson:"avatarUri"`
	Banner_uri           string             `json:"bannerUri" bson:"bannerUri"`
	Verified             string             `json:"verified"`
	Code                 string             `json:"code"`
	Description          string             `json:"description"`
	Roles                []string           `json:"roles" bson:"roles"`
	ExpoToken            []string           `json:"expoToken" bson:"expoToken"`
	PendingNotifications int                `json:"pendingNotifications" bson:"pendingNotifications"`
	Gems                 int                `json:"gems"`
	Banned               bool               `json:"banned"`
}

type PublicUser struct {
	ID          primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	DisplayName string             `json:"displayName"`
	UserName    string             `json:"username"`
	CreatedAt   time.Time          `json:"createdAt" bson:"createdAt"`
	Followers   int                `json:"followers"`
	Following   int                `json:"following"`
	Subscribers int                `json:"subscribers"`
	Avatar_uri  string             `json:"avatarUri" bson:"avatarUri"`
	Banner_uri  string             `json:"bannerUri" bson:"bannerUri"`
	Description string             `json:"description"`
	Roles       []string           `json:"roles,omitempty" bson:"roles,omitempty"`
}

type Post struct {
	ID            primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Title         string             `json:"title"`
	Description   string             `json:"description"`
	User_id       primitive.ObjectID `json:"user_id" bson:"user_id"`
	Content       []string           `json:"content"`
	BluredContent []string           `json:"bluredContent"`
	Tags          []string           `json:"tags"`
	Gems          int                `json:"gems"`
	Likes         int                `json:"likes"`
	Comments      int                `json:"comments"`
	Shares        int                `json:"shares"`
	Saves         int                `json:"saves"`
	CreatedAt     time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt     time.Time          `json:"updatedAt" bson:"updatedAt"`
	Tier          int                `json:"tier"`
	Hidden        bool               `json:"hidden"`
	Impression    int                `json:"impression"`
}

type Comment struct {
	ID        primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	UserID    primitive.ObjectID `json:"user_id" bson:"user_id"`
	PostID    primitive.ObjectID `json:"post_id" bson:"post_id"`
	Gems      int32              `json:"gems"`
	Text      string             `json:"text"`
	Likes     int32              `json:"likes"`
	Replies   int32              `json:"replies"`
	CreatedAt time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt" bson:"updatedAt"`
	Hidden    bool               `json:"hidden"`
}

type Reply struct {
	ID        primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	UserID    primitive.ObjectID `json:"user_id" bson:"user_id"`
	CommentID primitive.ObjectID `json:"comment_id" bson:"comment_id"`
	Text      string             `json:"text"`
	CreatedAt time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt" bson:"updatedAt"`
}

type UserStore interface {
	CreateUser(User) error
	GetUserByEmail(email string) (*User, error)
	GetUserByID(id string) (*User, error)
	GetUserByObjectID(primitive.ObjectID) (*User, error)
	GetUserByUsername(email string) (*User, error)
	UpdateAvatar(primitive.ObjectID, string) error
	UpdateBanner(primitive.ObjectID, string) error
	UpdateUserCode(string, string) error
	UpdateValidationStatus(email string) error
	CheckAndRetrieveAvatar(primitive.ObjectID) (string, error)
	CheckAndRetrieveBanner(primitive.ObjectID) (string, error)
	SendEmail(string) error
}

type PostStore interface {
	CreatePost(Post) error
}

type RegisterUserPayload struct {
	FirstName   string `json:"firstName" validate:"required"`
	LastName    string `json:"lastName" validate:"required"`
	UserName    string `json:"userName" validate:"required"`
	DisplayName string `json:"displayName" validate:"required"`
	Email       string `json:"email" validate:"required,email"`
	Password    string `json:"password" validate:"required,min=3,max=130"`
	BirthDate   string `json:"birthDate" validate:"required"`
}

type LoginUserPayload struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type CreatePostPayload struct {
	Title       string   `json:"title" validate:"required"`
	Description string   `json:"description"`
	Content     []string `json:"content" validate:"required"`
	Tags        []string `json:"tags"`
	UserID      string   `json:"user_id"`
}

type Like_Dislike_PostPayload struct {
	Post_id primitive.ObjectID `json:"post_id" validate:"required"`
}

type LatestPostsPayload struct {
	Post_id primitive.ObjectID `json:"post_id" validate:"required"`
}

type AvatarPayload struct {
	Avatar string `json:"avatar" validate:"required"`
}

type BannerPayload struct {
	Banner string `json:"banner" validate:"required"`
}

type SendMailPayload struct {
	Email string `json:"email" validate:"required"`
}

type ValidateCodePayload struct {
	Email string `json:"email" validate:"required"`
	Code  string `json:"code" validate:"required"`
}

type FollowUserPayload struct {
	Follow_id primitive.ObjectID `json:"creator_id" validate:"required"`
}

type CreateCommentPayload struct {
	PostID primitive.ObjectID `json:"post_id" bson:"post_id" validate:"required"`
	Text   string             `json:"text" bson:"text" validate:"required"`
	Gems   int                `json:"gems" bson:"gems" validate:"min=0"`
}

type Like_Dislike_CommentPayload struct {
	Comment_id primitive.ObjectID `json:"comment_id" validate:"required"`
}

type CreateReplyPayload struct {
	CommentID primitive.ObjectID `json:"comment_id" bson:"comment_id" validate:"required"`
	Text      string             `json:"text" bson:"text" validate:"required"`
}

type DescriptionPayload struct {
	Description string `json:"description" validate:"required"`
}

type UsernamePayload struct {
	Username string `json:"username" validate:"required"`
}

type ChangeNamePayload struct {
	FirstName string `json:"firstName" validate:"required"`
	LastName  string `json:"lastName" validate:"required"`
}

type ChangeDisplayNamePayload struct {
	DisplayName string `json:"displayName" validate:"required"`
}

type MessagePayload struct {
	Text   string             `json:"text" validate:"required"`
	Gems   int                `json:"gems" validate:"required"`
	ChatID primitive.ObjectID `json:"chat_id" bson:"chat_id" validate:"required"`
}

type MessageSent struct {
	Text     string             `json:"text" validate:"required"`
	SenderID primitive.ObjectID `json:"sender_id" validate:"required"`
	Gems     int                `json:"gems" validate:"required"`
	ChatID   primitive.ObjectID `json:"chat_id" bson:"chat_id" validate:"required"`
}

type CreateChatPayload struct {
	CreatorID primitive.ObjectID `json:"creator_id"   bson:"creator_id"`
}

type CreateRoomPayload struct {
	ID      primitive.ObjectID `json:"id"`
	Userid  primitive.ObjectID `json:"user_id"`
	Request bool               `json:"request"`
}

type AddExpoTokenPayload struct {
	ExpoToken string `json:"ExpoToken"   bson:"ExpoToken"`
}

type PostNotifications struct {
	ID        primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	PostID    primitive.ObjectID `json:"post_id"`
	CreatorID primitive.ObjectID `json:"creator_id"`
	Type      string             `json:"type"`
}

type LikeNotificationPayload struct {
	User_id    primitive.ObjectID `json:"user_id"`
	Post_id    primitive.ObjectID `json:"post_id"`
	Comment_id primitive.ObjectID `json:"comment_id"`
}

type NotificationsSettingsPayload struct {
	User_id        primitive.ObjectID `json:"user_id"`
	UnreadMessages bool               `json:"unreadMessages"`
	NewPost        bool               `json:"newPost"`
	NewAboutEskiwi bool               `json:"newAboutEskiwi"`
	NewSub         bool               `json:"newSub"`
	NewComment     bool               `json:"newComment"`
	NewLike        bool               `json:"newLike"`
}

type FollowNotificationPayload struct {
	User_id    primitive.ObjectID `json:"user_id"`
	Creator_id primitive.ObjectID `json:"creator_id"`
}

type ChatNotificationPayload struct {
	User_id primitive.ObjectID `json:"user_id"`
	Chat_id primitive.ObjectID `json:"chat_id"`
	Text    string             `json:"text"`
}

type ChangePasswordPayload struct {
	Email    string `json:"email" validate:"required"`
	Code     string `json:"code" validate:"required"`
	Password string `json:"password" validate:"required,min=3,max=130"`
}

type CreateCommentDonationPayload struct {
	Post_id    primitive.ObjectID `json:"post_id" bson:"post_id" validate:"required"`
	Gems       int                `json:"gems" bson:"gems" validate:"required"`
	User_id    primitive.ObjectID `json:"user_id" bson:"user_id" validate:"required"`
	Comment_id primitive.ObjectID `json:"_id" bson:"_id" validate:"required"`
}

type CreateChatDonationPayload struct {
	Creator_id primitive.ObjectID `json:"creator_id" bson:"creator_id" validate:"required"`
	Gems       int                `json:"gems" bson:"gems" validate:"required"`
	User_id    primitive.ObjectID `json:"user_id" bson:"user_id" validate:"required"`
	ChatID     primitive.ObjectID `json:"chat_id" bson:"chat_id" validate:"required"`
}

type ChatsSettingsPayload struct {
	User_id primitive.ObjectID `json:"user_id"`
	Price   int                `json:"price" validate:"min=1"`
	Rules   []string           `json:"rules" bson:"rules"`
}

type AcceptChatPayload struct {
	ChatID primitive.ObjectID `json:"chat_id"   bson:"chat_id"`
}

type AcceptMessageReqPayload struct {
	TransID int `json:"transaction_id"   bson:"transaction_id"`
}

type GemsOnChatPayload struct {
	ChatID    primitive.ObjectID `json:"chat_id"   bson:"chat_id"`
	UserID    primitive.ObjectID `json:"user_id"`
	CreatorID primitive.ObjectID `json:"creator_id" bson:"creator_id" validate:"required"`
	Gems      int                `json:"gems" bson:"gems" validate:"required"`
	Type      string             `json:"type"`
	Pending   bool               `json:"pending"`
}

type Subscription struct {
	ID                    primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	UserID                primitive.ObjectID `json:"user_id" bson:"user_id"`
	CreatorID             primitive.ObjectID `json:"creator_id" bson:"creator_id"`
	TierID                primitive.ObjectID `json:"tier_id"`
	Tier                  int                `json:"tier"`
	Pending               bool               `json:"pending"`
	ProductID             string             `json:"productID" bson:"productID"`
	CreatedAt             time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt             time.Time          `json:"updatedAt" bson:"updatedAt"`
	Active                bool               `json:"active"`
	EndDate               time.Time          `json:"endDate" bson:"endDate"`
	OriginalTransactionID string             `json:"original_transaction_id" bson:"original_transaction_id"`
	Valid                 bool               `json:"valid"`
}

type SubscriptionTiers struct {
	ID         primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	CreatorID  primitive.ObjectID `json:"creator_id" bson:"creator_id"`
	Name       string             `json:"name" bson:"name"`
	Benefits   []string           `json:"benefits" bson:"benefits"`
	Tier       int                `json:"tier" validate:"min=1,max=5"`
	Subscribed bool               `json:"subscribed"`
}

type CreateSubscriptionPayload struct {
	CreatorID primitive.ObjectID `json:"creator_id" bson:"creator_id"`
	Tier      int                `json:"tier" validate:"min=1,max=5"`
	ProductID string             `json:"product_id"`
}

type GetTierMembersPayload struct {
	Tier int `json:"tier" validate:"min=0,max=5"`
}

type BanUserPayload struct {
	TargetID primitive.ObjectID `json:"target_id"`
}

type SharePostPostPayload struct {
	Post_id primitive.ObjectID `json:"post_id" validate:"required"`
}

type PostShare struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	UserID    primitive.ObjectID `bson:"user_id" json:"user_id"`
	PostID    primitive.ObjectID `bson:"post_id" json:"post_id"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}

type VerifyCreatorPayload struct {
	InstagramAccessToken string `json:"instagram_access_token" validate:"required"`
}

type RetrieveSub struct {
	SubID   primitive.ObjectID `bson:"sub_id" json:"sub_id"`
	Creator PublicUser         `bson:"creator" json:"creator"`
	Tier    int                `json:"tier"`
}
