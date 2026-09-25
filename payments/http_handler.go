package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	common "github.com/Eskiwi-Organization/infra/commons"

	"go.mongodb.org/mongo-driver/mongo"
)

var gemMap = map[string]int{
	"100.gems":   100,
	"300.gems":   300,
	"500.gems":   500,
	"1500.gems":  1500,
	"5000.gems":  5000,
	"10000.gems": 10000,
	"30000.gems": 30000,
}

type handler struct {
	mongodbclient *mongo.Client
	mongodatabase *mongo.Database
	store         PaymentsStore
}

func NewHandler(
	mongodbclient *mongo.Client,
	store PaymentsStore,
) *handler {
	return &handler{
		mongodbclient: mongodbclient,
		mongodatabase: mongodbclient.Database("Eskiwi"),
		store:         store,
	}
}

func (h *handler) registerRoutes(mux *http.ServeMux) {

	mux.HandleFunc("POST /buyGems", h.BuyGems)
	mux.HandleFunc("POST /initialSubPurchase", h.InitialSubPurchase)
	mux.HandleFunc("POST /renewal", h.Renewal)
	mux.HandleFunc("POST /cancellation", h.Cancellation)

}

func (h *handler) BuyGems(w http.ResponseWriter, r *http.Request) {

	authToken := r.Header.Get("Authorization")
	if authToken != headerAuth {
		common.WriteJSON(w, http.StatusBadRequest, nil)
		return
	}

	var rcData RCJSON

	if err := common.ParseJSON(r, &rcData); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("Original App User ID: %s", rcData.Event.OriginalAppUserID)
	log.Printf("Original Transaction ID: %v", rcData.Event.OriginalTransactionID)
	log.Printf("Product ID: %s", rcData.Event.ProductID)

	//Se busca la persona en MONGO
	user, err := h.store.GetUserByID(rcData.Event.AppUserID)
	if err != nil {
		log.Printf("error getting user: %v", err)
		common.WriteJSON(w, http.StatusBadRequest, err)
		return
	}
	//Se verifica si ya existe en SQL
	exists, err := h.store.UserExists(user.ID.Hex())
	if err != nil {
		log.Fatalf("error checking user existence: %v", err)
		common.WriteJSON(w, http.StatusBadRequest, err)
		return
	}
	if !exists {
		err := h.store.CreateUser(user)
		if err != nil {
			log.Fatalf("could not create user: %v", err)
			common.WriteJSON(w, http.StatusBadRequest, err)
			return
		}
	}

	//Se saca la info de SQL
	usersql, err := h.store.GetUserByIDsql(user.ID.Hex())
	if err != nil {
		log.Printf("error getting user from sql: %v", err)
		common.WriteJSON(w, http.StatusBadRequest, err)
		return
	}

	log.Printf("userID from sql %v", usersql.ID)

	log.Printf("productid %v", rcData.Event.ProductID)

	log.Printf("gems purchased %v", gemMap[rcData.Event.ProductID])
	newUserGems := usersql.Gems + gemMap[rcData.Event.ProductID]

	log.Printf("gems of user %v", usersql.Gems)

	err = h.store.UpdateUserGems(usersql.ID.Hex(), newUserGems)
	if err != nil {
		log.Printf("error updating user gems: %v", err)
		common.WriteJSON(w, http.StatusBadRequest, err)
		return
	}
	log.Printf("gems updated %v", newUserGems)

	err = h.store.UpdateUserGemsMongo(user.ID, newUserGems)
	if err != nil {
		log.Printf("error updating user gems: %v", err)
		common.WriteJSON(w, http.StatusBadRequest, err)
		return
	}

	purchase := GemPurchase{
		BuyerID:         rcData.Event.AppUserID,
		CreatedAt:       time.Now(),
		RcTransactionID: rcData.Event.OriginalTransactionID, // Transaction ID can be a number, so converting to string
		RcProductID:     rcData.Event.ProductID,
		Platform:        rcData.Event.Store,
		GemAmount:       gemMap[rcData.Event.ProductID], // Convert product_id to gem amount
		Valid:           true,                           // Default to true, can be updated later if needed
	}

	err = h.store.CreateGemPurchase(&purchase)
	if err != nil {
		log.Printf("Error creating gem purchase: %v", err)
	}

	common.WriteJSON(w, http.StatusOK, nil)
}

func (h *handler) InitialSubPurchase(w http.ResponseWriter, r *http.Request) {

	authToken := r.Header.Get("Authorization")
	if authToken != headerAuth {
		common.WriteJSON(w, http.StatusBadRequest, nil)
		return
	}

	var rcData RCJSON

	if err := common.ParseJSON(r, &rcData); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	log.Print("Subscription InitialPurchase")
	log.Printf("Original App User ID: %s", rcData.Event.OriginalAppUserID)
	log.Printf("Original Transaction ID: %v", rcData.Event.OriginalTransactionID)
	log.Printf("Product ID: %s", rcData.Event.ProductID)

	//Se busca la persona en MONGO
	user, err := h.store.GetUserByID(rcData.Event.AppUserID)
	if err != nil {
		log.Printf("error getting user: %v", err)
		common.WriteJSON(w, http.StatusBadRequest, err)
		return
	}

	//Se verifica si ya existe en SQL
	exists, err := h.store.UserExists(user.ID.Hex())
	if err != nil {
		log.Fatalf("error checking user existence: %v", err)
		common.WriteJSON(w, http.StatusBadRequest, err)
		return
	}
	if !exists {
		err := h.store.CreateUser(user)
		if err != nil {
			log.Fatalf("could not create user: %v", err)
			common.WriteJSON(w, http.StatusBadRequest, err)
			return
		}
	}

	subscription, err := h.store.GetSubscription(user.ID, rcData.Event.ProductID)
	if err != nil {
		fmt.Printf("Error fetching subscription: %v\n", err)
		common.WriteJSON(w, http.StatusBadRequest, err)
		return
	}

	err = h.store.ActivateSubscription(user.ID, rcData.Event.ProductID, rcData.Event.OriginalTransactionID)
	if err != nil {
		log.Println("Error activating subscription:", err)
		common.WriteJSON(w, http.StatusBadRequest, err)
		return
	}

	err = h.store.UserSubscriberCount(user.ID, 1)
	if err != nil {
		log.Println("Error increasing subscription count:", err)
		common.WriteJSON(w, http.StatusBadRequest, err)
		return
	}

	purchase := SubPurchase{
		BuyerID:         rcData.Event.AppUserID,
		CreatorID:       subscription.CreatorID.Hex(),
		CreatedAt:       time.Now(),
		RcTransactionID: rcData.Event.OriginalTransactionID, // Transaction ID can be a number, so converting to string
		RcProductID:     rcData.Event.ProductID,
		Platform:        rcData.Event.Store,
		Tier:            subscription.Tier,
		Valid:           true, // Default to true, can be updated later if needed
	}

	err = h.store.CreateSubPurchase(&purchase)
	if err != nil {
		log.Println("Error creating subscription transaction:", err)
		common.WriteJSON(w, http.StatusBadRequest, err)
		return
	}

	log.Print("Subscription activated")
	common.WriteJSON(w, http.StatusOK, nil)
}

func (h *handler) Renewal(w http.ResponseWriter, r *http.Request) {

	authToken := r.Header.Get("Authorization")
	if authToken != headerAuth {
		common.WriteJSON(w, http.StatusBadRequest, nil)
		return
	}

	var rcData RCJSON

	if err := common.ParseJSON(r, &rcData); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	log.Print("Subscription Renewal")
	log.Printf("Original App User ID: %s", rcData.Event.OriginalAppUserID)
	log.Printf("Original Transaction ID: %v", rcData.Event.OriginalTransactionID)
	log.Printf("Product ID: %s", rcData.Event.ProductID)

	//Se busca la persona en MONGO
	user, err := h.store.GetUserByID(rcData.Event.AppUserID)
	if err != nil {
		log.Printf("error getting user: %v", err)
		common.WriteJSON(w, http.StatusBadRequest, err)
		return
	}

	//Se verifica si ya existe en SQL
	exists, err := h.store.UserExists(user.ID.Hex())
	if err != nil {
		log.Fatalf("error checking user existence: %v", err)
		common.WriteJSON(w, http.StatusBadRequest, err)
		return
	}
	if !exists {
		err := h.store.CreateUser(user)
		if err != nil {
			log.Fatalf("could not create user: %v", err)
			common.WriteJSON(w, http.StatusBadRequest, err)
			return
		}
	}

	subscription, err := h.store.GetSubscription(user.ID, rcData.Event.ProductID)
	if err != nil {
		fmt.Printf("Error fetching subscription: %v\n", err)
		common.WriteJSON(w, http.StatusBadRequest, err)
		return
	}

	err = h.store.RenewActivateSubscription(user.ID, rcData.Event.ProductID, rcData.Event.OriginalTransactionID)
	if err != nil {
		log.Println("Error activating subscription:", err)
		common.WriteJSON(w, http.StatusBadRequest, err)
		return
	}

	purchase := SubPurchase{
		BuyerID:         rcData.Event.AppUserID,
		CreatorID:       subscription.CreatorID.Hex(),
		CreatedAt:       time.Now(),
		RcTransactionID: rcData.Event.OriginalTransactionID, // Transaction ID can be a number, so converting to string
		RcProductID:     rcData.Event.ProductID,
		Platform:        rcData.Event.Store,
		Tier:            subscription.Tier,
		Valid:           true, // Default to true, can be updated later if needed
	}

	err = h.store.CreateSubPurchase(&purchase)
	if err != nil {
		log.Println("Error creating subscription transaction:", err)
		common.WriteJSON(w, http.StatusBadRequest, err)
		return
	}

	common.WriteJSON(w, http.StatusOK, nil)
}

func (h *handler) Cancellation(w http.ResponseWriter, r *http.Request) {

	authToken := r.Header.Get("Authorization")
	if authToken != headerAuth {
		common.WriteJSON(w, http.StatusBadRequest, nil)
		return
	}

	var rcData RCJSON

	if err := common.ParseJSON(r, &rcData); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		common.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("Original App User ID: %s", rcData.Event.OriginalAppUserID)
	log.Printf("Original Transaction ID: %v", rcData.Event.OriginalTransactionID)
	log.Printf("Product ID: %s", rcData.Event.ProductID)

	//Se busca la persona en MONGO
	user, err := h.store.GetUserByID(rcData.Event.AppUserID)
	if err != nil {
		log.Printf("error getting user: %v", err)
		common.WriteJSON(w, http.StatusBadRequest, err)
		return
	}

	//Se verifica si ya existe en SQL
	exists, err := h.store.UserExists(user.ID.Hex())
	if err != nil {
		log.Fatalf("error checking user existence: %v", err)
		common.WriteJSON(w, http.StatusBadRequest, err)
		return
	}
	if !exists {
		err := h.store.CreateUser(user)
		if err != nil {
			log.Fatalf("could not create user: %v", err)
			common.WriteJSON(w, http.StatusBadRequest, err)
			return
		}
	}

	//Se saca la info de SQL
	//usersql, err := h.store.GetUserByIDsql(user.ID.Hex())
	//if err != nil {
	//	log.Printf("error getting user from sql: %v", err)
	//	common.WriteJSON(w, http.StatusBadRequest, err)
	//	return
	//}
	//
	/////////////FALTA CREAR TRANSACCION
	//

	err = h.store.CancelActivateSubscription(user.ID, rcData.Event.ProductID, rcData.Event.OriginalTransactionID)
	if err != nil {
		log.Println("Error canceling subscription:", err)
		common.WriteJSON(w, http.StatusBadRequest, err)
	}

	err = h.store.UserSubscriberCount(user.ID, -1)
	if err != nil {
		log.Println("Error increasing subscription count:", err)
		common.WriteJSON(w, http.StatusBadRequest, err)
	}

	common.WriteJSON(w, http.StatusOK, nil)
}
