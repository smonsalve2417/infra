package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	common "github.com/Eskiwi-Organization/infra/commons"
	pb "github.com/Eskiwi-Organization/infra/commons/api"
	"github.com/Eskiwi-Organization/infra/commons/auth"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/websocket"

	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Adjust this to ensure your origin checks are adequate for your security requirements
	},
}

type handler struct {
	//notificationsClient pb.NotificationServiceClient
	webClient      pb.WebServiceClient
	postClient     pb.PostServiceClient
	userClient     pb.UserServiceClient
	notifClient    pb.NotificationServiceClient
	paymentsClient pb.PaymentsServiceClient
	mongodbclient  *mongo.Client
	mongodatabase  *mongo.Database
}

func NewHandler(
	webClient pb.WebServiceClient,
	postClient pb.PostServiceClient,
	userClient pb.UserServiceClient,
	notifClient pb.NotificationServiceClient,
	paymentsClient pb.PaymentsServiceClient,
	mongodbclient *mongo.Client,
) *handler {
	return &handler{
		webClient:      webClient,
		postClient:     postClient,
		userClient:     userClient,
		notifClient:    notifClient,
		paymentsClient: paymentsClient,
		mongodbclient:  mongodbclient,
		mongodatabase:  mongodbclient.Database("Eskiwi"),
	}
}

func (h *handler) registerRoutes(mux *http.ServeMux) {

	//WebSocket
	mux.HandleFunc("GET /ws/joinRoom/{roomID}", auth.WithJWTAuth(h.JoinRoom, h.mongodatabase))
	//Notifications
	mux.HandleFunc("GET /notification/getNotifications/{page}", auth.WithJWTAuth(h.HandleGetNotifications, h.mongodatabase))
	//Posts
	mux.HandleFunc("POST /posts/createPost", auth.WithJWTAuth(h.HandleCreatePost, h.mongodatabase))
	mux.HandleFunc("DELETE /posts/createPost/{postID}", auth.WithJWTAuth(h.HandleDeletePost, h.mongodatabase))
	mux.HandleFunc("POST /posts/likePost", auth.WithJWTAuth(h.HandleLikePost, h.mongodatabase))
	mux.HandleFunc("DELETE /posts/likePost/{postID}", auth.WithJWTAuth(h.HandleUnlikePost, h.mongodatabase))
	mux.HandleFunc("GET /posts/getUserLike/{postID}", auth.WithJWTAuth(h.HandleGetUserLike, h.mongodatabase))
	//		comments
	mux.HandleFunc("POST /posts/createComment", auth.WithJWTAuth(h.HandleCreateComment, h.mongodatabase))
	mux.HandleFunc("DELETE /posts/createComment/{commentID}", auth.WithJWTAuth(h.HandleDeleteComment, h.mongodatabase))
	mux.HandleFunc("POST /posts/likeComment", auth.WithJWTAuth(h.HandleLikeComment, h.mongodatabase))
	mux.HandleFunc("DELETE /posts/likeComment/{commentID}", auth.WithJWTAuth(h.HandleUnLikeComment, h.mongodatabase))
	mux.HandleFunc("GET /posts/getUserCommentLike/{commentID}", auth.WithJWTAuth(h.HandleGetUserCommentLike, h.mongodatabase))
	//		replies
	mux.HandleFunc("POST /posts/createReply", auth.WithJWTAuth(h.HandleCreateReply, h.mongodatabase))
	mux.HandleFunc("DELETE /posts/createReply/{commentID}/{replyID}", auth.WithJWTAuth(h.HandleDeleteReply, h.mongodatabase))
	//		pagination
	mux.HandleFunc("GET /posts/getPost/{postID}", auth.WithJWTAuth(h.HandleGetPost, h.mongodatabase))
	mux.HandleFunc("GET /posts/getLatestPosts/{page}", auth.WithJWTAuth(h.HandleLatestPosts, h.mongodatabase))
	mux.HandleFunc("GET /posts/getLatestUserPosts/{userID}/{page}", auth.WithJWTAuth(h.GetLatestUserPosts, h.mongodatabase))
	mux.HandleFunc("GET /posts/getLatestComments/{postID}/{page}", auth.WithJWTAuth(h.HandleLatestComments, h.mongodatabase))
	mux.HandleFunc("GET /posts/getLatestReplies/{commentID}/{page}", auth.WithJWTAuth(h.HandleLatestReplies, h.mongodatabase))
	//User
	//		auth
	mux.HandleFunc("POST /auth/login", h.HandleLogin)
	mux.HandleFunc("POST /auth/verifyCode", h.HandleVerifyCode)
	mux.HandleFunc("POST /auth/register", h.HandleRegister)
	mux.HandleFunc("POST /auth/sendVerificationCode", h.HandleGetVerificationCode)
	mux.HandleFunc("POST /auth/changePassword", h.HandleChangePassword)
	mux.HandleFunc("GET /auth/getUser", auth.WithJWTAuth(h.HandleGetUser, h.mongodatabase))
	//		Fin auth
	mux.HandleFunc("GET /user/getUserByID/{userID}", auth.WithJWTAuth(h.HandleGetUserByID, h.mongodatabase))
	mux.HandleFunc("POST /user/updateAvatar", auth.WithJWTAuth(h.UpdateAvatar, h.mongodatabase))
	mux.HandleFunc("POST /user/updateBanner", auth.WithJWTAuth(h.UpdateBanner, h.mongodatabase))
	mux.HandleFunc("POST /user/followUser", auth.WithJWTAuth(h.HandleFollowUser, h.mongodatabase))
	mux.HandleFunc("DELETE /user/followUser/{targetID}", auth.WithJWTAuth(h.HandleUnFollowUser, h.mongodatabase))
	mux.HandleFunc("GET /user/getUserFollow/{targetID}", auth.WithJWTAuth(h.HandleGetUserFollow, h.mongodatabase))
	mux.HandleFunc("POST /user/updateDescription", auth.WithJWTAuth(h.HandleUpdateDescription, h.mongodatabase))
	mux.HandleFunc("POST /user/updateUsername", auth.WithJWTAuth(h.HandleUpdateUsername, h.mongodatabase))
	mux.HandleFunc("POST /user/ExpoToken", auth.WithJWTAuth(h.AddExpoToken, h.mongodatabase))
	mux.HandleFunc("DELETE /user/ExpoToken/{expotoken}", auth.WithJWTAuth(h.DeleteExpoToken, h.mongodatabase))
	mux.HandleFunc("POST /user/updateNotificationsSettings", auth.WithJWTAuth(h.UpdateNotificationsSettings, h.mongodatabase))
	mux.HandleFunc("GET /user/getNotificationsSettings", auth.WithJWTAuth(h.GetNotificationsSettings, h.mongodatabase))
	mux.HandleFunc("GET /user/searchCreators/{creatorUsername}", auth.WithJWTAuth(h.SearchCreators, h.mongodatabase))
	mux.HandleFunc("GET /user/getTopCreators", auth.WithJWTAuth(h.GetTopCreator, h.mongodatabase))
	mux.HandleFunc("POST /user/addChatSettings", auth.WithJWTAuth(h.AddChatSettings, h.mongodatabase))
	mux.HandleFunc("GET /user/getChatSettings/{userID}", auth.WithJWTAuth(h.GetChatSettings, h.mongodatabase))
	mux.HandleFunc("GET /user/getTransactions/{page}", auth.WithJWTAuth(h.GetTransactions, h.mongodatabase))
	mux.HandleFunc("POST /user/createSubscriptionTier", auth.WithJWTAuth(h.CreateSubscriptionTier, h.mongodatabase))
	mux.HandleFunc("POST /user/updateSubscriptionTier", auth.WithJWTAuth(h.UpdateSubscriptionTier, h.mongodatabase))
	mux.HandleFunc("GET /user/getSubscriptionTier/{creatorID}", auth.WithJWTAuth(h.GetSubscriptionTier, h.mongodatabase))

	//Web
	mux.HandleFunc("POST /ws/createChat", auth.WithJWTAuth(h.CreateChat, h.mongodatabase))
	mux.HandleFunc("POST /ws/createRoom", auth.WithJWTAuth(h.CreateRoom, h.mongodatabase))
	mux.HandleFunc("GET /ws/latestChats/{page}", auth.WithJWTAuth(h.GetLatestsChats, h.mongodatabase))
	mux.HandleFunc("GET /ws/latestRequestChats/{page}", auth.WithJWTAuth(h.GetLatestsRequestChats, h.mongodatabase))
	mux.HandleFunc("GET /ws/Chat/{chatID}", auth.WithJWTAuth(h.GetChat, h.mongodatabase))
	mux.HandleFunc("GET /ws/latestMessages/{chatid}/{page}", auth.WithJWTAuth(h.GetLatestsMessages, h.mongodatabase))
	mux.HandleFunc("POST /ws/uploadWebImage", auth.WithJWTAuth(h.HandleUploadWebImage, h.mongodatabase))
	mux.HandleFunc("POST /ws/acceptChatReq/{chatID}", auth.WithJWTAuth(h.AcceptChatReq, h.mongodatabase))
	mux.HandleFunc("DELETE /ws/acceptChatReq/{chatID}", auth.WithJWTAuth(h.RejectChatReq, h.mongodatabase))

	//Payments
	mux.HandleFunc("GET /payments/getAvailableSubscriptionGroup", auth.WithJWTAuth(h.GetAvailableSubscriptionGroup, h.mongodatabase))
	mux.HandleFunc("POST /payments/createSubscribeToCreator", auth.WithJWTAuth(h.CreateSubscribeToCreator, h.mongodatabase))
}

func (h *handler) JoinRoom(w http.ResponseWriter, r *http.Request) {

	RoomID := r.PathValue("roomID")

	// Prepare the header for the WebSocket dial request
	header := http.Header{}
	// Copy the Authorization header from the original HTTP request
	header.Set("Authorization", r.Header.Get("Authorization"))
	header.Set("token", r.Header.Get("token"))

	tokenQuery := r.URL.Query().Get("token")

	print(tokenQuery)

	connBackend, _, err := websocket.DefaultDialer.Dial("ws://websocket:8080/api/ws/joinRoom/"+RoomID+"?token="+tokenQuery, header)
	if err != nil {
		http.Error(w, "Could not open websocket connection", http.StatusBadRequest)
		return
	}
	defer connBackend.Close()

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket Upgrade error:", err)
		return
	}
	defer conn.Close()

	done := make(chan struct{})
	go func() {
		transferMessages(conn, connBackend)
		done <- struct{}{}
	}()
	go func() {
		transferMessages(connBackend, conn)
		done <- struct{}{}
	}()

	// Wait for either goroutine to finish
	<-done
	// Close both connections when done
	connBackend.Close()
	conn.Close()
}

func transferMessages(src, dst *websocket.Conn) {
	for {
		mt, message, err := src.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket closed unexpectedly: %v", err)
			} else {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}
		if err := dst.WriteMessage(mt, message); err != nil {
			log.Printf("WebSocket write error: %v", err)
			break
		}
	}
}

func (h *handler) HandleGetNotifications(w http.ResponseWriter, r *http.Request) {

	page := r.PathValue("page")
	pagenum, err := strconv.Atoi(page)
	if err != nil {
		fmt.Println("Error converting string to int:", err)
		common.WriteError(w, http.StatusBadRequest, "invalid page number")
		return
	}

	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	pageNum32 := int32(pagenum)

	o, err := h.notifClient.GetLatestNotifications(r.Context(), &pb.GetLatestNotificationsRequest{
		Page:   pageNum32,
		UserID: userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("gRPC Invalid Argument Error: %v", rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("gRPC Internal Server Error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleGetVerificationCode(w http.ResponseWriter, r *http.Request) {

	var payload common.SendMailPayload

	if err := common.ParseJSON(r, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := common.Validate.Struct(payload); err != nil {
		errors := err.(validator.ValidationErrors)
		formattedErrors := common.FormatValidationErrors(errors)
		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
		return
	}

	o, err := h.userClient.SendUserCode(r.Context(), &pb.SendUserCodeRequest{
		Email: payload.Email,
	})
	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("Error: %v, GRPC Status: %v", err, rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("Internal server error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	var payload common.ChangePasswordPayload

	if err := common.ParseJSON(r, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := common.Validate.Struct(payload); err != nil {
		errors := err.(validator.ValidationErrors)
		formattedErrors := common.FormatValidationErrors(errors)
		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
		return
	}

	o, err := h.userClient.ChangePassword(r.Context(), &pb.ChangePasswordRequest{
		Email:    payload.Email,
		Code:     payload.Code,
		Password: payload.Password,
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) forwardToPostService(r *http.Request) (*http.Response, error) {
	// Prepare a new request to the post service
	//env
	postServiceURL := "http://" + "posts:3010" + "/api/posts/createPost"
	req, err := http.NewRequest(r.Method, postServiceURL, r.Body)
	if err != nil {
		return nil, err
	}

	// Copy the authorization header to the new request

	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	req.Header = r.Header

	// Forward the request
	client := &http.Client{}
	return client.Do(req)
}

func (h *handler) HandleCreatePost(w http.ResponseWriter, r *http.Request) {
	// Forward the request to the post service
	resp, err := h.forwardToPostService(r)
	if err != nil {
		log.Printf("Error forwarding request to post service: %v", err)
		common.WriteError(w, http.StatusInternalServerError, "failed to forward request")
		return
	}
	defer resp.Body.Close()

	// Copy the response from the post service to the client
	w.WriteHeader(resp.StatusCode)
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	io.Copy(w, resp.Body)
}

func (h *handler) HandleDeletePost(w http.ResponseWriter, r *http.Request) {

	Post_id := r.PathValue("postID")

	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	o, err := h.postClient.DeletePost(r.Context(), &pb.DeletePostRequest{
		PostID: Post_id,
		UserID: userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("Error: %v, GRPC Status: %v", err, rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("Internal server error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleLikePost(w http.ResponseWriter, r *http.Request) {

	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	var payload common.Like_Dislike_PostPayload

	if err := common.ParseJSON(r, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := common.Validate.Struct(payload); err != nil {
		log.Printf("Validation error: %v", err)
		errors := err.(validator.ValidationErrors)
		formattedErrors := common.FormatValidationErrors(errors)
		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
		return
	}

	o, err := h.postClient.LikePost(r.Context(), &pb.LikePostRequest{
		PostID: payload.Post_id.Hex(),
		UserID: userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("Error: %v, GRPC Status: %v", err, rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("Internal server error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleUnlikePost(w http.ResponseWriter, r *http.Request) {
	Post_id := r.PathValue("postID")
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	o, err := h.postClient.UnlikePost(r.Context(), &pb.LikePostRequest{
		PostID: Post_id,
		UserID: userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("Error: %v, GRPC Status: %v", err, rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("Internal server error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleGetUserLike(w http.ResponseWriter, r *http.Request) {
	Post_id := r.PathValue("postID")
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	o, err := h.postClient.GetUserLike(r.Context(), &pb.GetUserLikeRequest{
		PostID: Post_id,
		UserID: userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("Error: %v, GRPC Status: %v", err, rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("Internal server error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleCreateComment(w http.ResponseWriter, r *http.Request) {

	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	var payload common.CreateCommentPayload

	if err := common.ParseJSON(r, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := common.Validate.Struct(payload); err != nil {
		log.Printf("Validation error: %v", err)
		errors := err.(validator.ValidationErrors)
		formattedErrors := common.FormatValidationErrors(errors)
		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
		return
	}

	log.Printf("amount of gems in gateway: %v", payload.Gems)
	log.Printf("amount of gems in gateway int32: %v", int32(payload.Gems))

	comment := &pb.CreateCommentPayload{
		UserID: userID.Hex(),
		PostID: payload.PostID.Hex(),
		Text:   payload.Text,
		Gems:   int32(payload.Gems),
	}

	o, err := h.postClient.CreateComment(r.Context(), &pb.CreateCommentRequest{
		Comment: comment,
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("Error: %v, GRPC Status: %v", err, rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("Internal server error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleDeleteComment(w http.ResponseWriter, r *http.Request) {

	Comment_id := r.PathValue("commentID")

	o, err := h.postClient.DeleteComment(r.Context(), &pb.DeleteCommentRequest{
		CommentID: Comment_id,
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("Error: %v, GRPC Status: %v", err, rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("Internal server error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleLikeComment(w http.ResponseWriter, r *http.Request) {

	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	var payload common.Like_Dislike_CommentPayload

	if err := common.ParseJSON(r, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := common.Validate.Struct(payload); err != nil {
		log.Printf("Validation error: %v", err)
		errors := err.(validator.ValidationErrors)
		formattedErrors := common.FormatValidationErrors(errors)
		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
		return
	}

	o, err := h.postClient.LikeComment(r.Context(), &pb.LikeCommentRequest{
		CommentID: payload.Comment_id.Hex(),
		UserID:    userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("Error: %v, GRPC Status: %v", err, rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("Internal server error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleUnLikeComment(w http.ResponseWriter, r *http.Request) {
	Comment_id := r.PathValue("commentID")
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	o, err := h.postClient.UnLikeComment(r.Context(), &pb.LikeCommentRequest{
		CommentID: Comment_id,
		UserID:    userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("Error: %v, GRPC Status: %v", err, rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("Internal server error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleGetUserCommentLike(w http.ResponseWriter, r *http.Request) {
	CommentID := r.PathValue("commentID")
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	o, err := h.postClient.GetUserCommentLike(r.Context(), &pb.GetUserCommentLikeRequest{
		CommentID: CommentID,
		UserID:    userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("Error: %v, GRPC Status: %v", err, rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("Internal server error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleCreateReply(w http.ResponseWriter, r *http.Request) {

	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	var payload common.CreateReplyPayload

	if err := common.ParseJSON(r, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := common.Validate.Struct(payload); err != nil {
		log.Printf("Validation error: %v", err)
		errors := err.(validator.ValidationErrors)
		formattedErrors := common.FormatValidationErrors(errors)
		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
		return
	}

	reply := &pb.CreateReplyPayload{
		UserID:    userID.Hex(),
		CommentID: payload.CommentID.Hex(),
		Text:      payload.Text,
	}

	o, err := h.postClient.CreateReply(r.Context(), &pb.CreateReplyRequest{
		Reply: reply,
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("Error: %v, GRPC Status: %v", err, rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("Internal server error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleDeleteReply(w http.ResponseWriter, r *http.Request) {
	Reply_id := r.PathValue("replyID")
	Comment_id := r.PathValue("commentID")

	o, err := h.postClient.DeleteReply(r.Context(), &pb.DeleteReplyRequest{
		ReplyID:   Reply_id,
		CommentID: Comment_id,
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("Error: %v, GRPC Status: %v", err, rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("Internal server error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleGetPost(w http.ResponseWriter, r *http.Request) {
	postID := r.PathValue("postID")

	o, err := h.postClient.GetPost(r.Context(), &pb.GetPostRequest{
		PostID: postID,
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("gRPC Invalid Argument Error: %v", rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("gRPC Internal Server Error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleLatestPosts(w http.ResponseWriter, r *http.Request) {
	page := r.PathValue("page")
	pagenum, err := strconv.Atoi(page)
	if err != nil {
		fmt.Println("Error converting string to int:", err)
		common.WriteError(w, http.StatusBadRequest, "invalid page number")
		return
	}

	pageNum32 := int32(pagenum)

	o, err := h.postClient.GetLatestPosts(r.Context(), &pb.GetLatestPostsRequest{
		Page: pageNum32,
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("gRPC Invalid Argument Error: %v", rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("gRPC Internal Server Error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleLatestComments(w http.ResponseWriter, r *http.Request) {
	postID := r.PathValue("postID")
	page := r.PathValue("page")
	pagenum, err := strconv.Atoi(page)
	if err != nil {
		fmt.Println("Error converting string to int:", err)
		common.WriteError(w, http.StatusBadRequest, "invalid page number")
		return
	}

	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	pageNum32 := int32(pagenum)

	o, err := h.postClient.GetLatestComments(r.Context(), &pb.GetLatestCommentsRequest{
		Page:   pageNum32,
		PostID: postID,
		UserID: userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("gRPC Invalid Argument Error: %v", rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("gRPC Internal Server Error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleLatestReplies(w http.ResponseWriter, r *http.Request) {
	commentID := r.PathValue("commentID")
	page := r.PathValue("page")
	pagenum, err := strconv.Atoi(page)
	if err != nil {
		fmt.Println("Error converting string to int:", err)
		common.WriteError(w, http.StatusBadRequest, "invalid page number")
		return
	}

	pageNum32 := int32(pagenum)

	o, err := h.postClient.GetLatestReplies(r.Context(), &pb.GetLatestRepliesRequest{
		Page:      pageNum32,
		CommentID: commentID,
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("gRPC Invalid Argument Error: %v", rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("gRPC Internal Server Error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) GetLatestUserPosts(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userID")
	page := r.PathValue("page")
	pagenum, err := strconv.Atoi(page)
	if err != nil {
		fmt.Println("Error converting string to int:", err)
		common.WriteError(w, http.StatusBadRequest, "invalid page number")
		return
	}

	pageNum32 := int32(pagenum)

	o, err := h.postClient.GetLatestUserPosts(r.Context(), &pb.GetLatestUserPostsRequest{
		Page:     pageNum32,
		TargetID: userID,
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("gRPC Invalid Argument Error: %v", rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("gRPC Internal Server Error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var payload common.LoginUserPayload

	if err := common.ParseJSON(r, &payload); err != nil {
		log.Printf("HandleLogin: Error parsing JSON payload: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := common.Validate.Struct(payload); err != nil {
		log.Printf("Validation error: %v", err)
		errors := err.(validator.ValidationErrors)
		formattedErrors := common.FormatValidationErrors(errors)
		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
		return
	}
	//////////////
	log.Printf("HandleLogin: Parsed payload: %+v", payload)

	o, err := h.userClient.LoginUser(r.Context(), &pb.LoginUserRequest{
		Email:    payload.Email,
		Password: payload.Password,
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		log.Printf("HandleLogin: gRPC error - code: %v, message: %v", rStatus.Code(), rStatus.Message())
		if rStatus.Code() != codes.InvalidArgument {
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("HandleLogin: Internal server error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleVerifyCode(w http.ResponseWriter, r *http.Request) {
	var payload common.ValidateCodePayload

	if err := common.ParseJSON(r, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := common.Validate.Struct(payload); err != nil {
		errors := err.(validator.ValidationErrors)
		formattedErrors := common.FormatValidationErrors(errors)
		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
		return
	}

	o, err := h.userClient.ValidateUser(r.Context(), &pb.ValidateUserRequest{
		Email: payload.Email,
		Code:  payload.Code,
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var payload common.RegisterUserPayload

	if err := common.ParseJSON(r, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := common.Validate.Struct(payload); err != nil {
		errors := err.(validator.ValidationErrors)
		formattedErrors := common.FormatValidationErrors(errors)
		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
		return
	}

	o, err := h.userClient.RegisterUser(r.Context(), &pb.RegisterUserRequest{
		FirstName: payload.FirstName,
		LastName:  payload.LastName,
		Username:  payload.UserName,
		Email:     payload.Email,
		Password:  payload.Password,
		BirthDate: payload.BirthDate,
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusConflict, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleGetUser(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	o, err := h.userClient.GetUser(r.Context(), &pb.GetUserRequest{
		UserID: userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("gRPC Invalid Argument Error: %v", rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("gRPC Internal Server Error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleGetUserByID(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userID")

	o, err := h.userClient.GetUserByID(r.Context(), &pb.GetUserRequest{
		UserID: userID,
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("gRPC Invalid Argument Error: %v", rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("gRPC Internal Server Error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleUpdateAvatar(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	var payload common.AvatarPayload

	if err := common.ParseJSON(r, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := common.Validate.Struct(payload); err != nil {
		errors := err.(validator.ValidationErrors)
		formattedErrors := common.FormatValidationErrors(errors)
		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
		return
	}

	o, err := h.userClient.UpdateAvatar(r.Context(), &pb.UpdateAvatarRequest{
		Avatar: payload.Avatar,
		UserID: userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) forwardToUserServiceUpdateAvatar(r *http.Request) (*http.Response, error) {
	// Prepare a new request to the post service
	//env
	userServiceURL := "http://" + "user:4010" + "/api/user/updateAvatar"
	req, err := http.NewRequest(r.Method, userServiceURL, r.Body)
	if err != nil {
		return nil, err
	}

	// Copy the authorization header to the new request

	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	req.Header = r.Header

	// Forward the request
	client := &http.Client{}
	return client.Do(req)
}

func (h *handler) UpdateAvatar(w http.ResponseWriter, r *http.Request) {
	// Forward the request to the post service
	resp, err := h.forwardToUserServiceUpdateAvatar(r)
	if err != nil {
		log.Printf("Error forwarding request to post service: %v", err)
		common.WriteError(w, http.StatusInternalServerError, "failed to forward request")
		return
	}
	defer resp.Body.Close()

	// Copy the response from the post service to the client
	w.WriteHeader(resp.StatusCode)
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	io.Copy(w, resp.Body)
}

func (h *handler) HandleUpdateBanner(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	var payload common.BannerPayload

	if err := common.ParseJSON(r, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := common.Validate.Struct(payload); err != nil {
		errors := err.(validator.ValidationErrors)
		formattedErrors := common.FormatValidationErrors(errors)
		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
		return
	}

	o, err := h.userClient.UpdateBanner(r.Context(), &pb.UpdateBannerRequest{
		Banner: payload.Banner,
		UserID: userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) forwardToUserServiceUpdateBanner(r *http.Request) (*http.Response, error) {
	// Prepare a new request to the post service
	//env
	userServiceURL := "http://" + "user:4010" + "/api/user/updateBanner"
	req, err := http.NewRequest(r.Method, userServiceURL, r.Body)
	if err != nil {
		return nil, err
	}

	// Copy the authorization header to the new request

	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	req.Header = r.Header

	// Forward the request
	client := &http.Client{}
	return client.Do(req)
}

func (h *handler) UpdateBanner(w http.ResponseWriter, r *http.Request) {
	// Forward the request to the post service
	resp, err := h.forwardToUserServiceUpdateBanner(r)
	if err != nil {
		log.Printf("Error forwarding request to post service: %v", err)
		common.WriteError(w, http.StatusInternalServerError, "failed to forward request")
		return
	}
	defer resp.Body.Close()

	// Copy the response from the post service to the client
	w.WriteHeader(resp.StatusCode)
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	io.Copy(w, resp.Body)
}

func (h *handler) HandleFollowUser(w http.ResponseWriter, r *http.Request) {

	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	var payload common.FollowUserPayload

	if err := common.ParseJSON(r, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := common.Validate.Struct(payload); err != nil {
		log.Printf("Validation error: %v", err)
		errors := err.(validator.ValidationErrors)
		formattedErrors := common.FormatValidationErrors(errors)
		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
		return
	}

	o, err := h.userClient.FollowUser(r.Context(), &pb.FollowUserRequest{
		TargetID: payload.Follow_id.Hex(),
		UserID:   userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("Error: %v, GRPC Status: %v", err, rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("Internal server error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleUnFollowUser(w http.ResponseWriter, r *http.Request) {

	targetID := r.PathValue("targetID")

	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	o, err := h.userClient.UnFollowUser(r.Context(), &pb.FollowUserRequest{
		TargetID: targetID,
		UserID:   userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("Error: %v, GRPC Status: %v", err, rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("Internal server error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleGetUserFollow(w http.ResponseWriter, r *http.Request) {
	Target_id := r.PathValue("targetID")
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	o, err := h.userClient.GetUserFollow(r.Context(), &pb.GetUserFollowRequest{
		TargetID: Target_id,
		UserID:   userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("Error: %v, GRPC Status: %v", err, rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("Internal server error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleUpdateDescription(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	var payload common.DescriptionPayload

	if err := common.ParseJSON(r, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := common.Validate.Struct(payload); err != nil {
		errors := err.(validator.ValidationErrors)
		formattedErrors := common.FormatValidationErrors(errors)
		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
		return
	}

	o, err := h.userClient.UpdateDescription(r.Context(), &pb.UpdateDescriptionRequest{
		Text:   payload.Description,
		UserID: userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) HandleUpdateUsername(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	var payload common.UsernamePayload

	if err := common.ParseJSON(r, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := common.Validate.Struct(payload); err != nil {
		errors := err.(validator.ValidationErrors)
		formattedErrors := common.FormatValidationErrors(errors)
		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
		return
	}

	o, err := h.userClient.UpdateUsername(r.Context(), &pb.UpdateUsernameRequest{
		Username: payload.Username,
		UserID:   userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) AddExpoToken(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	var payload common.AddExpoTokenPayload

	if err := common.ParseJSON(r, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := common.Validate.Struct(payload); err != nil {
		errors := err.(validator.ValidationErrors)
		formattedErrors := common.FormatValidationErrors(errors)
		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
		return
	}

	o, err := h.userClient.AddExpoToken(r.Context(), &pb.AddExpoTokenRequest{
		ExpoToken: payload.ExpoToken,
		UserID:    userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) DeleteExpoToken(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	Expotoken := r.PathValue("expotoken")

	o, err := h.userClient.DeleteExpoToken(r.Context(), &pb.AddExpoTokenRequest{
		ExpoToken: Expotoken,
		UserID:    userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) UpdateNotificationsSettings(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	var payload common.NotificationsSettingsPayload
	if err := common.ParseJSON(r, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := common.Validate.Struct(payload); err != nil {
		log.Printf("Validation error: %v", err)
		errors := err.(validator.ValidationErrors)
		formattedErrors := common.FormatValidationErrors(errors)
		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
		return
	}

	o, err := h.userClient.UpdateNotificationsSettings(r.Context(), &pb.NotificationsSettings{
		UnreadMessages:   payload.UnreadMessages,
		New_Post:         payload.NewPost,
		New_About_Eskiwi: payload.NewAboutEskiwi,
		New_Sub:          payload.NewSub,
		New_Comment:      payload.NewComment,
		New_Like:         payload.NewLike,
		UserXid:          userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("gRPC Invalid Argument Error: %v", rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("gRPC Internal Server Error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) GetNotificationsSettings(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	o, err := h.userClient.GetUserNotificationSettings(r.Context(), &pb.GetUserNotificationSettingsRequest{
		UserID: userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("gRPC Invalid Argument Error: %v", rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("gRPC Internal Server Error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) SearchCreators(w http.ResponseWriter, r *http.Request) {
	CreatorUsername := r.PathValue("creatorUsername")

	o, err := h.userClient.SearchCreators(r.Context(), &pb.SearchCreatorsRequest{
		Username: CreatorUsername,
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("gRPC Invalid Argument Error: %v", rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("gRPC Internal Server Error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) GetTopCreator(w http.ResponseWriter, r *http.Request) {

	o, err := h.userClient.GetTopCreator(r.Context(), &pb.GetTopCreatorRequest{})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("gRPC Invalid Argument Error: %v", rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("gRPC Internal Server Error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) AddChatSettings(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	var payload common.ChatsSettingsPayload

	if err := common.ParseJSON(r, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := common.Validate.Struct(payload); err != nil {
		errors := err.(validator.ValidationErrors)
		formattedErrors := common.FormatValidationErrors(errors)
		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
		return
	}

	var settings pb.ChatsSettings
	settings.UserId = userID.Hex()
	settings.Price = int32(payload.Price)
	settings.Rules = payload.Rules

	o, err := h.userClient.CreateChatsSettings(r.Context(), &pb.CreateChatsSettingsRequest{
		Chatsettings: &settings,
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) GetChatSettings(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userID")

	o, err := h.userClient.GetChatSettings(r.Context(), &pb.GetChatSettingsRequest{
		UserId: userID,
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("gRPC Invalid Argument Error: %v", rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("gRPC Internal Server Error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) GetTransactions(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	page := r.PathValue("page")
	pagenum, err := strconv.Atoi(page)
	if err != nil {
		fmt.Println("Error converting string to int:", err)
		common.WriteError(w, http.StatusBadRequest, "invalid page number")
		return
	}

	o, err := h.userClient.GetTransactions(r.Context(), &pb.GetTransactionsRequest{
		UserId: userID.Hex(),
		Page:   int32(pagenum),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("gRPC Invalid Argument Error: %v", rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("gRPC Internal Server Error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) CreateSubscriptionTier(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	var payload common.SubscriptionTiers
	if err := common.ParseJSON(r, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := common.Validate.Struct(payload); err != nil {
		log.Printf("Validation error: %v", err)
		errors := err.(validator.ValidationErrors)
		formattedErrors := common.FormatValidationErrors(errors)
		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
		return
	}

	o, err := h.userClient.CreateSubscriptionTier(r.Context(), &pb.SubscriptionTiers{
		CreatorId: userID.Hex(),
		Name:      payload.Name,
		Benefits:  payload.Benefits,
		Tier:      int32(payload.Tier),
		Price:     int32(payload.Price),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("gRPC Invalid Argument Error: %v", rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("gRPC Internal Server Error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) UpdateSubscriptionTier(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	var payload common.SubscriptionTiers
	if err := common.ParseJSON(r, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := common.Validate.Struct(payload); err != nil {
		log.Printf("Validation error: %v", err)
		errors := err.(validator.ValidationErrors)
		formattedErrors := common.FormatValidationErrors(errors)
		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
		return
	}

	o, err := h.userClient.UpdateSubscriptionTier(r.Context(), &pb.SubscriptionTiers{
		CreatorId: userID.Hex(),
		Name:      payload.Name,
		Benefits:  payload.Benefits,
		Tier:      int32(payload.Tier),
		Price:     int32(payload.Price),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("gRPC Invalid Argument Error: %v", rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("gRPC Internal Server Error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) GetSubscriptionTier(w http.ResponseWriter, r *http.Request) {
	creatorID := r.PathValue("creatorID")

	o, err := h.userClient.GetSubscriptionTier(r.Context(), &pb.GetSubscriptionTierRequest{
		CreatorId: creatorID,
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			log.Printf("gRPC Invalid Argument Error: %v", rStatus.Message())
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		log.Printf("gRPC Internal Server Error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) CreateChat(w http.ResponseWriter, r *http.Request) {

	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	var payload common.CreateChatPayload
	if err := common.ParseJSON(r, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := common.Validate.Struct(payload); err != nil {
		log.Printf("Validation error: %v", err)
		errors := err.(validator.ValidationErrors)
		formattedErrors := common.FormatValidationErrors(errors)
		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
		return
	}

	o, err := h.webClient.CreateChat(r.Context(), &pb.CreateChatRequest{
		UserID:    userID.Hex(),
		CreatorID: payload.CreatorID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err == fmt.Errorf("status:chat already exists") {
		common.WriteJSON(w, http.StatusBadRequest, err)
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) CreateRoom(w http.ResponseWriter, r *http.Request) {

	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	var payload common.CreateRoomPayload
	if err := common.ParseJSON(r, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := common.Validate.Struct(payload); err != nil {
		log.Printf("Validation error: %v", err)
		errors := err.(validator.ValidationErrors)
		formattedErrors := common.FormatValidationErrors(errors)
		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
		return
	}

	o, err := h.webClient.CreateRoom(r.Context(), &pb.CreateRoomRequest{
		Id:      payload.ID.Hex(),
		UserID:  userID.Hex(),
		Request: payload.Request,
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) GetLatestsChats(w http.ResponseWriter, r *http.Request) {

	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	page := r.PathValue("page")
	pagenum, err := strconv.Atoi(page)
	if err != nil {
		fmt.Println("Error converting string to int:", err)
		common.WriteError(w, http.StatusBadRequest, "invalid page number")
		return
	}

	o, err := h.webClient.GetLatestsChats(r.Context(), &pb.GetLatestsChatsRequest{
		Page:   int32(pagenum),
		UserID: userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) GetLatestsRequestChats(w http.ResponseWriter, r *http.Request) {

	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	page := r.PathValue("page")
	pagenum, err := strconv.Atoi(page)
	if err != nil {
		fmt.Println("Error converting string to int:", err)
		common.WriteError(w, http.StatusBadRequest, "invalid page number")
		return
	}

	o, err := h.webClient.GetLatestsRequestChats(r.Context(), &pb.GetLatestsChatsRequest{
		Page:   int32(pagenum),
		UserID: userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) GetChat(w http.ResponseWriter, r *http.Request) {

	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	chatID := r.PathValue("chatID")

	o, err := h.webClient.GetChat(r.Context(), &pb.GetChatRequest{
		ChatID: chatID,
		UserID: userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) GetLatestsMessages(w http.ResponseWriter, r *http.Request) {

	page := r.PathValue("page")
	pagenum, err := strconv.Atoi(page)
	if err != nil {
		fmt.Println("Error converting string to int:", err)
		common.WriteError(w, http.StatusBadRequest, "invalid page number")
		return
	}

	chatID := r.PathValue("chatid")

	o, err := h.webClient.GetLatestsMessages(r.Context(), &pb.GetLatestsMessagesRequest{
		Page:   int32(pagenum),
		ChatID: chatID,
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) forwardToWebService(r *http.Request) (*http.Response, error) {
	// Prepare a new request to the post service
	//env
	webServiceURL := "http://" + "websocket:8080" + "/api/ws/uploadChatImage"
	req, err := http.NewRequest(r.Method, webServiceURL, r.Body)
	if err != nil {
		return nil, err
	}

	// Copy the authorization header to the new request

	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	req.Header = r.Header

	// Forward the request
	client := &http.Client{}
	return client.Do(req)
}

func (h *handler) HandleUploadWebImage(w http.ResponseWriter, r *http.Request) {

	// Forward the request to the post service
	resp, err := h.forwardToWebService(r)
	if err != nil {
		log.Printf("Error forwarding request to post service: %v", err)
		common.WriteError(w, http.StatusInternalServerError, "failed to forward request")
		return
	}
	defer resp.Body.Close()

	// Copy the response from the post service to the client
	w.WriteHeader(resp.StatusCode)
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	io.Copy(w, resp.Body)
}

func (h *handler) AcceptChatReq(w http.ResponseWriter, r *http.Request) {

	chatID := r.PathValue("chatID")

	o, err := h.webClient.AcceptChatReq(r.Context(), &pb.AcceptChatReqRequest{
		ChatID: chatID,
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) RejectChatReq(w http.ResponseWriter, r *http.Request) {

	chatID := r.PathValue("chatID")

	o, err := h.webClient.RejectChatReq(r.Context(), &pb.AcceptChatReqRequest{
		ChatID: chatID,
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) GetAvailableSubscriptionGroup(w http.ResponseWriter, r *http.Request) {

	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	o, err := h.paymentsClient.GetAvailableSubscriptionGroup(r.Context(), &pb.GetAvailableSubscriptionGroupRequest{
		UserID: userID.Hex(),
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

func (h *handler) CreateSubscribeToCreator(w http.ResponseWriter, r *http.Request) {

	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	var payload common.CreateSubscriptionPayload
	if err := common.ParseJSON(r, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := common.Validate.Struct(payload); err != nil {
		log.Printf("Validation error: %v", err)
		errors := err.(validator.ValidationErrors)
		formattedErrors := common.FormatValidationErrors(errors)
		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
		return
	}

	o, err := h.paymentsClient.CreateSubscribeToCreator(r.Context(), &pb.CreateSubscribeToCreatorRequest{
		UserID:    userID.Hex(),
		CreatorID: payload.CreatorID.Hex(),
		Tier:      int32(payload.Tier),
		ProductID: payload.ProductID,
	})

	rStatus := status.Convert(err)
	if rStatus != nil {
		if rStatus.Code() != codes.InvalidArgument {
			common.WriteError(w, http.StatusBadRequest, rStatus.Message())
			return
		}
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, o)
	}

	common.WriteJSON(w, http.StatusOK, o)
}

//func (h *handler) forwardToWebSocket(r *http.Request) (*http.Response, error) {
//	// Prepare a new request to the post service
//	//env
//	webServiceURL := "ws://" + "websocket:8080" + "/api/ws/joinRoom/{roomID}"
//	req, err := http.NewRequest(r.Method, webServiceURL, r.Body)
//	if err != nil {
//		return nil, err
//	}
//
//	// Copy the authorization header to the new request
//
//	req.Header.Set("Authorization", r.Header.Get("Authorization"))
//	req.Header = r.Header
//
//	// Forward the request
//	client := &http.Client{}
//	return client.Do(req)
//}

//func (h *handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
//
//	// Forward the request to the post service
//	resp, err := h.forwardToWebService(r)
//	if err != nil {
//		log.Printf("Error forwarding request to post service: %v", err)
//		common.WriteError(w, http.StatusInternalServerError, "failed to forward request")
//		return
//	}
//	defer resp.Body.Close()
//
//	// Copy the response from the post service to the client
//	w.WriteHeader(resp.StatusCode)
//	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
//	io.Copy(w, resp.Body)
//}
