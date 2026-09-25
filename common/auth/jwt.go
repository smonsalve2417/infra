package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	common "github.com/Eskiwi-Organization/infra/commons"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type contextKey string

const (
	UserKey           contextKey = "_id"
	RateLimitMaxCount            = 100         // Max number of requests
	RateLimitWindow              = time.Minute // Time window for rate limiting
)

var userRateLimitStore = make(map[string][]time.Time)
var mu sync.Mutex

func CreateJWT(secret []byte, userID string) (string, error) {

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"_id":       userID,
		"expiredAt": time.Now().Add(3600 * 24 * 365).Unix(),
	})
	tokenString, err := token.SignedString(secret)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func WithJWTAuth(handlerFunc http.HandlerFunc, database *mongo.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenString := GetTokenFromRequest(r)

		token, err := ValidateJWT(tokenString)
		if err != nil {
			log.Printf("failed to validate token %v", err)
			PermissionDenied(w)
			return
		}

		if !token.Valid {
			log.Println("invalid token")
			PermissionDenied(w)
			return
		}

		claims := token.Claims.(jwt.MapClaims)
		userID := claims["_id"].(string)

		user, err := GetUserByID(userID, database)
		if err != nil {
			log.Printf("failed to get user by id: %v", err)
			PermissionDenied(w)
			return
		}

		if user.Banned {
			BannedDenied(w)
			return
		}

		if !ApplyRateLimit(userID) {
			log.Printf("Rate limit exceeded: %v", userID)
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, UserKey, user.ID)
		r = r.WithContext(ctx)

		handlerFunc(w, r)

	}
}

func ApplyRateLimit(userID string) bool {
	mu.Lock()
	defer mu.Unlock()

	now := time.Now()

	// Initialize rate limit data for the user if not present
	if _, exists := userRateLimitStore[userID]; !exists {
		userRateLimitStore[userID] = []time.Time{now}
		return true
	}

	// Filter out old requests that are outside the rate limit window
	timestamps := userRateLimitStore[userID]
	withinWindow := []time.Time{}
	for _, ts := range timestamps {
		if now.Sub(ts) <= RateLimitWindow {
			withinWindow = append(withinWindow, ts)
		}
	}

	// Update rate limit data with the current request
	if len(withinWindow) >= RateLimitMaxCount {
		// Rate limit exceeded
		return false
	}

	// Store the current timestamp and save back the updated list
	withinWindow = append(withinWindow, now)
	userRateLimitStore[userID] = withinWindow
	return true
}

func PermissionDenied(w http.ResponseWriter) {
	common.WriteError(w, http.StatusForbidden, "permission denied")
}

func BannedDenied(w http.ResponseWriter) {
	common.WriteError(w, http.StatusForbidden, "banned user")
}

func GetTokenFromRequest(r *http.Request) string {
	tokenAuth := r.Header.Get("Authorization")
	tokenQuery := r.URL.Query().Get("token")

	if tokenAuth != "" {
		if len(tokenAuth) > 7 && tokenAuth[:7] == "Bearer " {
			return tokenAuth[7:]
		}
		return tokenAuth
	}

	if tokenQuery != "" {
		return tokenQuery
	}

	return ""
}

func ValidateJWT(tokenString string) (*jwt.Token, error) {
	////////////////
	log.Printf("ValidateJWT: Validating token: %s", tokenString)
	////////////////
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte("tragatela"), nil
	})
}

func GetUserIDFromContext(ctx context.Context) (primitive.ObjectID, error) {
	userID, ok := ctx.Value(UserKey).(primitive.ObjectID)
	if !ok {
		return userID, fmt.Errorf("user ID not found in context or not a string: " + string(UserKey))
	}
	return userID, nil
}

func GetUserByID(id string, database *mongo.Database) (*common.User, error) {
	collection := database.Collection("users")
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
