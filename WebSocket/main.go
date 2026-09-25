// main.go
package main

import (
	"context"
	"log"
	"net"
	"net/http"

	common "github.com/Eskiwi-Organization/infra/commons"
	"github.com/Eskiwi-Organization/infra/commons/aws_s3"
	"github.com/Eskiwi-Organization/infra/commons/broker"
	"github.com/Eskiwi-Organization/infra/commons/db"
	"google.golang.org/grpc"
)

var (
	mongoAddr            = common.GetEnv("MONGO_ADDR", "mongodb://root:example@localhost:27017/")
	mongoDatabaseName    = common.GetEnv("MONGO_DATABASE_NAME", "Eskiwi")
	grpcAddr             = common.GetEnv("GRPC_ADDR", "localhost:8090")
	AWS_Chats_Bucket_URL = common.GetEnv("AWS_CHATS_BUCKET_URL", "chats.eskiwi.com")
	S3Access             = common.GetEnv("S3ACCESS", "")
	S3Secret             = common.GetEnv("S3SECRET", "")
	S3Region             = common.GetEnv("S3REGION", "eu-west-3")
	CloudFlare_Link      = common.GetEnv("CLOUDFLARE_LINK", "da2mmmrwmvcrh.cloudfront.net")
	amqpUser             = common.GetEnv("RABBITMQ_USER", "guest")
	amqpPass             = common.GetEnv("RABBITMQ_PASS", "guest")
	amqpHost             = common.GetEnv("RABBITMQ_HOST", "localhost")
	amqpPort             = common.GetEnv("RABBITMQ_PORT", "5672")
)

func main() {

	mongoCLient, err := db.NewMongoDBStorage(mongoAddr, mongoDatabaseName)
	if err != nil {
		log.Fatal("Error connecting to MongoDB: ", err)
	}

	WebS3Client, err := aws_s3.NewS3Client(S3Access, S3Secret, AWS_Chats_Bucket_URL, S3Region)
	if err != nil {
		log.Fatal("Error connecting to AWS-s3Client avatar bucket: ", err)
	}

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
	svc := NewService()

	store := NewStore(mongoCLient.GetDatabase().Client(), WebS3Client, ch)

	hub := NewHub()
	mux := http.NewServeMux()
	handler := NewHandler(mongoCLient.GetDatabase().Client(), hub, store, ch)
	handler.registerRoutes(mux)

	NewGRPCHandler(grpcServer, svc, store, hub, ch)

	go hub.Run()

	//grpc

	svc.CreateChat(context.Background())
	svc.CreateRoom(context.Background())
	svc.GetLatestsChats(context.Background())
	svc.GetLatestsRequestChats(context.Background())
	svc.GetChat(context.Background())
	svc.GetLatestsMessages(context.Background())
	svc.AcceptChatReq(context.Background())
	svc.RejectChatReq(context.Background())
	svc.AcceptChatInsideReq(context.Background())
	svc.RejectChatInsideReq(context.Background())

	go func() {
		log.Println("GRPC Server Started at", grpcAddr)
		if err := grpcServer.Serve(l); err != nil {
			log.Fatal(err.Error())
		}
	}()
	//f grpc

	log.Printf("Starting HTTP server at %s", ":8080")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal("Failed to start http server", err)
	}

}
