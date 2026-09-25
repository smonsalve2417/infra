package main

import (
	"context"
	"encoding/json"
	"log"

	common "github.com/Eskiwi-Organization/infra/commons"
	pb "github.com/Eskiwi-Organization/infra/commons/api"
	broker "github.com/Eskiwi-Organization/infra/commons/broker"
	amqp "github.com/rabbitmq/amqp091-go"
)

type consumer struct {
	service NotificationService
}

func NewConsumer(service NotificationService) *consumer {
	return &consumer{service}
}

func (c *consumer) ListenSendCode(ch *amqp.Channel) {
	q, err := ch.QueueDeclare(broker.SendCodemailCreatedEvent, true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	forever := make(chan struct{})

	go func() {
		for msg := range msgs {
			log.Printf("Received message: %v", msg)

			o := &pb.SendUserCodeRequest{}
			if err := json.Unmarshal(msg.Body, o); err != nil {
				log.Printf("failed to unmarshal mail: %v", err)
				continue
			}
			err = c.service.SendUserCode(context.Background(), o)
			if err != nil {
				log.Printf("failed sending mail: %v", err)
				continue
			}

		}
	}()

	<-forever
}

func (c *consumer) ListenCreatePosts(ch *amqp.Channel) {
	q, err := ch.QueueDeclare(broker.SendPostsNotificationCreatedEvent, true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	forever := make(chan struct{})

	go func() {
		for msg := range msgs {
			log.Printf("Received Post created: %v", msg)

			//sacar info recibida
			var payload common.Post
			if err := json.Unmarshal(msg.Body, &payload); err != nil {
				log.Printf("failed to unmarshal mail: %v", err)
				continue
			}

			//proceso a crear, en este caso la noti del post
			err = c.service.CreatePostNotification(context.Background(), payload)
			if err != nil {
				log.Printf("failed creating post notification: %v", err)
				continue
			}

		}
	}()

	<-forever
}

func (c *consumer) ListenLikePosts(ch *amqp.Channel) {
	q, err := ch.QueueDeclare(broker.SendLikeNotificationCreatedEvent, true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	forever := make(chan struct{})

	go func() {
		for msg := range msgs {
			log.Printf("Received Like created: %v", msg)

			//sacar info recibida
			var payload common.LikeNotificationPayload
			if err := json.Unmarshal(msg.Body, &payload); err != nil {
				log.Printf("failed to unmarshal mail: %v", err)
				continue
			}

			//proceso a crear, en este caso la noti del post
			err = c.service.CreatePostLikeNotification(context.Background(), payload)
			if err != nil {
				log.Printf("failed creating like notification: %v", err)
				continue
			}

		}
	}()

	<-forever
}

func (c *consumer) ListenCommentPosts(ch *amqp.Channel) {
	q, err := ch.QueueDeclare(broker.SendCommentNotificationCreatedEvent, true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	forever := make(chan struct{})

	go func() {
		for msg := range msgs {
			log.Printf("Received Comment created: %v", msg)

			//sacar info recibida
			var payload common.LikeNotificationPayload
			if err := json.Unmarshal(msg.Body, &payload); err != nil {
				log.Printf("failed to unmarshal mail: %v", err)
				continue
			}

			//proceso a crear, en este caso la noti del post
			err = c.service.CreatePostCommentNotification(context.Background(), payload)
			if err != nil {
				log.Printf("failed creating comment notification: %v", err)
				continue
			}

		}
	}()

	<-forever
}

func (c *consumer) ListenFollowUser(ch *amqp.Channel) {
	q, err := ch.QueueDeclare(broker.SendFollowNotificationCreatedEvent, true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	forever := make(chan struct{})

	go func() {
		for msg := range msgs {
			log.Printf("Received Follow created: %v", msg)

			//sacar info recibida
			var payload common.FollowNotificationPayload
			if err := json.Unmarshal(msg.Body, &payload); err != nil {
				log.Printf("failed to unmarshal mail: %v", err)
				continue
			}

			//proceso a crear, en este caso la noti del post
			err = c.service.CreateUserFollowNotification(context.Background(), payload)
			if err != nil {
				log.Printf("failed creating user follow notification: %v", err)
				continue
			}

		}
	}()

	<-forever
}

func (c *consumer) ListenChatmsg(ch *amqp.Channel) {
	q, err := ch.QueueDeclare(broker.SendChatNotificationCreatedEvent, true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	forever := make(chan struct{})

	go func() {
		for msg := range msgs {
			log.Printf("Received Message sent: %v", msg)

			//sacar info recibida
			var payload common.ChatNotificationPayload
			if err := json.Unmarshal(msg.Body, &payload); err != nil {
				log.Printf("failed to unmarshal mail: %v", err)
				continue
			}

			//proceso a crear, en este caso la noti del post
			err = c.service.CreateChatmsgNotification(context.Background(), payload)
			if err != nil {
				log.Printf("failed creating user follow notification: %v", err)
				continue
			}

		}
	}()

	<-forever
}
