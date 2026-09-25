package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"

	common "github.com/Eskiwi-Organization/infra/commons"
	pb "github.com/Eskiwi-Organization/infra/commons/api"
	"github.com/Eskiwi-Organization/infra/commons/aws_s3"
	"github.com/Eskiwi-Organization/infra/commons/broker"
	"github.com/Eskiwi-Organization/infra/commons/db"
	"github.com/go-sql-driver/mysql"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	grpcAddr                = common.GetEnv("GRPC_ADDR", "localhost:4000")
	notificationServiceAddr = common.GetEnv("NOTIFICATION_SERVICE_ADDR", "localhost:2000")
	mongoAddr               = common.GetEnv("MONGO_ADDR", "mongodb://root:example@mongodb:27017/")
	mongoDatabaseName       = common.GetEnv("MONGO_DATABASE_NAME", "Eskiwi")
	AWS_Avatar_Bucket_URL   = common.GetEnv("AWS_AVATAR_BUCKET_URL", "avatars.eskiwi.com")
	JWTSecret               = common.GetEnv("JWTSECRET", "tragatela")
	S3Access                = common.GetEnv("S3ACCESS", "")
	S3Secret                = common.GetEnv("S3SECRET", "")
	S3Region                = common.GetEnv("S3REGION", "eu-west-3")
	CloudFlare_Link         = common.GetEnv("CLOUDFLARE_LINK", "d28b1xvie0871u.cloudfront.net")
	amqpUser                = common.GetEnv("RABBITMQ_USER", "guest")
	amqpPass                = common.GetEnv("RABBITMQ_PASS", "guest")
	amqpHost                = common.GetEnv("RABBITMQ_HOST", "localhost")
	amqpPort                = common.GetEnv("RABBITMQ_PORT", "5672")
)

func main() {

	fmt.Println("GRPC_ADDR:", grpcAddr)
	fmt.Println("NOTIFICATION_SERVICE_ADDR:", notificationServiceAddr)
	fmt.Println("MONGO_ADDR:", mongoAddr)
	fmt.Println("MONGO_DATABASE_NAME:", mongoDatabaseName)
	fmt.Println("AWS_AVATAR_BUCKET_URL:", AWS_Avatar_Bucket_URL)
	fmt.Println("JWTSECRET:", JWTSecret)
	fmt.Println("S3ACCESS:", S3Access)
	fmt.Println("S3SECRET:", S3Secret)
	fmt.Println("S3REGION:", S3Region)
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

	AvatarS3Client, err := aws_s3.NewS3Client(S3Access, S3Secret, AWS_Avatar_Bucket_URL, S3Region)
	if err != nil {
		log.Fatal("Error connecting to AWS-s3Client avatar bucket: ", err)
	}

	notificationConn, err := grpc.Dial(notificationServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to dial server: %v", err)
	}
	defer notificationConn.Close()
	log.Println("Dialing notificacions service at:", notificationServiceAddr)
	notificationsClient := pb.NewNotificationServiceClient(notificationConn)

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

	store := NewStore(mongoCLient.GetDatabase().Client(), AvatarS3Client, mysql)
	svc := NewService()
	NewGRPCHandler(grpcServer, notificationsClient, svc, store, ch)

	svc.SendUserCode(context.Background())
	svc.LoginUser(context.Background())
	svc.ValidateUser(context.Background())
	svc.RegisterUser(context.Background())
	svc.GetUser(context.Background())
	svc.GetUserByID(context.Background())
	svc.UpdateAvatar(context.Background())
	svc.UpdateBanner(context.Background())
	svc.FollowUser(context.Background())
	svc.UnFollowUser(context.Background())
	svc.GetUserFollow(context.Background())
	svc.UpdateDescription(context.Background())
	svc.AddExpoToken(context.Background())
	svc.DeleteExpoToken(context.Background())
	svc.UpdateNotificationsSettings(context.Background())
	svc.GetUserNotificationSettings(context.Background())
	svc.SearchCreators(context.Background())
	svc.GetTopCreator(context.Background())
	svc.ChangePassword(context.Background())
	svc.UpdateUsername(context.Background())
	svc.GetChatSettings(context.Background())
	svc.GetTransactions(context.Background())
	svc.CreateSubscriptionTier(context.Background())
	svc.GetSubscriptionTier(context.Background())

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

	fmt.Printf("Starting HTTP server at %s", "4010")

	if err := http.ListenAndServe(":4010", mux); err != nil {
		log.Fatal("Failed to start http server", err)
	}
}
