package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	common "github.com/Eskiwi-Organization/infra/commons"
	pb "github.com/Eskiwi-Organization/infra/commons/api"
	auth "github.com/Eskiwi-Organization/infra/commons/auth"
	broker "github.com/Eskiwi-Organization/infra/commons/broker"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc"
)

type grpcHandler struct {
	pb.UnimplementedUserServiceServer

	notificationsClient pb.NotificationServiceClient

	service UserService
	store   UserStore
	channel *amqp.Channel
}

func NewGRPCHandler(grpcServer *grpc.Server, notificationsClient pb.NotificationServiceClient, service UserService, store UserStore, channel *amqp.Channel) {
	handler := &grpcHandler{
		service: service, notificationsClient: notificationsClient, store: store, channel: channel,
	}
	pb.RegisterUserServiceServer(grpcServer, handler)

}

func (h *grpcHandler) SendUserCode(ctx context.Context, p *pb.SendUserCodeRequest) (*pb.SendUserCodeResponse, error) {

	log.Printf("SendUserCode: Received login send code for email: %v", p.Email)

	marshalledRequest, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}

	q, err := h.channel.QueueDeclare(broker.SendCodemailCreatedEvent, true, false, false, false, nil)
	if err != nil {
		return nil, err
	}

	h.channel.PublishWithContext(ctx, "", q.Name, false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         marshalledRequest,
		DeliveryMode: amqp.Persistent,
	})

	o := &pb.SendUserCodeResponse{
		Status: "request sent succesfully",
	}
	return o, nil
}

func (h *grpcHandler) LoginUser(ctx context.Context, p *pb.LoginUserRequest) (*pb.LoginUserResponse, error) {

	///////////////////
	log.Printf("LoginUser: Received login request for email: %v", p.Email)
	//Se obtiene de la base de datos un json son la info de user q tiene ese correo
	user, err := h.store.GetUserByEmail(p.Email)
	if err != nil {
		log.Printf("LoginUser: Failed to get user by email: %v, error: %v", p.Email, err)
		o := &pb.LoginUserResponse{Status: "user with email " + p.Email + " does not exist"}
		return o, err
	}

	//Se hashea la contraseña y se verifica si es la misma
	if !auth.CheckPasswordHash(p.Password, user.Password) {
		log.Printf("LoginUser: Invalid password for user with email: %s", p.Email)
		err = fmt.Errorf("invalid email or password")
		o := &pb.LoginUserResponse{Status: "invalid email or password"}
		return o, err
	}

	//Revisar si no esta verificado
	if user.Verified != "true" {
		log.Printf("LoginUser: User with email %s is not verified", p.Email)
		err = fmt.Errorf("not validated")
		o := &pb.LoginUserResponse{Verified: "false"}
		return o, err
	}

	if user.Banned {
		err = fmt.Errorf("banned user")
		o := &pb.LoginUserResponse{}
		return o, err
	}

	// Successful login
	secret := []byte(JWTSecret)

	token, err := auth.CreateJWT(secret, user.ID.Hex())
	if err != nil {
		log.Printf("LoginUser: Failed to create JWT for user with email %s: %v", p.Email, err)
		o := &pb.LoginUserResponse{Status: err.Error()}
		return o, err
	}

	o := &pb.LoginUserResponse{
		Status:      "successfull",
		AccessToken: token}
	return o, nil
}

func (h *grpcHandler) ValidateUser(ctx context.Context, p *pb.ValidateUserRequest) (*pb.ValidateUserResponse, error) {
	user, err := h.store.GetUserByEmail(p.Email)
	if err != nil {
		o := &pb.ValidateUserResponse{Status: err.Error()}
		return o, err
	}

	if user.Code != p.Code {
		err = fmt.Errorf("codes are different")
		o := &pb.ValidateUserResponse{Status: "codes are different"}
		return o, err
	}

	err = h.store.UpdateValidationStatus(p.Email)
	if err != nil {
		o := &pb.ValidateUserResponse{Status: err.Error()}
		return o, err
	}

	o := &pb.ValidateUserResponse{Status: "validated " + user.UserName}
	return o, nil
}

func (h *grpcHandler) RegisterUser(ctx context.Context, p *pb.RegisterUserRequest) (*pb.RegisterUserResponse, error) {
	_, err := h.store.GetUserByEmail(p.Email)
	if err == nil {
		err = fmt.Errorf("user with email " + p.Email + " already exists")
		o := &pb.RegisterUserResponse{Status: "user with email " + p.Email + " already exists"}
		return o, err
	}

	_, err = h.store.GetUserByUsername(p.Username)
	if err == nil {
		err = fmt.Errorf("user with username " + p.Username + " already exists")
		o := &pb.RegisterUserResponse{Status: "user with username " + p.Username + " already exists"}
		return o, err
	}

	isCreator, err := h.store.IsEmailInWaitingCreators(p.Email)
	if err != nil {
		// Handle the error, for example, logging or returning it
		log.Printf("Error checking email in waiting-creators list: %v", err)
		return nil, err
	}

	roles := []string{}
	if isCreator {
		// If the user is a creator, add the "creator" role
		roles = append(roles, "creator")
	}

	hashedPassword, err := auth.HashPassword(p.Password)
	if err != nil {
		o := &pb.RegisterUserResponse{Status: err.Error()}
		return o, err
	}

	userID, err := h.store.CreateUser(common.User{
		FirstName:   p.FirstName,
		LastName:    p.LastName,
		UserName:    p.Username,
		DisplayName: p.DisplayName,
		Email:       p.Email,
		Password:    hashedPassword,
		BirthDate:   p.BirthDate,
		CreatedAt:   time.Now(),
		Verified:    "false",
		Followers:   0,
		Following:   0,
		Subscribers: 0,
		Avatar_uri:  "https://avatars.eskiwi.com/DefaultUser.jpg",
		Banner_uri:  "https://avatars.eskiwi.com/DefaultBanner.jpg",
		ExpoToken:   []string{},
		Roles:       roles,
		Banned:      false,
	})

	if err != nil {
		o := &pb.RegisterUserResponse{Status: err.Error()}
		return o, err
	}

	settings := &common.NotificationsSettingsPayload{
		User_id:        userID,
		UnreadMessages: true,
		NewPost:        true,
		NewAboutEskiwi: true,
		NewSub:         true,
		NewComment:     true,
		NewLike:        true,
	}

	err = h.store.UpsertNotificationSettings(*settings)
	if err != nil {
		return nil, err
	}

	log.Printf("New user registred! Registration of: %v", p.Email)
	o := &pb.RegisterUserResponse{Status: "successfull"}
	return o, nil
}

func (h *grpcHandler) GetUser(ctx context.Context, p *pb.GetUserRequest) (*pb.User, error) {
	log.Printf("GetUser: Received request to get user with ID: %v", p.UserID)

	objID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {

		errMsg := "error converting to primitive objectID: " + err.Error()
		log.Printf("GetUser: %v", errMsg)

		o := &pb.User{Status: "error converting to primitive objectID: " + err.Error()}
		return o, err
	}

	user, err := h.store.GetUserByObjectID(objID)
	if err != nil {
		errMsg := "error retrieving user: " + err.Error()
		log.Printf("GetUser: %v", errMsg)
		o := &pb.User{Status: err.Error()}
		return o, err
	}

	unread, err := h.store.TotalUnreadMessages(objID)
	if err != nil {
		errMsg := "error retrieving user unread messages: " + err.Error()
		log.Printf("GetUser: %v", errMsg)
		o := &pb.User{Status: err.Error()}
		return o, err
	}

	o := &pb.User{
		Id:                   p.UserID,
		FirstName:            user.FirstName,
		LastName:             user.LastName,
		DisplayName:          user.DisplayName,
		Username:             user.UserName,
		Email:                user.Email,
		Password:             user.Password,
		BirthDate:            user.BirthDate,
		CreatedAt:            user.CreatedAt.Format(time.RFC3339),
		AvatarUri:            user.Avatar_uri,
		BannerUri:            user.Banner_uri,
		Verified:             user.Verified,
		Code:                 user.Code,
		Followers:            int64(user.Followers),
		Following:            int64(user.Following),
		Subscribers:          int64(user.Subscribers),
		Description:          user.Description,
		Expo_Token:           user.ExpoToken,
		PendingNotifications: int32(user.PendingNotifications),
		Roles:                user.Roles,
		PendingMessages:      int32(unread),
		Gems:                 int32(user.Gems),
	}
	return o, nil
}

func (h *grpcHandler) GetUserByID(ctx context.Context, p *pb.GetUserRequest) (*pb.User, error) {
	log.Printf("GetUser: Received request to get user with ID: %v", p.UserID)

	objID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {

		errMsg := "error converting to primitive objectID: " + err.Error()
		log.Printf("GetUser: %v", errMsg)

		o := &pb.User{Status: "error converting to primitive objectID: " + err.Error()}
		return o, err
	}

	user, err := h.store.GetUserByObjectID(objID)
	if err != nil {
		errMsg := "error retrieving user: " + err.Error()
		log.Printf("GetUser: %v", errMsg)
		o := &pb.User{Status: err.Error()}
		return o, err
	}

	o := &pb.User{
		Id:          p.UserID,
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		DisplayName: user.DisplayName,
		Username:    user.UserName,
		Email:       user.Email,
		Password:    user.Password,
		BirthDate:   user.BirthDate,
		CreatedAt:   user.CreatedAt.Format(time.RFC3339),
		AvatarUri:   user.Avatar_uri,
		BannerUri:   user.Banner_uri,
		Verified:    user.Verified,
		Code:        user.Code,
		Followers:   int64(user.Followers),
		Following:   int64(user.Following),
		Subscribers: int64(user.Subscribers),
	}
	return o, nil
}

func (h *grpcHandler) UpdateAvatar(ctx context.Context, p *pb.UpdateAvatarRequest) (*pb.UpdateAvatarResponse, error) {
	objID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		o := &pb.UpdateAvatarResponse{Status: "error converting to primitive objectID: " + err.Error()}
		return o, err
	}

	err = h.store.UpdateAvatar(objID, p.Avatar)
	if err != nil {
		o := &pb.UpdateAvatarResponse{Status: "error updating: " + err.Error()}
		return o, err
	}

	o := &pb.UpdateAvatarResponse{Status: "successfull"}
	return o, nil
}

func (h *grpcHandler) UpdateBanner(ctx context.Context, p *pb.UpdateBannerRequest) (*pb.UpdateBannerResponse, error) {
	objID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		o := &pb.UpdateBannerResponse{Status: "error converting to primitive objectID: " + err.Error()}
		return o, err
	}

	err = h.store.UpdateBanner(objID, p.Banner)
	if err != nil {
		o := &pb.UpdateBannerResponse{Status: "error updating: " + err.Error()}
		return o, err
	}

	o := &pb.UpdateBannerResponse{Status: "successfull"}
	return o, nil
}

func (h *grpcHandler) FollowUser(ctx context.Context, p *pb.FollowUserRequest) (*pb.FollowUserResponse, error) {
	UserObjID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		o := &pb.FollowUserResponse{Status: "error converting user to primitive objectID: " + err.Error()}
		return o, err
	}

	TargetObjID, err := primitive.ObjectIDFromHex(p.TargetID)
	if err != nil {
		o := &pb.FollowUserResponse{Status: "error converting follow to primitive objectID: " + err.Error()}
		return o, err
	}

	if UserObjID == TargetObjID {
		err = fmt.Errorf("following one self is not allowed")
		return nil, err
	}

	err = h.store.CreateUserFollow(UserFollow{
		FollowingID: TargetObjID,
		FollowerID:  UserObjID,
		CreatedAt:   time.Now(),
	})
	if err != nil {
		o := &pb.FollowUserResponse{Status: "error creating follow: " + err.Error()}
		return o, err
	}

	//sumar following al userID
	err = h.store.UserFollowingCount(UserObjID, 1)
	if err != nil {
		o := &pb.FollowUserResponse{Status: "error uploading following increment to mongo user: " + err.Error()}
		return o, err
	}

	//sumar follower al targetID
	err = h.store.UserFollowerCount(TargetObjID, 1)
	if err != nil {
		o := &pb.FollowUserResponse{Status: "error uploading follower increment to mongo user: " + err.Error()}
		return o, err
	}

	var payload common.FollowNotificationPayload

	payload.Creator_id = TargetObjID
	payload.User_id = UserObjID

	marshalledRequest, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		return nil, err
	}

	q, err := h.channel.QueueDeclare(broker.SendFollowNotificationCreatedEvent, true, false, false, false, nil)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		return nil, err
	}

	h.channel.PublishWithContext(ctx, "", q.Name, false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         marshalledRequest,
		DeliveryMode: amqp.Persistent,
	})

	o := &pb.FollowUserResponse{Status: "successfull"}
	return o, nil
}

func (h *grpcHandler) UnFollowUser(ctx context.Context, p *pb.FollowUserRequest) (*pb.FollowUserResponse, error) {
	UserObjID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		o := &pb.FollowUserResponse{Status: "error converting user to primitive objectID: " + err.Error()}
		return o, err
	}

	TargetObjID, err := primitive.ObjectIDFromHex(p.TargetID)
	if err != nil {
		o := &pb.FollowUserResponse{Status: "error converting follow to primitive objectID: " + err.Error()}
		return o, err
	}

	err = h.store.DeleteUserFollow(UserFollow{
		FollowingID: TargetObjID,
		FollowerID:  UserObjID,
	})
	if err != nil {
		o := &pb.FollowUserResponse{Status: "error creating follow: " + err.Error()}
		return o, err
	}

	//restar following al userID
	err = h.store.UserFollowingCount(UserObjID, -1)
	if err != nil {
		o := &pb.FollowUserResponse{Status: "error uploading unfollowing increment to mongo user: " + err.Error()}
		return o, err
	}

	//restar follower al targetID
	err = h.store.UserFollowerCount(TargetObjID, -1)
	if err != nil {
		o := &pb.FollowUserResponse{Status: "error uploading unfollower increment to mongo user: " + err.Error()}
		return o, err
	}

	o := &pb.FollowUserResponse{Status: "successfull"}
	return o, nil
}

func (h *grpcHandler) GetUserFollow(ctx context.Context, p *pb.GetUserFollowRequest) (*pb.GetUserFollowResponse, error) {
	log.Printf("New GetUserLike received!")

	objUserID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		return nil, err
	}

	objTargetID, err := primitive.ObjectIDFromHex(p.TargetID)
	if err != nil {
		return nil, err
	}

	var userFollow UserFollow

	userFollow.FollowingID = objTargetID
	userFollow.FollowerID = objUserID

	following, err := h.store.CheckUserFollowLike(userFollow)
	if err != nil {
		return nil, err
	}

	o := &pb.GetUserFollowResponse{
		Following: following,
	}

	return o, nil
}

func (h *grpcHandler) UpdateDescription(ctx context.Context, p *pb.UpdateDescriptionRequest) (*pb.UpdateDescriptionResponse, error) {
	log.Printf("New description received!")

	objUserID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		return nil, err
	}

	err = h.store.UpdateUserDescription(p.Text, objUserID)
	if err != nil {
		return nil, err
	}

	o := &pb.UpdateDescriptionResponse{
		Status: "updated",
	}

	return o, nil
}

func (h *grpcHandler) AddExpoToken(ctx context.Context, p *pb.AddExpoTokenRequest) (*pb.AddExpoTokenResponse, error) {
	userObjID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		o := &pb.AddExpoTokenResponse{}
		return o, err
	}

	err = h.store.AddExpoToken(userObjID, p.ExpoToken)
	if err != nil {
		o := &pb.AddExpoTokenResponse{}
		return o, err
	}

	o := &pb.AddExpoTokenResponse{Status: "token added"}
	return o, nil
}

func (h *grpcHandler) DeleteExpoToken(ctx context.Context, p *pb.AddExpoTokenRequest) (*pb.AddExpoTokenResponse, error) {
	userObjID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		o := &pb.AddExpoTokenResponse{}
		return o, err
	}

	err = h.store.RemoveExpoToken(userObjID, p.ExpoToken)
	if err != nil {
		o := &pb.AddExpoTokenResponse{}
		return o, err
	}

	o := &pb.AddExpoTokenResponse{Status: "token deleted"}
	return o, nil
}

func (h *grpcHandler) UpdateNotificationsSettings(ctx context.Context, p *pb.NotificationsSettings) (*pb.UpdateNotificationsSettingsResponse, error) {

	convertedStruct, err := ConvertProtoMsgToStruct(p)
	if err != nil {
		fmt.Printf("Error converting gRPC message to struct: %v\n", err)
	}

	err = h.store.UpsertNotificationSettings(*convertedStruct)
	if err != nil {
		o := &pb.UpdateNotificationsSettingsResponse{}
		return o, err
	}

	o := &pb.UpdateNotificationsSettingsResponse{
		Status: "updated",
	}
	return o, nil
}

func (h *grpcHandler) GetUserNotificationSettings(ctx context.Context, p *pb.GetUserNotificationSettingsRequest) (*pb.NotificationsSettings, error) {
	log.Printf("New GetUserNotificationSettings received!")

	objUserID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		return nil, err
	}

	notificationSettings, err := h.store.GetUserNotificationSettings(objUserID)
	if err != nil {
		o := &pb.NotificationsSettings{}
		return o, err
	}

	o, err := ConvertStructToProtoNotification(notificationSettings)
	if err != nil {
		fmt.Printf("Error converting struct to gRPC message: %v\n", err)
	}

	return o, nil
}

func (h *grpcHandler) SearchCreators(ctx context.Context, p *pb.SearchCreatorsRequest) (*pb.SearchCreatorsResponse, error) {
	log.Printf("New SearchCreators received!")

	CreatorsList, err := h.store.GetUsersByUsername(p.Username)
	if err != nil {
		o := &pb.SearchCreatorsResponse{}
		return o, err
	}

	o, err := h.store.convertToSearchCreatorsResponse(CreatorsList)
	if err != nil {
		fmt.Printf("Error converting struct to gRPC publicUsers: %v\n", err)
	}

	return o, nil
}

func (h *grpcHandler) GetTopCreator(ctx context.Context, p *pb.GetTopCreatorRequest) (*pb.GetTopCreatorResponse, error) {
	log.Printf("New SearchCreators received!")

	o := &pb.GetTopCreatorResponse{
		TopCreators: topCreatorsList,
	}

	return o, nil
}

func (h *grpcHandler) ChangePassword(ctx context.Context, p *pb.ChangePasswordRequest) (*pb.ChangePasswordResponse, error) {
	user, err := h.store.GetUserByEmail(p.Email)
	if err != nil {
		o := &pb.ChangePasswordResponse{Status: err.Error()}
		return o, err
	}

	if user.Code != p.Code {
		err = fmt.Errorf("codes are different")
		o := &pb.ChangePasswordResponse{Status: "codes are different"}
		return o, err
	}

	hashedPassword, err := auth.HashPassword(p.Password)
	if err != nil {
		o := &pb.ChangePasswordResponse{Status: err.Error()}
		return o, err
	}

	err = h.store.UpdateUserPassword(p.Email, hashedPassword)
	if err != nil {
		o := &pb.ChangePasswordResponse{Status: err.Error()}
		return o, err
	}

	o := &pb.ChangePasswordResponse{Status: "changed password of " + user.UserName}
	return o, nil
}

func (h *grpcHandler) UpdateUsername(ctx context.Context, p *pb.UpdateUsernameRequest) (*pb.UpdateUsernameResponse, error) {
	user, err := h.store.GetUserByID(p.UserID)
	if err != nil {
		o := &pb.UpdateUsernameResponse{Status: err.Error()}
		return o, err
	}

	_, err = h.store.GetUserByUsername(p.Username)
	if err == nil {
		err = fmt.Errorf("user with username " + p.Username + " already exists")
		o := &pb.UpdateUsernameResponse{Status: "user with username " + p.Username + " already exists"}
		return o, err
	}

	err = h.store.UpdateUsername(p.Username, user.ID)
	if err != nil {
		o := &pb.UpdateUsernameResponse{Status: err.Error()}
		return o, err
	}

	o := &pb.UpdateUsernameResponse{Status: "changed username from " + user.UserName + " to " + p.Username}
	return o, nil
}

func (h *grpcHandler) CreateChatsSettings(ctx context.Context, p *pb.CreateChatsSettingsRequest) (*pb.CreateChatsSettingsResponse, error) {

	user, err := h.store.GetUserByID(p.Chatsettings.UserId)
	if err != nil {
		log.Printf("error getting user: %v", err)
		o := &pb.CreateChatsSettingsResponse{}
		return o, err
	}

	if !h.store.IsCreator(*user) {
		err = fmt.Errorf("unauthorized access: not a creator")
		o := &pb.CreateChatsSettingsResponse{}
		return o, err
	}

	convertedStruct, err := ConvertProtoChatSettingsToStruct(p)
	if err != nil {
		fmt.Printf("Error converting gRPC message to struct: %v\n", err)
		o := &pb.CreateChatsSettingsResponse{}
		return o, err
	}

	err = h.store.UpsertChatPriceAndRules(*convertedStruct)
	if err != nil {
		o := &pb.CreateChatsSettingsResponse{}
		return o, err
	}

	o := &pb.CreateChatsSettingsResponse{
		Status: "updated",
	}
	return o, nil
}

func (h *grpcHandler) GetChatSettings(ctx context.Context, p *pb.GetChatSettingsRequest) (*pb.ChatsSettings, error) {

	userObjID, err := primitive.ObjectIDFromHex(p.UserId)
	if err != nil {
		o := &pb.ChatsSettings{}
		return o, err
	}

	settings, err := h.store.GetChatPriceAndRules(userObjID)
	if err != nil {
		log.Printf("Error retrieving chat settings: %v", err)
		o := &pb.ChatsSettings{}
		return o, err
	}

	o := &pb.ChatsSettings{
		UserId: settings.User_id.Hex(),
		Price:  int32(settings.Price),
		Rules:  settings.Rules,
	}
	return o, nil
}

func (h *grpcHandler) GetTransactions(ctx context.Context, p *pb.GetTransactionsRequest) (*pb.TransactionsResponse, error) {
	log.Printf("New GetUserNotificationSettings received!")

	objUserID, err := primitive.ObjectIDFromHex(p.UserId)
	if err != nil {
		return nil, err
	}

	o, err := h.store.GetAllUserTransactions(objUserID, int(p.Page), 10)
	if err != nil {
		fmt.Printf("Error fetching transactions: %v\n", err)
	}

	return o, nil
}

func (h *grpcHandler) CreateSubscriptionTier(ctx context.Context, p *pb.SubscriptionTiers) (*pb.CreateSubscriptionTierResponse, error) {

	subTier, err := h.store.ConvertPBToSubscriptionTiers(p)
	if err != nil {
		o := &pb.CreateSubscriptionTierResponse{}
		return o, err
	}

	_, err = h.store.CreateSubscriptionTier(*subTier)
	if err != nil {
		o := &pb.CreateSubscriptionTierResponse{}
		return o, err
	}

	o := &pb.CreateSubscriptionTierResponse{
		Status: "created",
	}
	return o, nil
}

func (h *grpcHandler) UpdateSubscriptionTier(ctx context.Context, p *pb.SubscriptionTiers) (*pb.UpdateSubscriptionTierResponse, error) {

	subTier, err := h.store.ConvertPBToSubscriptionTiers(p)
	if err != nil {
		o := &pb.UpdateSubscriptionTierResponse{}
		return o, err
	}

	err = h.store.UpdateSubscriptionTier(*subTier)
	if err != nil {
		o := &pb.UpdateSubscriptionTierResponse{}
		return o, err
	}

	o := &pb.UpdateSubscriptionTierResponse{
		Status: "created",
	}
	return o, nil
}

func (h *grpcHandler) GetSubscriptionTier(ctx context.Context, p *pb.GetSubscriptionTierRequest) (*pb.GetSubscriptionTierResponse, error) {

	creatorObjID, err := primitive.ObjectIDFromHex(p.CreatorId)
	if err != nil {
		o := &pb.GetSubscriptionTierResponse{}
		return o, err
	}

	userObjID, err := primitive.ObjectIDFromHex(p.UserId)
	if err != nil {
		o := &pb.GetSubscriptionTierResponse{}
		return o, err
	}

	tiers, err := h.store.GetSubscriptionTiersAsPB(creatorObjID, userObjID)
	if err != nil {
		o := &pb.GetSubscriptionTierResponse{}
		return o, err
	}

	o := &pb.GetSubscriptionTierResponse{
		Tiers: tiers,
	}
	return o, nil
}

func (h *grpcHandler) GetTierMembers(ctx context.Context, p *pb.GetTierMembersRequest) (*pb.GetTierMembersResponse, error) {

	creatorObjID, err := primitive.ObjectIDFromHex(p.CreatorId)
	if err != nil {
		o := &pb.GetTierMembersResponse{}
		return o, err
	}

	members, err := h.store.FindTierMembers(creatorObjID, int(p.Tier), int(p.Page), 10)
	if err != nil {
		o := &pb.GetTierMembersResponse{}
		return o, err
	}

	o := &pb.GetTierMembersResponse{
		Members: members,
	}
	return o, nil
}

func (h *grpcHandler) UpdateName(ctx context.Context, p *pb.UpdateNameRequest) (*pb.UpdateNameResponse, error) {

	userObjID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		o := &pb.UpdateNameResponse{}
		return o, err
	}

	err = h.store.UpdateUserDetails(p.LastName, p.FirstName, userObjID)
	if err != nil {
		o := &pb.UpdateNameResponse{Status: err.Error()}
		return o, err
	}

	o := &pb.UpdateNameResponse{Status: "changed"}
	return o, nil
}

func (h *grpcHandler) BanUser(ctx context.Context, p *pb.BanUserRequest) (*pb.BanUserResponse, error) {

	User, err := h.store.GetUserByID(p.UserId)
	if err != nil {
		o := &pb.BanUserResponse{}
		return o, err
	}

	objUserID, err := primitive.ObjectIDFromHex(p.TargetId)
	if err != nil {
		return nil, err
	}

	if !h.store.IsAdmin(*User) {
		o := &pb.BanUserResponse{}
		err = fmt.Errorf("only admin")
		return o, err
	}

	err = h.store.BanUser(objUserID, User.ID)
	if err != nil {
		o := &pb.BanUserResponse{}
		return o, err
	}

	o := &pb.BanUserResponse{Status: "user banned"}
	return o, nil
}

func (h *grpcHandler) GetEarnings(ctx context.Context, p *pb.GetEarningsRequest) (*pb.Earnings, error) {
	log.Printf("New GetUserNotificationSettings received!")

	objUserID, err := primitive.ObjectIDFromHex(p.UserId)
	if err != nil {
		return nil, err
	}

	o, err := h.store.GetLast30DaysEarnings(objUserID)
	if err != nil {
		fmt.Printf("Error fetching transactions: %v\n", err)
	}

	return o, nil
}

func (h *grpcHandler) DeleteAccount(ctx context.Context, p *pb.DeleteAccountRequest) (*pb.DeleteAccountResponse, error) {

	objUserID, err := primitive.ObjectIDFromHex(p.UserId)
	if err != nil {
		return nil, err
	}

	err = h.store.WipeUserData(objUserID)
	if err != nil {
		o := &pb.DeleteAccountResponse{}
		return o, err
	}

	err = h.store.DeleteFollowsAndDecreaseCount(objUserID)
	if err != nil {
		o := &pb.DeleteAccountResponse{}
		return o, err
	}

	err = h.store.WipeSubscriptionTiers(objUserID)
	if err != nil {
		o := &pb.DeleteAccountResponse{}
		return o, err
	}

	err = h.store.DeleteUserNotifications(objUserID)
	if err != nil {
		o := &pb.DeleteAccountResponse{}
		return o, err
	}

	err = h.store.HidePostsByUser(objUserID)
	if err != nil {
		o := &pb.DeleteAccountResponse{}
		return o, err
	}

	o := &pb.DeleteAccountResponse{Status: "user banned"}
	return o, nil
}

func (h *grpcHandler) UpdateDisplayName(ctx context.Context, p *pb.UpdateDisplayNameRequest) (*pb.UpdateDisplayNameResponse, error) {

	userObjID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		o := &pb.UpdateDisplayNameResponse{}
		return o, err
	}

	err = h.store.UpdateUserDisplayName(p.DisplayName, userObjID)
	if err != nil {
		o := &pb.UpdateDisplayNameResponse{Status: err.Error()}
		return o, err
	}

	o := &pb.UpdateDisplayNameResponse{Status: "changed"}
	return o, nil
}

func (h *grpcHandler) VerifyCreator(ctx context.Context, p *pb.VerifyCreatorRequest) (*pb.VerifyCreatorResponse, error) {

	userObjID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		o := &pb.VerifyCreatorResponse{}
		return o, err
	}

	followersCount, instagramUserName, err := h.store.GetInstagramFollowersCountAndUsername(p.InstagramToken)
	if err != nil {
		o := &pb.VerifyCreatorResponse{}
		return o, err
	}

	if followersCount >= 10000 {
		err = h.store.AddCreatorRole(userObjID)
		if err != nil {
			o := &pb.VerifyCreatorResponse{}
			return o, err
		}
		o := &pb.VerifyCreatorResponse{Approved: true}
		return o, nil

	}

	err = h.store.UpsertPendingCreator(userObjID, followersCount, instagramUserName)
	if err != nil {
		o := &pb.VerifyCreatorResponse{}
		return o, err
	}

	o := &pb.VerifyCreatorResponse{Approved: false}
	return o, nil
}

func (h *grpcHandler) GetSubEarnings(ctx context.Context, p *pb.GetSubEarningsRequest) (*pb.Earnings, error) {
	log.Printf("New GetUserNotificationSettings received!")

	objUserID, err := primitive.ObjectIDFromHex(p.UserId)
	if err != nil {
		return nil, err
	}

	o, err := h.store.GetLast30DaysSubEarnings(objUserID)
	if err != nil {
		fmt.Printf("Error fetching transactions: %v\n", err)
	}

	return o, nil
}

func (h *grpcHandler) RetrieveSub(ctx context.Context, p *pb.RetrieveSubRequest) (*pb.RetrieveSubResponse, error) {

	userObjID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		o := &pb.RetrieveSubResponse{}
		return o, err
	}

	o, err := h.store.RetrieveUserSubscriptions(userObjID, int(p.Page), 10)
	if err != nil {
		o := &pb.RetrieveSubResponse{}
		return o, err
	}

	return o, nil
}
