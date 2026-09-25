package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"

	common "github.com/Eskiwi-Organization/infra/commons"
	broker "github.com/Eskiwi-Organization/infra/commons/broker"
	db "github.com/Eskiwi-Organization/infra/commons/db"
	"github.com/go-sql-driver/mysql"
	"google.golang.org/grpc"
)

var (
	httpAddr          = common.GetEnv("HTTP_ADDR", ":5000")
	grpcAddr          = common.GetEnv("GRPC_ADDR", "localhost:5010")
	headerAuth        = common.GetEnv("HEADER_AUTH", "d41d8cd98f00b204e9800998ecf8427e")
	mongoAddr         = common.GetEnv("MONGO_ADDR", "mongodb://root:example@mongodb:27017/")
	mongoDatabaseName = common.GetEnv("MONGO_DATABASE_NAME", "Eskiwi")
	amqpUser          = common.GetEnv("RABBITMQ_USER", "guest")
	amqpPass          = common.GetEnv("RABBITMQ_PASS", "guest")
	amqpHost          = common.GetEnv("RABBITMQ_HOST", "localhost")
	amqpPort          = common.GetEnv("RABBITMQ_PORT", "5672")
)

func main() {
	fmt.Println("HTTP_ADDR:", httpAddr)
	fmt.Println("MONGO_ADDR:", mongoAddr)
	fmt.Println("GRPC_ADDR:", grpcAddr)
	fmt.Println("MONGO_DATABASE_NAME:", mongoDatabaseName)
	fmt.Println("RABBITMQ_USER:", amqpUser)
	fmt.Println("RABBITMQ_PASS:", amqpPass)
	fmt.Println("RABBITMQ_HOST:", amqpHost)
	fmt.Println("RABBITMQ_PORT:", amqpPort)

	mysqlUser := "user"
	mysqlPassword := "userpassword"
	mysqlHost := "mysql"
	mysqlPort := "3306"
	mysqlDatabase := "testdb"

	cfg := mysql.Config{
		User:                 mysqlUser,
		Passwd:               mysqlPassword,
		Addr:                 mysqlHost + ":" + mysqlPort,
		DBName:               mysqlDatabase,
		Net:                  "tcp",
		AllowNativePasswords: true,
		ParseTime:            true,
	}

	mysql, err := db.NewMySQLStorage(cfg)
	if err != nil {
		log.Fatal("Error connecting to MYSQL: ", err)
	}

	mongoCLient, err := db.NewMongoDBStorage(mongoAddr, mongoDatabaseName)
	if err != nil {
		log.Fatal("Error connecting to MongoDB: ", err)
	}

	grpcServer := grpc.NewServer()
	l, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	//rabbitmq
	ch, close := broker.Connect(amqpUser, amqpPass, amqpHost, amqpPort)
	defer func() {
		close()
		ch.Close()
	}()

	store := NewStore(mongoCLient.GetDatabase().Client(), mysql)
	svc := NewService(store)
	NewGRPCHandler(grpcServer, svc, store)
	amqpConsumer := NewConsumer(svc)

	mux := http.NewServeMux()
	handler := NewHandler(mongoCLient.GetDatabase().Client(), store)
	handler.registerRoutes(mux)

	go amqpConsumer.ListenRegisterComment(ch)
	go amqpConsumer.ListenRegisterChat(ch)
	go amqpConsumer.ListenAcceptChat(ch)
	go amqpConsumer.ListenDenyChat(ch)
	go amqpConsumer.ListenAcceptMessage(ch)

	go func() {
		log.Println("GRPC Server Started at", grpcAddr)
		if err := grpcServer.Serve(l); err != nil {
			log.Fatal(err.Error())
		}
	}()

	svc.GetAvailableSubscriptionGroup(context.Background())
	svc.CreateSubscribeToCreator(context.Background())

	log.Printf("Starting HTTP server at %s", httpAddr)

	if err := http.ListenAndServe(httpAddr, mux); err != nil {
		log.Fatal("Failed to start http server", err)
	}
}
