package common

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var Validate = validator.New()

// formatValidationErrors formats a slice of validation errors into a single string
func FormatValidationErrors(validationErrors validator.ValidationErrors) string {
	if len(validationErrors) == 0 {
		return ""
	}
	// Convert the slice of validation errors to a single error message
	errorMessages := make([]string, len(validationErrors))
	for i, err := range validationErrors {
		errorMessages[i] = fmt.Sprintf("Field '%s' failed validation with error: %s", err.Field(), err.Tag())
	}
	return strings.Join(errorMessages, ", ")
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if msg, ok := v.(proto.Message); ok {
		// Use protojson to marshal protobuf messages with EmitUnpopulated option
		marshaller := protojson.MarshalOptions{
			EmitUnpopulated: true,
		}
		jsonBytes, err := marshaller.Marshal(msg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(jsonBytes)
	} else {
		// Use standard json encoding for non-protobuf messages
		json.NewEncoder(w).Encode(v)
	}
}

func ParseJSON(r *http.Request, payload any) error {
	if r.Body == nil {
		return fmt.Errorf("missing request body")
	}
	return json.NewDecoder(r.Body).Decode(payload)
}

func WriteError(w http.ResponseWriter, status int, err string) {
	WriteJSON(w, status, map[string]string{"error": err})
}

func GenerateRandomString(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	for i, b := range bytes {
		result[i] = charset[b%byte(len(charset))]
	}
	return string(result), nil
}

// GenerateRandomImageName generates a random image name with the current timestamp
func GenerateRandomImageName(extension string) (string, error) {
	randomString, err := GenerateRandomString(15)
	if err != nil {
		return "", err
	}
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s_%d.%s", randomString, timestamp, extension), nil
}

func GenerateRandomCode() (string, error) {
	const digits = "0123456789"
	code := make([]byte, 6)
	for i := range code {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random number: %w", err)
		}
		code[i] = digits[num.Int64()]
	}
	return string(code), nil
}
