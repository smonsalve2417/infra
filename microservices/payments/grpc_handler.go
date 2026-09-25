package main

import (
	"context"
	"log"
	"strconv"

	pb "github.com/Eskiwi-Organization/infra/commons/api"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc"
)

type grpcHandler struct {
	pb.UnimplementedPaymentsServiceServer

	service PaymentsService
	store   PaymentsStore
}

func NewGRPCHandler(grpcServer *grpc.Server, service PaymentsService, store PaymentsStore) {
	handler := &grpcHandler{
		service: service,
		store:   store,
	}
	pb.RegisterPaymentsServiceServer(grpcServer, handler)

}

func (h *grpcHandler) GetAvailableSubscriptionGroup(ctx context.Context, p *pb.GetAvailableSubscriptionGroupRequest) (*pb.GetAvailableSubscriptionGroupResponse, error) {
	log.Printf("New GetAvailableSubscriptionGroup received! from: %v", p.UserID)

	user := p.GetUserID()
	objID, err := primitive.ObjectIDFromHex(user)
	if err != nil {
		o := &pb.GetAvailableSubscriptionGroupResponse{}
		return o, err
	}

	productIDs, err := h.store.GetProductIDsByUserID(objID)
	if err != nil {
		o := &pb.GetAvailableSubscriptionGroupResponse{}
		return o, err
	}

	smallestMissing, err := h.store.FindSmallestMissingNumberFromProductIDs(productIDs)
	if err != nil {
		o := &pb.GetAvailableSubscriptionGroupResponse{}
		return o, err
	}

	strSmallestMissing := strconv.Itoa(smallestMissing)

	o := &pb.GetAvailableSubscriptionGroupResponse{Group: strSmallestMissing}
	return o, nil
}

func (h *grpcHandler) CreateSubscribeToCreator(ctx context.Context, p *pb.CreateSubscribeToCreatorRequest) (*pb.CreateSubscribeToCreatorResponse, error) {
	log.Printf("New GetAvailableSubscriptionGroup received! from: %v", p.UserID)

	userObjID, err := primitive.ObjectIDFromHex(p.UserID)
	if err != nil {
		o := &pb.CreateSubscribeToCreatorResponse{}
		return o, err
	}

	creatorObjID, err := primitive.ObjectIDFromHex(p.CreatorID)
	if err != nil {
		o := &pb.CreateSubscribeToCreatorResponse{}
		return o, err
	}

	tierResult, err := h.store.GetSubscriptionTierByCreatorIDAndTier(creatorObjID, int(p.Tier))
	if err != nil {
		log.Println("Error fetching tier:", err)
	}

	_, err = h.store.CreateSubscriptionFromTier(userObjID, creatorObjID, *tierResult, p.ProductID)
	if err != nil {
		log.Println("Error creating subscription:", err)
	}

	o := &pb.CreateSubscribeToCreatorResponse{Status: "successfull"}
	return o, nil
}
