package main

import (
	"fmt"
	"log"
	"net/http"

	common "github.com/Eskiwi-Organization/infra/commons"
	pb "github.com/Eskiwi-Organization/infra/commons/api"
	db "github.com/Eskiwi-Organization/infra/commons/db"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	httpAddr            = common.GetEnv("HTTP_ADDR", ":8080")
	mongoAddr           = common.GetEnv("MONGO_ADDR", "mongodb://root:example@mongodb:27017/")
	mongoDatabaseName   = common.GetEnv("MONGO_DATABASE_NAME", "Eskiwi")
	postServiceAddr     = common.GetEnv("POST_SERVICE_ADDR", "localhost:3000")
	userServiceAddr     = common.GetEnv("USER_SERVICE_ADDR", "localhost:4000")
	paymentsServiceAddr = common.GetEnv("PAYMENTS_SERVICE_ADDR", "localhost:5010")
	webServiceAddr      = common.GetEnv("WEB_SERVICE_ADDR", "localhost:8080")
	notifServiceAddr    = common.GetEnv("NOTIFICATION_SERVICE_ADDR", "localhost:2000")
)

func main() {

	fmt.Println("HTTP_ADDR:", httpAddr)
	fmt.Println("POST_SERVICE_ADDR:", postServiceAddr)
	fmt.Println("USER_SERVICE_ADDR:", userServiceAddr)
	fmt.Println("NOTIFICATION_SERVICE_ADDR:", notifServiceAddr)

	mongoCLient, err := db.NewMongoDBStorage(mongoAddr, mongoDatabaseName)
	if err != nil {
		log.Fatal("Error connecting to MongoDB: ", err)
	}
	//notifications

	//posts
	postConn, err := grpc.Dial(postServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to dial server: %v", err)
	}
	defer postConn.Close()
	log.Println("Dialing posts service at:", postServiceAddr)
	postsClient := pb.NewPostServiceClient(postConn)

	//user
	userConn, err := grpc.Dial(userServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to dial server: %v", err)
	}
	defer userConn.Close()
	log.Println("Dialing user service at:", userServiceAddr)
	userClient := pb.NewUserServiceClient(userConn)
	//
	webConn, err := grpc.Dial(webServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to dial server: %v", err)
	}
	defer userConn.Close()
	log.Println("Dialing user service at:", webServiceAddr)
	webClient := pb.NewWebServiceClient(webConn)
	//
	notifConn, err := grpc.Dial(notifServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to dial server: %v", err)
	}
	defer userConn.Close()
	log.Println("Dialing user service at:", notifServiceAddr)
	notifClient := pb.NewNotificationServiceClient(notifConn)
	//
	paymentsConn, err := grpc.Dial(paymentsServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to dial server: %v", err)
	}
	defer userConn.Close()
	log.Println("Dialing user service at:", paymentsServiceAddr)
	paymentsClient := pb.NewPaymentsServiceClient(paymentsConn)

	//
	mux := http.NewServeMux()
	handler := NewHandler(webClient, postsClient, userClient, notifClient, paymentsClient, mongoCLient.GetDatabase().Client())
	handler.registerRoutes(mux)

	log.Printf("Starting HTTP server at %s", httpAddr)

	if err := http.ListenAndServe(httpAddr, mux); err != nil {
		log.Fatal("Failed to start http server", err)
	}

}
