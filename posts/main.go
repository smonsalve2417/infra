package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"

	common "github.com/Eskiwi-Organization/infra/commons"
	aws_s3 "github.com/Eskiwi-Organization/infra/commons/aws_s3"
	"github.com/Eskiwi-Organization/infra/commons/broker"
	db "github.com/Eskiwi-Organization/infra/commons/db"
	"google.golang.org/grpc"
)

var (
	httpAddr            = common.GetEnv("HTTP_ADDR", ":3010")
	grpcAddr            = common.GetEnv("GRPC_ADDR", "localhost:3000")
	mongoAddr           = common.GetEnv("MONGO_ADDR", "mongodb://root:example@mongodb:27017/")
	mongoDatabaseName   = common.GetEnv("MONGO_DATABASE_NAME", "Eskiwi")
	AWS_Post_Bucket_URL = common.GetEnv("AWS_POST_BUCKET_URL", "posts.eskiwi.com")
	S3Access            = common.GetEnv("S3ACCESS", "")
	S3Secret            = common.GetEnv("S3SECRET", "")
	S3Region            = common.GetEnv("S3REGION", "eu-west-3")
	CloudFlare_Link     = common.GetEnv("CLOUDFLARE_LINK", "da2mmmrwmvcrh.cloudfront.net")
	amqpUser            = common.GetEnv("RABBITMQ_USER", "guest")
	amqpPass            = common.GetEnv("RABBITMQ_PASS", "guest")
	amqpHost            = common.GetEnv("RABBITMQ_HOST", "localhost")
	amqpPort            = common.GetEnv("RABBITMQ_PORT", "5672")
)

func main() {

	fmt.Println("GRPC_ADDR:", grpcAddr)
	fmt.Println("MONGO_ADDR:", mongoAddr)
	fmt.Println("MONGO_DATABASE_NAME:", mongoDatabaseName)
	fmt.Println("AWS_AVATAR_BUCKET_URL:", AWS_Post_Bucket_URL)
	fmt.Println("S3ACCESS:", S3Access)
	fmt.Println("S3SECRET:", S3Secret)
	fmt.Println("S3REGION:", S3Region)
	fmt.Println("RABBITMQ_USER:", amqpUser)
	fmt.Println("RABBITMQ_PASS:", amqpPass)
	fmt.Println("RABBITMQ_HOST:", amqpHost)
	fmt.Println("RABBITMQ_PORT:", amqpPort)

	mongoCLient, err := db.NewMongoDBStorage(mongoAddr, mongoDatabaseName)
	if err != nil {
		log.Fatal("Error connecting to MongoDB: ", err)
	}

	PostS3Client, err := aws_s3.NewS3Client(S3Access, S3Secret, AWS_Post_Bucket_URL, S3Region)
	if err != nil {
		log.Fatal("Error connecting to AWS-s3Client avatar bucket: ", err)
	}

	grpcServer := grpc.NewServer()
	l, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	//rabbitmq
	ch, close := broker.Connect(amqpUser, amqpPass, amqpHost, amqpPort)
	if err != nil {
		log.Fatalf("Failed to connect to broker: %v", err)
	}
	defer func() {
		if err := close(); err != nil {
			log.Printf("Error closing connection: %v", err)
		}
		if err := ch.Close(); err != nil {
			log.Printf("Error closing channel: %v", err)
		}
	}()

	store := NewStore(mongoCLient.GetDatabase().Client(), PostS3Client)
	svc := NewService()
	NewGRPCHandler(grpcServer, svc, store, ch)

	svc.CreatePost(context.Background())
	svc.DeletePost(context.Background())
	svc.LikePost(context.Background())
	svc.UnlikePost(context.Background())
	svc.CreateComment(context.Background())
	svc.DeleteComment(context.Background())
	svc.LikeComment(context.Background())
	svc.UnLikeComment(context.Background())
	svc.GetUserCommentLike(context.Background())
	svc.CreateReply(context.Background())
	svc.DeleteReply(context.Background())
	svc.GetPost(context.Background())
	svc.GetLatestPosts(context.Background())
	svc.GetLatestUserPosts(context.Background())
	svc.GetUserLike(context.Background())
	svc.GetLatestComments(context.Background())
	svc.GetLatestReplies(context.Background())
	svc.GetRecommended(context.Background())
	svc.SharePost(context.Background())

	go func() {
		log.Println("GRPC Server Started at", grpcAddr)
		if err := grpcServer.Serve(l); err != nil {
			log.Fatal(err.Error())
		}
	}()

	fmt.Print("mux")
	mux := http.NewServeMux()
	fmt.Print("handler")
	handler := NewHandler(svc, store, mongoCLient.GetDatabase().Client(), ch)
	handler.registerRoutes(mux)

	fmt.Printf("Starting HTTP server at %s", httpAddr)

	if err := http.ListenAndServe(httpAddr, mux); err != nil {
		log.Fatal("Failed to start http server", err)
	}
}
