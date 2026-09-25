package main

import (
	"context"
	"log"

	common "github.com/Eskiwi-Organization/infra/commons"
	pb "github.com/Eskiwi-Organization/infra/commons/api"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc"
)

type grpcHandler struct {
	pb.UnimplementedNotificationServiceServer

	service    NotificationService
	mailClient *MailClient
	store      NotificationStore
}

func NewGRPCHandler(grpcServer *grpc.Server, service NotificationService, mailClient *MailClient, store NotificationStore) {
	handler := &grpcHandler{
		service: service, mailClient: mailClient, store: store,
	}
	pb.RegisterNotificationServiceServer(grpcServer, handler)

}

func (h *grpcHandler) SendUserCode(ctx context.Context, p *pb.SendUserCodeRequest) (*pb.SendUserCodeResponse, error) {
	log.Printf("New SendUserCode received! Notification to: %v", p.Email)
	code, _ := common.GenerateRandomCode()
	err := h.mailClient.SendMail(p.Email, "Eskiwi Code verification", code)
	if err != nil {
		o := &pb.SendUserCodeResponse{Status: "error sending mail process: " + err.Error()}
		return o, err
	}

	err = h.store.UpdateUserCode(p.Email, code)
	if err != nil {
		o := &pb.SendUserCodeResponse{Status: "error updating code process: " + err.Error()}
		return o, err
	}

	o := &pb.SendUserCodeResponse{Status: "successfull"}
	return o, nil
}

func (h *grpcHandler) GetLatestNotifications(ctx context.Context, p *pb.GetLatestNotificationsRequest) (*pb.GetLatestNotificationsResponse, error) {
	log.Printf("New GetLatestNotifications received!")

	objUserID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		return nil, err
	}

	list, err := h.store.GetLatestsNotifications(int(p.Page), 10, objUserID)
	if err != nil {
		o := &pb.GetLatestNotificationsResponse{}
		return o, err
	}

	var response []*pb.Notification

	for _, notif := range list {

		o := ConvertToGrpcNotification(notif)
		response = append(response, o)
	}

	o := &pb.GetLatestNotificationsResponse{
		Notifications: response,
	}

	err = h.store.UpdateNotificationCounter(objUserID)
	if err != nil {
		o := &pb.GetLatestNotificationsResponse{}
		return o, err
	}

	return o, nil
}
