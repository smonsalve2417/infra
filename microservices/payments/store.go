package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	common "github.com/Eskiwi-Organization/infra/commons"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type store struct {
	client   *mongo.Client
	database *mongo.Database
	db       *sql.DB
}

var QueryTimeoutDuration = 10 * time.Second
var ctx = context.Background()

func NewStore(client *mongo.Client, db *sql.DB) *store {
	return &store{client: client, database: client.Database(mongoDatabaseName), db: db}
}

func (s *store) CreateUser(user *common.User) error {
	query := `
        INSERT INTO users (user_id, first_name, last_name, email, username, gems)
        VALUES (?, ?, ?, ?, ?, ?)
    `

	// Set a context timeout for the query
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	// Use ExecContext instead of QueryRowContext
	_, err := s.db.ExecContext(
		ctx,
		query,
		user.ID.Hex(), // Assuming ID is a MongoDB ObjectID that you convert to a string
		user.FirstName,
		user.LastName,
		user.Email,
		user.UserName,
		0,
	)

	if err != nil {
		return err
	}

	return nil
}

func (s *store) UserExists(userID string) (bool, error) {
	query := `
        SELECT EXISTS (
            SELECT 1 FROM users WHERE user_id = ?
        )
    `

	// Set a context timeout for the query
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var exists bool

	// Execute the query and scan the result into the 'exists' variable
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (s *store) CreateGemDonation(donation *GemDonation) error {
	query := `
        INSERT INTO gem_donations (sender_id, receiver_id, post_id, comment_id, created_at, valid, gem_amount)
        VALUES (?, ?, ?, ?, ?, ?, ?)
    `

	// Set a context timeout for the query
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	// Use ExecContext for queries that don't return rows
	result, err := s.db.ExecContext(
		ctx,
		query,
		donation.SenderID,
		donation.ReceiverID,
		donation.PostID,
		donation.CommentID,
		donation.CreatedAt,
		donation.Valid,
		donation.GemAmount,
	)

	if err != nil {
		return fmt.Errorf("error creating donation in sql: %v", err)
	}

	// Retrieve the last inserted transaction_id
	transactionID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("error fetching last insert id: %v", err)
	}

	donation.TransactionID = transactionID
	return nil
}

func (s *store) GetPostByID(postID primitive.ObjectID) (*common.Post, error) {
	collection := s.database.Collection("posts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": postID, "hidden": false}
	var post common.Post

	err := collection.FindOne(ctx, filter).Decode(&post)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("post not found: id %s", postID)
		}
		return nil, err
	}
	return &post, nil
}

func (s *store) GetUserByIDsql(userID string) (*common.User, error) {
	query := `
        SELECT user_id, first_name, last_name, email, username, gems
        FROM users
        WHERE user_id = ?
    `

	// Set a context timeout for the query
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	user := &common.User{}

	var userIDString string
	// Execute the query and scan the result into the user struct
	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&userIDString,
		&user.FirstName,
		&user.LastName,
		&user.Email,
		&user.UserName,
		&user.Gems,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user with id %v not found", userID)
		}
		return nil, err
	}

	objectID, err := primitive.ObjectIDFromHex(userIDString)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id format: %v", err)
	}
	user.ID = objectID

	return user, nil
}

func (s *store) UpdateUserGems(userID string, newGems int) error {
	query := `
        UPDATE users
        SET gems = ?
        WHERE user_id = ?
    `

	// Clean up the userID before passing it
	userID = strings.TrimSpace(strings.ToLower(userID))
	// Log the userID and gems for debugging
	log.Printf("Updating gems for user: %s, newGems: %d", userID, newGems)

	// Set a context timeout for the query
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeoutDuration)
	defer cancel()

	// Begin the transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}

	// Execute the update query within the transaction
	log.Printf("Executing query: UPDATE users SET gems = %d WHERE LOWER(userID) = LOWER('%s')", newGems, userID)

	result, err := tx.ExecContext(ctx, query, newGems, userID)
	if err != nil {
		tx.Rollback() // Rollback in case of error
		log.Printf("Error executing query: %v", err)
		return fmt.Errorf("error updating gems for user %v: %v", userID, err)
	}

	// Check if any row was actually updated
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		tx.Rollback() // Rollback in case of error
		log.Printf("Error checking rows affected: %v", err)
		return err
	}
	if rowsAffected == 0 {
		tx.Rollback() // Rollback since no row was found to update
		log.Printf("No rows updated for user: %s", userID)
		return fmt.Errorf("user with userID %v not found", userID)
	}

	// Commit the transaction after successful update
	err = tx.Commit()
	if err != nil {
		log.Printf("Error committing transaction: %v", err)
		return fmt.Errorf("error committing transaction: %v", err)
	}

	log.Printf("Successfully updated gems for user %s to %d", userID, newGems)
	return nil
}

func (s *store) GetUserByID(id string) (*common.User, error) {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Convert the string id to a primitive.ObjectID
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, fmt.Errorf("invalid id: %s", id)
	}

	filter := bson.M{"_id": objectID}
	var user common.User

	err = collection.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("user not found: id %s", id)
		}
		return nil, err
	}
	return &user, nil
}

func (s *store) CreateChatGem(chatGem *ChatGem) error {
	query := `
        INSERT INTO chat_gems (chat_id,sender_id, receiver_id, created_at, valid,denied,gem_amount, accepted)
        VALUES (?,?, ?, ?, ?, ?, ?, ?)
    `

	// Set a context timeout for the query
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeoutDuration)
	defer cancel()

	// Use ExecContext for queries that don't return rows
	result, err := s.db.ExecContext(
		ctx,
		query,
		chatGem.ChatID,
		chatGem.SenderID,   // Convert ObjectID to hex string (if using Mongo-style ObjectID)
		chatGem.ReceiverID, // Convert ObjectID to hex string
		chatGem.CreatedAt,
		chatGem.Valid,
		chatGem.Denied,
		chatGem.GemAmount,
		chatGem.Accepted,
	)

	if err != nil {
		return fmt.Errorf("error creating chat gem in sql: %v", err)
	}

	// Retrieve the last inserted transaction_id
	transactionID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("error fetching last insert id: %v", err)
	}

	chatGem.TransactionID = transactionID

	return nil
}

func (s *store) ChatGemExists(senderID string, receiverID string) (bool, error) {
	query := `
        SELECT COUNT(*)
        FROM chat_gems
        WHERE sender_id = ? AND receiver_id = ?
    `

	// Set a context timeout for the query
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeoutDuration)
	defer cancel()

	var count int

	// Execute the query to count how many matching records exist
	err := s.db.QueryRowContext(ctx, query, senderID, receiverID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("error checking chat gem existence: %v", err)
	}

	// If count is greater than 0, the record exists
	return count > 0, nil
}

func (s *store) UpdateUserGemsMongo(userID primitive.ObjectID, gems int) error {
	collection := s.database.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": userID}
	update := bson.M{"$set": bson.M{"gems": gems}}

	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) GetParticipantsByChatID(chatID primitive.ObjectID) (primitive.ObjectID, primitive.ObjectID, error) {
	collection := s.database.Collection("chats")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Define the filter to find the chat by _id
	filter := bson.M{"_id": chatID}

	// Define a struct to hold the result
	var result struct {
		CreatorID primitive.ObjectID `bson:"creator_id"`
		UserID    primitive.ObjectID `bson:"user_id"`
	}

	// Find the document and decode only the creator_id field
	err := collection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return primitive.NilObjectID, primitive.NilObjectID, fmt.Errorf("no chat found with ID: %v", chatID)
		}
		return primitive.NilObjectID, primitive.NilObjectID, fmt.Errorf("error retrieving chat: %v", err)
	}

	return result.CreatorID, result.UserID, nil
}

func (s *store) FindAndAcceptChatGem(senderID, receiverID string) (int, error) {
	query := `
        SELECT gem_amount 
        FROM chat_gems
        WHERE sender_id = ? AND receiver_id = ?
    `

	// Set a context timeout for the query
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeoutDuration)
	defer cancel()

	var gemAmount int

	// Query the gem_amount by sender_id and receiver_id
	err := s.db.QueryRowContext(ctx, query, senderID, receiverID).Scan(&gemAmount)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("no chat gem found between sender %s and receiver %s", senderID, receiverID)
		}
		return 0, fmt.Errorf("error querying chat gem: %v", err)
	}

	// Update the 'accepted' field to true (or 1) for the same sender_id and receiver_id
	updateQuery := `
        UPDATE chat_gems
        SET accepted = 1
        WHERE sender_id = ? AND receiver_id = ?
    `

	_, err = s.db.ExecContext(ctx, updateQuery, senderID, receiverID)
	if err != nil {
		return 0, fmt.Errorf("error updating chat gem to accepted: %v", err)
	}

	return gemAmount, nil
}

func (s *store) CreateMessageGem(ctx context.Context, messageGem *MessageGem) error {
	query := `
        INSERT INTO message_gems (chat_id, sender_id, receiver_id, created_at, valid, gem_amount)
        VALUES (?, ?, ?, ?, ?, ?)
    `

	// Set created_at to current time
	messageGem.CreatedAt = time.Now()

	// Use ExecContext for queries that don't return rows
	result, err := s.db.ExecContext(
		ctx,
		query,
		messageGem.ChatID, // New chat_id field
		messageGem.SenderID,
		messageGem.ReceiverID,
		messageGem.CreatedAt,
		messageGem.Valid, // Default true
		messageGem.GemAmount,
	)
	if err != nil {
		return fmt.Errorf("error creating message gem in sql: %v", err)
	}

	// Retrieve the last inserted transaction_id
	transactionID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("error fetching last insert id: %v", err)
	}

	// Set the TransactionID in the struct
	messageGem.TransactionID = transactionID

	return nil
}

func (s *store) CreateGemPurchase(purchase *GemPurchase) error {
	query := `
        INSERT INTO gem_purchases (buyer_id, created_at,rc_transaction_id, rc_product_id, platform, gem_amount, valid)
        VALUES (?, ?, ?, ?,  ?, ?, ?)
    `

	// Set a context timeout for the query
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeoutDuration)
	defer cancel()

	// Execute the query
	_, err := s.db.ExecContext(
		ctx,
		query,
		purchase.BuyerID,
		purchase.CreatedAt,
		purchase.RcTransactionID,
		purchase.RcProductID,
		purchase.Platform,
		purchase.GemAmount,
		purchase.Valid,
	)

	if err != nil {
		return fmt.Errorf("error creating gem purchase: %v", err)
	}

	return nil
}

func (s *store) UpdatePostGemsMongo(postID primitive.ObjectID, gems int) error {
	collection := s.database.Collection("posts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Filter for the post by its ObjectID
	filter := bson.M{"_id": postID}

	// Use the $inc operator to increment the "gems" field by the specified amount
	update := bson.M{"$inc": bson.M{"gems": gems}}

	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) GetChatGemAmount(senderID, receiverID string) (int, error) {
	query := `
        SELECT gem_amount
        FROM chat_gems
        WHERE sender_id = ? AND receiver_id = ?
    `

	// Set a context timeout for the query
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeoutDuration)
	defer cancel()

	// Variable to store the retrieved gem_amount
	var gemAmount int

	// Execute the query
	err := s.db.QueryRowContext(ctx, query, senderID, receiverID).Scan(&gemAmount)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("no chat gem found for sender %v and receiver %v", senderID, receiverID)
		}
		return 0, fmt.Errorf("error retrieving gem amount: %v", err)
	}

	// Return the retrieved gem amount
	return gemAmount, nil
}

func (s *store) DenyChatGem(chatID string) error {
	// Set a context timeout for the query
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeoutDuration)
	defer cancel()

	// Update the 'denied' field to true (1) and 'valid' to false (0)
	updateQuery := `
        UPDATE chat_gems
        SET denied = 1, valid = 0
        WHERE chat_id = ? 
    `

	_, err := s.db.ExecContext(ctx, updateQuery, chatID)
	if err != nil {
		return fmt.Errorf("error updating chat gem to denied and invalid: %v", err)
	}

	return nil
}

func (s *store) GetProductIDsByUserID(userID primitive.ObjectID) ([]string, error) {
	collection := s.database.Collection("user-subscriptions")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Define the filter to search by userID
	filter := bson.M{"user_id": userID}

	// Define a slice to store the product IDs
	var productIDs []string

	// Find all subscriptions that match the filter
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("error finding subscriptions: %v", err)
	}
	defer cursor.Close(ctx)

	// Iterate through the cursor and collect all ProductID values
	for cursor.Next(ctx) {
		var subscription common.Subscription
		if err := cursor.Decode(&subscription); err != nil {
			return nil, fmt.Errorf("error decoding subscription: %v", err)
		}

		// Append the ProductID to the list
		productIDs = append(productIDs, subscription.ProductID)
	}

	// Check if any documents were found
	if len(productIDs) == 0 {
		return nil, fmt.Errorf("no subscriptions found for user %s", userID.Hex())
	}

	return productIDs, nil
}

func (s *store) ExtractGroupNumber(word string) (string, error) {
	// Define the regular expression pattern
	pattern := `Group(\d+)_`

	// Compile the regular expression
	re := regexp.MustCompile(pattern)

	// Find the match in the string
	match := re.FindStringSubmatch(word)

	// If there is a match, return the first capture group
	if len(match) > 1 {
		return match[1], nil
	}

	return "", fmt.Errorf("no match found")
}

func (s *store) FindSmallestMissingNumberFromProductIDs(productIDs []string) (int, error) {
	// Regular expression to extract the number between "Group" and "Tier"
	re := regexp.MustCompile(`Group(\d+)_Tier_\d+`)

	var numbers []int
	for _, id := range productIDs {
		// Use regex to extract the number between "Group" and "Tier"
		matches := re.FindStringSubmatch(id)
		if len(matches) < 2 {
			return 0, fmt.Errorf("invalid product ID format: %s", id)
		}

		// Convert the extracted string to an integer
		number, err := strconv.Atoi(matches[1]) // matches[1] is the number between "Group" and "Tier"
		if err != nil {
			return 0, fmt.Errorf("error converting extracted number to integer: %v", err)
		}
		numbers = append(numbers, number)
	}

	// Create a map to store the numbers for quick lookup
	numberSet := make(map[int]bool)

	// Add all numbers to the set
	for _, num := range numbers {
		numberSet[num] = true
	}

	// Start from 1 and find the smallest number not in the set
	smallestMissing := 1
	for {
		// If the current number is not in the set, return it
		if !numberSet[smallestMissing] {
			return smallestMissing, nil
		}
		// Otherwise, increment and check the next number
		smallestMissing++
	}
}

func (s *store) GetSubscriptionTierByCreatorIDAndTier(creatorID primitive.ObjectID, tier int) (*common.SubscriptionTiers, error) {
	collection := s.database.Collection("subscription-tiers")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Filter by creatorID and tier
	filter := bson.M{
		"creator_id": creatorID,
		"tier":       tier,
	}

	// Execute the query
	var subscriptionTier common.SubscriptionTiers
	err := collection.FindOne(ctx, filter).Decode(&subscriptionTier)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("subscription tier with creatorID %s and tier %d not found", creatorID.Hex(), tier)
		}
		return nil, fmt.Errorf("error fetching subscription tier: %v", err)
	}

	return &subscriptionTier, nil
}

func (s *store) CreateSubscriptionFromTier(userID, creatorID primitive.ObjectID, tier common.SubscriptionTiers, productID string) (primitive.ObjectID, error) {
	collection := s.database.Collection("user-subscriptions")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a new Subscription object based on the SubscriptionTiers info
	subscription := common.Subscription{
		UserID:    userID,
		CreatorID: creatorID,
		TierID:    tier.ID, // ID from the SubscriptionTiers
		Tier:      tier.Tier,
		Pending:   true,      // Set to true if waiting for confirmation
		ProductID: productID, // Set ProductID according to the subscription system (for Apple/Google)
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Active:    false,       // Set to false initially, becomes true once confirmed
		EndDate:   time.Time{}, // You can set a default end date or update it after confirmation
	}

	// Insert the new subscription into the database
	result, err := collection.InsertOne(ctx, subscription)
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("error inserting new subscription: %v", err)
	}

	// Extract the inserted ID, assuming it's an ObjectID
	insertedID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return primitive.NilObjectID, fmt.Errorf("inserted document ID is not an ObjectID")
	}

	return insertedID, nil
}

func (s *store) ActivateSubscription(userID primitive.ObjectID, productID string) error {
	collection := s.database.Collection("user-subscriptions")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Filter by userID, pending=true, and productID
	filter := bson.M{
		"user_id":   userID,
		"pending":   true,
		"productID": productID,
	}

	// Find the subscription with pending=true and matching productID
	var subscription common.Subscription
	err := collection.FindOne(ctx, filter).Decode(&subscription)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("no pending subscription found for user %s with productID %s", userID.Hex(), productID)
		}
		return fmt.Errorf("error finding pending subscription: %v", err)
	}

	// Update the subscription: set pending=false, active=true, and update the endDate
	update := bson.M{
		"$set": bson.M{
			"pending":   false,
			"active":    true,
			"updatedAt": time.Now(),
			"endDate":   time.Now().AddDate(0, 0, 30), // Set endDate to 30 days from now
		},
	}

	// Update the subscription document in MongoDB
	_, err = collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("error updating subscription: %v", err)
	}

	return nil
}

func (s *store) RenewActivateSubscription(userID primitive.ObjectID, productID string) error {
	collection := s.database.Collection("user-subscriptions")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Filter by userID, pending=true, and productID
	filter := bson.M{
		"user_id":   userID,
		"productID": productID,
	}

	// Find the subscription with pending=true and matching productID
	var subscription common.Subscription
	err := collection.FindOne(ctx, filter).Decode(&subscription)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("no pending subscription found for user %s with productID %s", userID.Hex(), productID)
		}
		return fmt.Errorf("error finding pending subscription: %v", err)
	}

	// Update the subscription: set pending=false, active=true, and update the endDate
	update := bson.M{
		"$set": bson.M{
			"active":    true,
			"updatedAt": time.Now(),
			"endDate":   time.Now().AddDate(0, 0, 30), // Set endDate to 30 days from now
		},
	}

	// Update the subscription document in MongoDB
	_, err = collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("error updating subscription: %v", err)
	}

	return nil
}

func (s *store) CancelActivateSubscription(userID primitive.ObjectID, productID string) error {
	collection := s.database.Collection("user-subscriptions")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Filter by userID, pending=true, and productID
	filter := bson.M{
		"user_id":   userID,
		"productID": productID,
	}

	// Find the subscription with pending=true and matching productID
	var subscription common.Subscription
	err := collection.FindOne(ctx, filter).Decode(&subscription)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("no pending subscription found for user %s with productID %s", userID.Hex(), productID)
		}
		return fmt.Errorf("error finding pending subscription: %v", err)
	}

	// Update the subscription: set valid=false, tier=0, pending=true, and productID=""
	update := bson.M{
		"$set": bson.M{
			"pending":   true,  // Set pending back to true
			"active":    false, // Deactivate the subscription
			"valid":     false, // Mark as invalid
			"tier":      0,     // Reset the tier
			"updatedAt": time.Now(),
			"endDate":   time.Time{}, // Reset endDate
		},
	}

	// Update the subscription document in MongoDB
	_, err = collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("error updating subscription: %v", err)
	}

	return nil
}

func (s *store) CreateSubPurchase(purchase *SubPurchase) error {
	query := `
        INSERT INTO sub_purchases (buyer_id, creator_id, created_at, rc_transaction_id, rc_product_id, platform, tier, valid)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `

	// Set a context timeout for the query
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeoutDuration)
	defer cancel()

	// Execute the query
	_, err := s.db.ExecContext(
		ctx,
		query,
		purchase.BuyerID,
		purchase.CreatorID,
		purchase.CreatedAt, // You can set this to time.Now() if not provided in the struct
		purchase.RcTransactionID,
		purchase.RcProductID,
		purchase.Platform,
		purchase.Tier,
		purchase.Valid, // Insert the 'valid' status from the struct
	)

	if err != nil {
		return fmt.Errorf("error creating subscription purchase: %v", err)
	}

	return nil
}

func (s *store) GetSubscription(userID primitive.ObjectID, productID string) (*common.Subscription, error) {
	collection := s.database.Collection("user-subscriptions")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Filter by userID, creatorID, and productID
	filter := bson.M{
		"user_id":   userID,
		"productID": productID,
	}

	// Define a variable to store the subscription result
	var subscription common.Subscription

	// Query MongoDB to find the subscription
	err := collection.FindOne(ctx, filter).Decode(&subscription)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("no subscription found for user %s, and productID %s", userID.Hex(), productID)
		}
		return nil, fmt.Errorf("error finding subscription: %v", err)
	}

	// Return the subscription
	return &subscription, nil
}
