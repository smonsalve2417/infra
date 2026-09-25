package main

import (
	"crypto/tls"
	"fmt"
	"strconv"

	"gopkg.in/gomail.v2"
)

type MailClient struct {
	dialer *gomail.Dialer
}

func NewMailClient() *MailClient {
	Port, err := strconv.Atoi(sesPort)
	if err != nil {
		fmt.Println("Error converting string to int:", err)
		return nil
	}
	dialer := gomail.NewDialer(
		"email-smtp.eu-west-3.amazonaws.com", // SMTP host
		Port,                                 // SMTP port
		sesUser,                              // SMTP user
		sesPassword,                          // SMTP password
	)
	dialer.SSL = true
	dialer.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	return &MailClient{dialer: dialer}
}

func (mc *MailClient) SendMail(to, subject, body string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", "no-reply-eskiwi@eskiwi.com")
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", body)

	if err := mc.dialer.DialAndSend(m); err != nil {
		return err
	}

	return nil
}
