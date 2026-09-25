package main

import (
	"context"
	"encoding/json"
	"log"

	common "github.com/Eskiwi-Organization/infra/commons"
	broker "github.com/Eskiwi-Organization/infra/commons/broker"
	amqp "github.com/rabbitmq/amqp091-go"
)

type consumer struct {
	service PaymentsService
}

func NewConsumer(service PaymentsService) *consumer {
	return &consumer{service}
}

func (c *consumer) ListenRegisterComment(ch *amqp.Channel) {
	// Declarar la cola para recibir comentarios
	q, err := ch.QueueDeclare(broker.SendDonationCommentCreatedEvent, true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Consumir mensajes de la cola
	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	forever := make(chan struct{})

	go func() {
		for msg := range msgs {
			log.Printf("Received message: %v", msg)

			// Deserializar el comentario recibido
			var comment common.Comment
			if err := json.Unmarshal(msg.Body, &comment); err != nil {
				log.Printf("failed to unmarshal mail: %v", err)
				continue
			}

			// Intentar registrar el comentario en el servicio
			err = c.service.RegisterComment(context.Background(), comment)

			// Preparar la respuesta de éxito/fallo
			var response struct {
				Success bool `json:"success"`
			}

			if err != nil {
				log.Printf("failed registering comment: %v", err)
				response.Success = false
			} else {
				response.Success = true
			}

			// Enviar la respuesta al canal de respuesta (ReplyTo) con la misma CorrelationId
			responseBytes, err := json.Marshal(response)
			if err != nil {
				log.Printf("failed to marshal response: %v", err)
				continue
			}

			err = ch.Publish(
				"",          // exchange
				msg.ReplyTo, // routing key (callback queue)
				false,       // mandatory
				false,       // immediate
				amqp.Publishing{
					ContentType:   "application/json",
					Body:          responseBytes,
					CorrelationId: msg.CorrelationId, // Asegurar que el CorrelationId sea el mismo
				},
			)
			if err != nil {
				log.Printf("failed to publish response: %v", err)
				continue
			}
		}
	}()

	<-forever
}

func (c *consumer) ListenRegisterChat(ch *amqp.Channel) {
	// Declarar la cola para recibir comentarios
	q, err := ch.QueueDeclare(broker.SendCreateChatCreatedEvent, true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Consumir mensajes de la cola
	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	forever := make(chan struct{})

	go func() {
		for msg := range msgs {
			log.Printf("Received message: %v", msg)

			// Deserializar el comentario recibido
			var ChatCreation common.CreateChatDonationPayload
			if err := json.Unmarshal(msg.Body, &ChatCreation); err != nil {
				log.Printf("failed to unmarshal mail: %v", err)
				continue
			}

			log.Printf("amount of gems pre function: %v", ChatCreation.Gems)

			// Intentar registrar el comentario en el servicio
			err = c.service.RegisterChat(context.Background(), ChatCreation)

			// Preparar la respuesta de éxito/fallo
			var response struct {
				Success bool `json:"success"`
			}

			if err != nil {
				log.Printf("failed registering chat: %v", err)
				response.Success = false
			} else {
				response.Success = true
			}

			// Enviar la respuesta al canal de respuesta (ReplyTo) con la misma CorrelationId
			responseBytes, err := json.Marshal(response)
			if err != nil {
				log.Printf("failed to marshal response: %v", err)
				continue
			}

			err = ch.Publish(
				"",          // exchange
				msg.ReplyTo, // routing key (callback queue)
				false,       // mandatory
				false,       // immediate
				amqp.Publishing{
					ContentType:   "application/json",
					Body:          responseBytes,
					CorrelationId: msg.CorrelationId, // Asegurar que el CorrelationId sea el mismo
				},
			)
			if err != nil {
				log.Printf("failed to publish response: %v", err)
				continue
			}
		}
	}()

	<-forever
}

func (c *consumer) ListenAcceptChat(ch *amqp.Channel) {
	// Declarar la cola para recibir comentarios
	q, err := ch.QueueDeclare(broker.SendAcceptChatCreatedEvent, true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Consumir mensajes de la cola
	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	forever := make(chan struct{})

	go func() {
		for msg := range msgs {
			log.Printf("Received message: %v", msg)

			// Deserializar el comentario recibido
			var ChatAccept common.AcceptChatPayload
			if err := json.Unmarshal(msg.Body, &ChatAccept); err != nil {
				log.Printf("failed to unmarshal mail: %v", err)
				continue
			}

			// Intentar registrar el comentario en el servicio
			err = c.service.AcceptChat(context.Background(), ChatAccept)

			// Preparar la respuesta de éxito/fallo
			var response struct {
				Success bool `json:"success"`
			}

			if err != nil {
				log.Printf("failed accepting chat: %v", err)
				response.Success = false
			} else {
				response.Success = true
			}

			// Enviar la respuesta al canal de respuesta (ReplyTo) con la misma CorrelationId
			responseBytes, err := json.Marshal(response)
			if err != nil {
				log.Printf("failed to marshal response: %v", err)
				continue
			}

			err = ch.Publish(
				"",          // exchange
				msg.ReplyTo, // routing key (callback queue)
				false,       // mandatory
				false,       // immediate
				amqp.Publishing{
					ContentType:   "application/json",
					Body:          responseBytes,
					CorrelationId: msg.CorrelationId, // Asegurar que el CorrelationId sea el mismo
				},
			)
			if err != nil {
				log.Printf("failed to publish response: %v", err)
				continue
			}
		}
	}()

	<-forever
}

func (c *consumer) ListenDenyChat(ch *amqp.Channel) {
	// Declarar la cola para recibir comentarios
	q, err := ch.QueueDeclare(broker.SendDenyChatCreatedEvent, true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Consumir mensajes de la cola
	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	forever := make(chan struct{})

	go func() {
		for msg := range msgs {
			log.Printf("Received message: %v", msg)

			// Deserializar el comentario recibido
			var ChatAccept common.AcceptChatPayload
			if err := json.Unmarshal(msg.Body, &ChatAccept); err != nil {
				log.Printf("failed to unmarshal mail: %v", err)
				continue
			}

			// Intentar registrar el comentario en el servicio
			err = c.service.DenyChat(context.Background(), ChatAccept)

			// Preparar la respuesta de éxito/fallo
			var response struct {
				Success bool `json:"success"`
			}

			if err != nil {
				log.Printf("failed accepting chat: %v", err)
				response.Success = false
			} else {
				response.Success = true
			}

			// Enviar la respuesta al canal de respuesta (ReplyTo) con la misma CorrelationId
			responseBytes, err := json.Marshal(response)
			if err != nil {
				log.Printf("failed to marshal response: %v", err)
				continue
			}

			err = ch.Publish(
				"",          // exchange
				msg.ReplyTo, // routing key (callback queue)
				false,       // mandatory
				false,       // immediate
				amqp.Publishing{
					ContentType:   "application/json",
					Body:          responseBytes,
					CorrelationId: msg.CorrelationId, // Asegurar que el CorrelationId sea el mismo
				},
			)
			if err != nil {
				log.Printf("failed to publish response: %v", err)
				continue
			}
		}
	}()

	<-forever
}

func (c *consumer) ListenAcceptMessage(ch *amqp.Channel) {
	// Declarar la cola para recibir comentarios
	q, err := ch.QueueDeclare(broker.SendMessageGemsCreatedEvent, true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Consumir mensajes de la cola
	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	forever := make(chan struct{})

	go func() {
		for msg := range msgs {
			log.Printf("Received message: %v", msg)

			// Deserializar el comentario recibido
			var chatMessage common.GemsOnChatPayload
			if err := json.Unmarshal(msg.Body, &chatMessage); err != nil {
				log.Printf("failed to unmarshal mail: %v", err)
				continue
			}

			// Intentar registrar el comentario en el servicio
			err = c.service.RegisterChatMessage(context.Background(), chatMessage)

			// Preparar la respuesta de éxito/fallo
			var response struct {
				Success bool `json:"success"`
			}

			if err != nil {
				log.Printf("failed registering comment: %v", err)
				response.Success = false
			} else {
				response.Success = true
			}

			// Enviar la respuesta al canal de respuesta (ReplyTo) con la misma CorrelationId
			responseBytes, err := json.Marshal(response)
			if err != nil {
				log.Printf("failed to marshal response: %v", err)
				continue
			}

			err = ch.Publish(
				"",          // exchange
				msg.ReplyTo, // routing key (callback queue)
				false,       // mandatory
				false,       // immediate
				amqp.Publishing{
					ContentType:   "application/json",
					Body:          responseBytes,
					CorrelationId: msg.CorrelationId, // Asegurar que el CorrelationId sea el mismo
				},
			)
			if err != nil {
				log.Printf("failed to publish response: %v", err)
				continue
			}
		}
	}()

	<-forever
}
