package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	common "github.com/Eskiwi-Organization/infra/commons"
	"github.com/Eskiwi-Organization/infra/commons/auth"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/mongo"
)

type handler struct {
	//notificationsClient pb.NotificationServiceClient
	service       UserService
	store         UserStore
	mongodatabase *mongo.Database
	mongodbclient *mongo.Client
	channel       *amqp.Channel
}

func NewHandler(
	service UserService,
	store UserStore,
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
	mux.HandleFunc("POST /api/user/updateAvatar", auth.WithJWTAuth(h.UpdateAvatar, h.mongodatabase))
	mux.HandleFunc("POST /api/user/updateBanner", auth.WithJWTAuth(h.UpdateBanner, h.mongodatabase))

}

func (h *handler) UpdateAvatar(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
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
	if err := r.ParseMultipartForm(2 << 20); err != nil { // 2 MB max memory
		http.Error(w, "Failed to parse multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Extract and save images
	files := r.MultipartForm.File["avatar"]

	for _, fileHeader := range files {
		if fileHeader.Size > 2<<20 { // 4 MB in bytes
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
		return
	}

	url := contentUrls[0]

	err = h.store.UpdateAvatar(userID, url)
	if err != nil {
		common.WriteError(w, http.StatusInternalServerError, "Error Avatar update: "+err.Error())
		return
	}

	// Respond with success message
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Avatar updated: %s", url)))

}

func (h *handler) UpdateBanner(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
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
	if err := r.ParseMultipartForm(2 << 20); err != nil { // 4 MB max memory
		http.Error(w, "Failed to parse multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Extract and save images
	files := r.MultipartForm.File["banner"]

	for _, fileHeader := range files {
		if fileHeader.Size > 2<<20 { // 4 MB in bytes
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
		return
	}

	url := contentUrls[0]

	err = h.store.UpdateBanner(userID, url)
	if err != nil {
		common.WriteError(w, http.StatusInternalServerError, "Error Banner update: "+err.Error())
		return
	}

	// Respond with success message
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Banner updated: %s", url)))

}
