package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	common "github.com/Eskiwi-Organization/infra/commons"
	"github.com/Eskiwi-Organization/infra/commons/auth"
	broker "github.com/Eskiwi-Organization/infra/commons/broker"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/mongo"
)

type handler struct {
	//notificationsClient pb.NotificationServiceClient
	service       PostService
	store         PostStore
	mongodatabase *mongo.Database
	mongodbclient *mongo.Client
	channel       *amqp.Channel
}

func NewHandler(
	service PostService,
	store PostStore,
	mongodbclient *mongo.Client,
	channel *amqp.Channel,
) *handler {
	return &handler{
		service:       service,
		store:         store,
		mongodbclient: mongodbclient,
		mongodatabase: mongodbclient.Database("Eskiwi"),
		channel:       channel,
	}
}

func (h *handler) registerRoutes(mux *http.ServeMux) {

	//Notifications
	mux.HandleFunc("POST /api/posts/createPost", auth.WithJWTAuth(h.HandleCreatePost, h.mongodatabase))

}

func (h *handler) HandleCreatePost(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	user, err := h.store.GetUserByID(userID.Hex())
	if err != nil {
		log.Printf("error getting user: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if !h.store.IsCreator(*user) {
		log.Print("Unauthorized access: not a creator")
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: not a creator")
		return
	}

	// Check and log content type
	contentType := r.Header.Get("Content-Type")
	log.Printf("Content-Type: %s", contentType)
	if contentType == "" {
		http.Error(w, "Content-Type header is missing", http.StatusBadRequest)
		return
	}

	// Ensure Content-Type is multipart/form-data
	if contentType != "multipart/form-data" && !strings.HasPrefix(contentType, "multipart/form-data;") {
		http.Error(w, "Invalid Content-Type. Expected multipart/form-data", http.StatusBadRequest)
		return
	}
	// Parse the multipart form
	if err := r.ParseMultipartForm(4 << 20); err != nil {
		http.Error(w, "Failed to parse multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Extract text fields
	title := r.FormValue("title")
	description := r.FormValue("description")
	tier := r.FormValue("tier")
	tags := r.Form["tags"]

	tiernum, err := strconv.Atoi(tier)
	if err != nil {
		fmt.Println("Error converting string to int:", err)
		common.WriteError(w, http.StatusBadRequest, "invalid tier number")
		return
	}

	if tiernum < 0 || tiernum > 5 {
		log.Printf("The number is not between 0 and 5 : %v", tiernum)
		common.WriteError(w, http.StatusBadRequest, "invalid tier number")
		return
	}

	// Log for debugging
	log.Printf("Received Post - Title: %s, Description: %s, Tags: %v", title, description, tags)

	// Extract and save images normaly
	files := r.MultipartForm.File["images"]

	for _, fileHeader := range files {
		if fileHeader.Size > 4<<20 { // 4 MB in bytes
			http.Error(w, "File too large. Maximum allowed file size is 4MB", http.StatusBadRequest)
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, "Error opening file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		// Log the accepted file size
		log.Printf("Uploaded File: %s, Size: %d bytes", fileHeader.Filename, fileHeader.Size)
	}

	contentUrls, err := h.store.UploadMultiPartContent(files)
	if err != nil {
		common.WriteError(w, http.StatusInternalServerError, "Error in MultiPartContent upload: "+err.Error())
	}

	bluredFiles := r.MultipartForm.File["bluredImages"]

	for _, fileHeader := range bluredFiles {
		if fileHeader.Size > 4<<20 { // 4 MB in bytes
			http.Error(w, "File too large. Maximum allowed file size is 4MB", http.StatusBadRequest)
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, "Error opening file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		// Log the accepted file size
		log.Printf("Uploaded File: %s, Size: %d bytes", fileHeader.Filename, fileHeader.Size)
	}

	bluredContentUrls, err := h.store.UploadMultiPartContent(bluredFiles)
	if err != nil {
		common.WriteError(w, http.StatusInternalServerError, "Error in MultiPartContent upload: "+err.Error())
	}

	PostID, err := h.store.CreatePosts(common.Post{
		Title:         title,
		Description:   description,
		User_id:       userID,
		Content:       contentUrls,
		BluredContent: bluredContentUrls, ////////////////////////////////////////////////////////////////////////////////
		Tags:          tags,
		Gems:          0,
		Likes:         0,
		Comments:      0,
		Shares:        0,
		Saves:         0,
		CreatedAt:     time.Now(),
		Tier:          tiernum,
		Hidden:        false,
		Impression:    0,
	})
	if err != nil {
		common.WriteError(w, http.StatusInternalServerError, "Error in post upload: "+err.Error())
	}

	var payload common.Post

	payload.ID = PostID
	payload.User_id = userID

	marshalledRequest, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	q, err := h.channel.QueueDeclare(broker.SendPostsNotificationCreatedEvent, true, false, false, false, nil)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		common.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.channel.PublishWithContext(context.Background(), "", q.Name, false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         marshalledRequest,
		DeliveryMode: amqp.Persistent,
	})

	// Respond with success message
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Post created with title: %s", title)))

}
