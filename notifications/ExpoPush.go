package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// Notification payload
type ExpoPushMessage struct {
	To    []string `json:"to"`
	Title string   `json:"title"`
	Body  string   `json:"body"`
	Sound string   `json:"sound,omitempty"`
}

// Send a push notification using Expo's API
func sendPushNotification(pushTokens []string, title, body string) error {
	url := "https://exp.host/--/api/v2/push/send"

	// Create the notification payload with multiple push tokens
	message := ExpoPushMessage{
		To:    pushTokens,
		Title: title,
		Body:  body,
		Sound: "default",
	}

	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("error marshaling notification payload: %v", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("error sending push notification: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
