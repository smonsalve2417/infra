package broker

import (
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

func Connect(user, pass, host, port string) (*amqp.Channel, func() error) {
	address := fmt.Sprintf("amqp://%s:%s@%s:%s", user, pass, host, port)

	conn, err := amqp.Dial(address)
	if err != nil {
		log.Fatal(err)
	}

	ch, err := conn.Channel()
	if err != nil {
		log.Fatal(err)
	}

	//mail code
	err = ch.ExchangeDeclare(SendCodemailCreatedEvent, "direct", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	err = ch.ExchangeDeclare(SendCodemailSentEvent, "direct", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	//Post
	err = ch.ExchangeDeclare(SendPostsNotificationCreatedEvent, "direct", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	err = ch.ExchangeDeclare(SendPostsNotificationSentEvent, "direct", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	//Like
	err = ch.ExchangeDeclare(SendLikeNotificationCreatedEvent, "direct", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	err = ch.ExchangeDeclare(SendLikeNotificationSentEvent, "direct", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	//comment
	err = ch.ExchangeDeclare(SendDonationCommentCreatedEvent, "direct", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	err = ch.ExchangeDeclare(SendDonationCommentSentEvent, "direct", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	//Chats
	err = ch.ExchangeDeclare(SendCreateChatCreatedEvent, "direct", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	err = ch.ExchangeDeclare(SendCreateChatSentEvent, "direct", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	//	accept chat
	err = ch.ExchangeDeclare(SendAcceptChatCreatedEvent, "direct", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	err = ch.ExchangeDeclare(SendAcceptChatSentEvent, "direct", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	err = ch.ExchangeDeclare(SendDenyChatCreatedEvent, "direct", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	err = ch.ExchangeDeclare(SendDenyChatSentEvent, "direct", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	err = ch.ExchangeDeclare(SendMessageGemsCreatedEvent, "direct", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	err = ch.ExchangeDeclare(SendMessageGemsSentEvent, "direct", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	err = ch.ExchangeDeclare(SendAcceptMessageReqCreatedEvent, "direct", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	err = ch.ExchangeDeclare(SendAcceptMessageReqSentEvent, "direct", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	err = ch.ExchangeDeclare(SendRejectMessageReqCreatedEvent, "direct", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	err = ch.ExchangeDeclare(SendRejectMessageReqSentEvent, "direct", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}
	return ch, conn.Close

}
