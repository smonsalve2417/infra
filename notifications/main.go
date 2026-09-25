package main

import (
	"context"
	"fmt"
	"log"
	"net"

	common "github.com/Eskiwi-Organization/infra/commons"
	broker "github.com/Eskiwi-Organization/infra/commons/broker"
	db "github.com/Eskiwi-Organization/infra/commons/db"
	"google.golang.org/grpc"
)

var (
	grpcAddr          = common.GetEnv("GRPC_ADDR", "localhost:2000")
	sesUser           = common.GetEnv("SESUSER", "")
	sesPassword       = common.GetEnv("SESPASSWORD", "")
	sesPort           = common.GetEnv("SESPORT", "465")
	mongoAddr         = common.GetEnv("MONGO_ADDR", "mongodb://root:example@mongodb:27017/")
	mongoDatabaseName = common.GetEnv("MONGO_DATABASE_NAME", "Eskiwi")
	amqpUser          = common.GetEnv("RABBITMQ_USER", "guest")
	amqpPass          = common.GetEnv("RABBITMQ_PASS", "guest")
	amqpHost          = common.GetEnv("RABBITMQ_HOST", "localhost")
	amqpPort          = common.GetEnv("RABBITMQ_PORT", "5672")
)

func main() {

	fmt.Println("GRPC_ADDR:", grpcAddr)
	fmt.Println("SESUSER:", sesUser)
	fmt.Println("SESPASSWORD:", sesPassword)
	fmt.Println("SESPORT:", sesPort)
	fmt.Println("MONGO_ADDR:", mongoAddr)
	fmt.Println("MONGO_DATABASE_NAME:", mongoDatabaseName)
	fmt.Println("RABBITMQ_USER:", amqpUser)
	fmt.Println("RABBITMQ_PASS:", amqpPass)
	fmt.Println("RABBITMQ_HOST:", amqpHost)
	fmt.Println("RABBITMQ_PORT:", amqpPort)

	mongoCLient, err := db.NewMongoDBStorage(mongoAddr, mongoDatabaseName)
	if err != nil {
		log.Fatal("Error connecting to MongoDB: ", err)
	}

	//rabbitmq
	ch, close := broker.Connect(amqpUser, amqpPass, amqpHost, amqpPort)
	defer func() {
		close()
		ch.Close()
	}()

	grpcServer := grpc.NewServer()
	l, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	mailClient := NewMailClient()

	store := NewStore(mongoCLient.GetDatabase().Client())
	svc := NewService(mailClient, store)
	amqpConsumer := NewConsumer(svc)

	svc.GetLatestNotifications(context.Background())

	go amqpConsumer.ListenSendCode(ch)
	go amqpConsumer.ListenCreatePosts(ch)
	go amqpConsumer.ListenLikePosts(ch)
	go amqpConsumer.ListenCommentPosts(ch)
	go amqpConsumer.ListenFollowUser(ch)
	go amqpConsumer.ListenChatmsg(ch)

	NewGRPCHandler(grpcServer, svc, mailClient, store)

	log.Println("GRPC Server Started at", grpcAddr)

	if err := grpcServer.Serve(l); err != nil {
		log.Fatal(err.Error())
	}
}
