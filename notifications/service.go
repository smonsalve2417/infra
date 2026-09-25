package main

import (
	"context"
	"log"

	common "github.com/Eskiwi-Organization/infra/commons"
	pb "github.com/Eskiwi-Organization/infra/commons/api"
)

type service struct {
	mailClient *MailClient
	store      NotificationStore
}

func NewService(mailClient *MailClient, store NotificationStore) *service {
	return &service{mailClient: mailClient, store: store}
}

func (s *service) SendUserCode(ctx context.Context, p *pb.SendUserCodeRequest) error {
	log.Printf("New SendUserCode received! Notification to: %v", p.Email)
	code, _ := common.GenerateRandomCode()
	err := s.mailClient.SendMail(p.Email, "Eskiwi Code verification", code)
	if err != nil {
		err = s.store.UpdateUserCode(p.Email, code)
		log.Printf("error sending mail process: %v", err)
		return err
	}

	err = s.store.UpdateUserCode(p.Email, code)
	if err != nil {
		log.Printf("error updating code process: %v", err)
		return err
	}

	return nil
}

func (s *service) CreatePostNotification(ctx context.Context, post common.Post) error {
	log.Printf("New CreatePostNotification received! Notification of: %v", post.ID)

	err := s.store.CreatePostNotifications(post.ID, post.User_id, "newPost")
	if err != nil {
		log.Printf("error creating postNotification: %v", err)
		return err
	}

	return nil
}

func (s *service) CreatePostLikeNotification(ctx context.Context, payload common.LikeNotificationPayload) error {
	log.Printf("New CreatePostLikeNotification received! Notification of: %v", payload.Post_id)

	err := s.store.CreatePostLikeNotifications(payload.Post_id, payload.User_id, "Like")
	if err != nil {
		log.Printf("error creating postNotification: %v", err)
		return err
	}

	return nil
}

func (s *service) CreatePostCommentNotification(ctx context.Context, payload common.LikeNotificationPayload) error {
	log.Printf("New CreatePostCommentNotification received! Notification of: %v", payload.Post_id)

	err := s.store.CreatePostCommentNotifications(payload.Post_id, payload.Comment_id, payload.User_id, "Comment")
	if err != nil {
		log.Printf("error creating commentNotification: %v", err)
		return err
	}

	return nil
}

func (s *service) CreateUserFollowNotification(ctx context.Context, payload common.FollowNotificationPayload) error {
	log.Printf("New CreateUserFollowNotification received! Notification of: %v", payload.User_id)

	err := s.store.CreateUserFollowNotification(payload.Creator_id, payload.User_id, "Follow")
	if err != nil {
		log.Printf("error creating FollowNotification: %v", err)
		return err
	}

	return nil
}

func (s *service) GetLatestNotifications(context.Context) error {
	return nil
}

func (s *service) CreateChatmsgNotification(ctx context.Context, payload common.ChatNotificationPayload) error {
	log.Printf("New CreateChatmsgNotification received! Notification of: %v", payload.User_id)

	err := s.store.CreateChatmsgNotification(payload.Chat_id, payload.User_id, payload.Text)
	if err != nil {
		log.Printf("error creating FollowNotification: %v", err)
		return err
	}

	return nil
}
