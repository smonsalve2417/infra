package main

import (
	"log"
	"net/http"
	"strings"

	common "github.com/Eskiwi-Organization/infra/commons"
	"github.com/Eskiwi-Organization/infra/commons/auth"
	"github.com/gorilla/websocket"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type handler struct {
	hub           *Hub
	mongodbclient *mongo.Client
	mongodatabase *mongo.Database
	store         WebStore
	channel       *amqp.Channel
}

func NewHandler(
	mongodbclient *mongo.Client,
	hub *Hub,
	store WebStore, channel *amqp.Channel,
) *handler {
	return &handler{
		mongodbclient: mongodbclient,
		mongodatabase: mongodbclient.Database("Eskiwi"),
		hub:           hub,
		store:         store,
		channel:       channel,
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (h *handler) registerRoutes(mux *http.ServeMux) {

	mux.HandleFunc("GET /api/ws/joinRoom/{roomID}", auth.WithJWTAuth(h.JoinRoom, h.mongodatabase))
	mux.HandleFunc("POST /api/ws/uploadChatImage", auth.WithJWTAuth(h.UploadChatImage, h.mongodatabase))
	//mux.HandleFunc("GET /api/ws/latestChats/{page}", auth.WithJWTAuth(h.GetLatestsChats, h.mongodatabase))
	//mux.HandleFunc("GET /api/ws/latestMessages/{chatid}/{page}", auth.WithJWTAuth(h.GetLatestsMessages, h.mongodatabase))
	//mux.HandleFunc("POST /api/ws/createChat", auth.WithJWTAuth(h.CreateChat, h.mongodatabase))
	//mux.HandleFunc("POST /api/ws/createRoom", auth.WithJWTAuth(h.CreateRoom, h.mongodatabase))

}

//func (h *handler) CreateChat(w http.ResponseWriter, r *http.Request) {
//
//	userID, err := auth.GetUserIDFromContext(r.Context())
//	if err != nil {
//		log.Printf("Unauthorized access: %v", err)
//		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
//		return
//	}
//
//	var payload CreateChatPayload
//	if err := common.ParseJSON(r, &payload); err != nil {
//		log.Printf("Error parsing JSON: %v", err)
//		common.WriteError(w, http.StatusBadRequest, err.Error())
//		return
//	}
//	if err := common.Validate.Struct(payload); err != nil {
//		log.Printf("Validation error: %v", err)
//		errors := err.(validator.ValidationErrors)
//		formattedErrors := common.FormatValidationErrors(errors)
//		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
//		return
//	}
//
//	err, chatExists := h.store.CreateChat(Chat{
//		CreatorID: payload.CreatorID,
//		UserID:    userID,
//		CreatedAt: time.Now(),
//		Request:   true,
//		Pending:   false,
//	})
//	if err != nil {
//		common.WriteError(w, http.StatusBadRequest, "error: "+err.Error())
//		return
//	}
//	if chatExists {
//		common.WriteError(w, http.StatusBadRequest, "status:chat already exists")
//		return
//	}
//
//	chatID, err := h.store.GetChatID(userID, payload.CreatorID)
//	if err != nil {
//		common.WriteError(w, http.StatusBadRequest, "error: "+err.Error())
//		return
//	}
//
//	var response CreateChatResponse
//	response.ChatID = chatID
//
//	common.WriteJSON(w, http.StatusOK, response)
//}

//func (h *handler) CreateRoom(w http.ResponseWriter, r *http.Request) {
//	var req CreateRoomPayload
//	if err := common.ParseJSON(r, &req); err != nil {
//		log.Printf("Error parsing JSON: %v", err)
//		common.WriteError(w, http.StatusBadRequest, err.Error())
//		return
//	}
//	if err := common.Validate.Struct(req); err != nil {
//		log.Printf("Validation error: %v", err)
//		errors := err.(validator.ValidationErrors)
//		formattedErrors := common.FormatValidationErrors(errors)
//		common.WriteError(w, http.StatusBadRequest, "invalid payload: "+formattedErrors)
//		return
//	}
//
//	if h.hub.Rooms[req.ID.Hex()] != nil {
//		response := CreateRoomResponse{Created: true}
//		common.WriteJSON(w, http.StatusOK, response)
//		return
//	}
//
//	h.hub.Rooms[req.ID.Hex()] = &Room{
//		ID:      req.ID.Hex(),
//		Clients: make(map[string]*Client),
//		Request: req.Request,
//	}
//	response := CreateRoomResponse{Created: false}
//
//	common.WriteJSON(w, http.StatusOK, response)
//}

func (h *handler) JoinRoom(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "upgrader fail: "+err.Error())
		return
	}

	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		log.Printf("Unauthorized access: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	roomID := r.PathValue("roomID")

	chatObjID, err := primitive.ObjectIDFromHex(roomID)
	if err != nil {
		log.Printf("Error converting chatid to primitive.ObjetctID: %v", err)
		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
		return
	}

	clientID := userID.String()

	client := &Client{
		conn:    conn,
		Message: make(chan *Message, 10),
		ID:      clientID,
		UserID:  userID,
		RoomID:  roomID,
		Chatid:  chatObjID,
		store:   h.store,
		channel: h.channel,
	}

	log.Printf("A new user has joined the room: %v", userID)

	h.hub.register <- client

	go client.writeMessage()
	client.readMessage(h.hub)
}

func (h *handler) UploadChatImage(w http.ResponseWriter, r *http.Request) {

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
	if err := r.ParseMultipartForm(64 << 20); err != nil { // 32 MB max memory
		http.Error(w, "Failed to parse multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Extract and save images
	files := r.MultipartForm.File["images"]

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
	}

	var response ChatImagesURLs

	response.URLs = contentUrls

	common.WriteJSON(w, http.StatusOK, response)

}

//func (h *handler) GetLatestsChats(w http.ResponseWriter, r *http.Request) {
//	page := r.PathValue("page")
//	pagenum, err := strconv.Atoi(page)
//	if err != nil {
//		fmt.Println("Error converting string to int:", err)
//		common.WriteError(w, http.StatusBadRequest, "invalid page number")
//		return
//	}
//
//	userID, err := auth.GetUserIDFromContext(r.Context())
//	if err != nil {
//		log.Printf("Unauthorized access: %v", err)
//		common.WriteError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
//		return
//	}
//
//	list, err := h.store.GetLatestsChats(pagenum, 10, userID)
//	if err != nil {
//		common.WriteJSON(w, http.StatusBadRequest, err)
//	}
//
//	common.WriteJSON(w, http.StatusOK, list)
//}

// yhftdytrdtyrdtyd
//func (h *handler) GetLatestsMessages(w http.ResponseWriter, r *http.Request) {
//	page := r.PathValue("page")
//	pagenum, err := strconv.Atoi(page)
//	if err != nil {
//		fmt.Println("Error converting string to int:", err)
//		common.WriteError(w, http.StatusBadRequest, "invalid page number")
//		return
//	}
//
//	chatID := r.PathValue("chatid")
//	chatObjID, err := primitive.ObjectIDFromHex(chatID)
//	if err != nil {
//		fmt.Println("Error converting chatid to objectID:", err)
//		common.WriteError(w, http.StatusBadRequest, "invalid chatID")
//		return
//	}
//
//	list, err := h.store.UpdateAndRetrievePaginatedMessages(chatObjID, pagenum, 10)
//	if err != nil {
//		common.WriteJSON(w, http.StatusBadRequest, err)
//		return
//	}
//
//	common.WriteJSON(w, http.StatusOK, list)
//}
