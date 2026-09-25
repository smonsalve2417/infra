package main

import "context"

type service struct {
}

func NewService() *service {
	return &service{}
}

func (s *service) SendUserCode(context.Context) error {
	return nil
}
func (s *service) LoginUser(context.Context) error {
	return nil
}
func (s *service) ValidateUser(context.Context) error {
	return nil
}
func (s *service) RegisterUser(context.Context) error {
	return nil
}
func (s *service) GetUser(context.Context) error {
	return nil
}
func (s *service) GetUserByID(context.Context) error {
	return nil
}
func (s *service) UpdateAvatar(context.Context) error {
	return nil
}
func (s *service) UpdateBanner(context.Context) error {
	return nil
}

func (s *service) FollowUser(context.Context) error {
	return nil
}
func (s *service) UnFollowUser(context.Context) error {
	return nil
}

func (s *service) GetUserFollow(context.Context) error {
	return nil
}

func (s *service) UpdateDescription(context.Context) error {
	return nil
}

func (s *service) AddExpoToken(context.Context) error {
	return nil
}

func (s *service) DeleteExpoToken(context.Context) error {
	return nil
}

func (s *service) UpdateNotificationsSettings(context.Context) error {
	return nil
}

func (s *service) GetUserNotificationSettings(context.Context) error {
	return nil
}

func (s *service) SearchCreators(context.Context) error {
	return nil
}

func (s *service) GetTopCreator(context.Context) error {
	return nil
}

func (s *service) ChangePassword(context.Context) error {
	return nil
}

func (s *service) UpdateUsername(context.Context) error {
	return nil
}

func (s *service) GetChatSettings(context.Context) error {
	return nil
}

func (s *service) GetTransactions(context.Context) error {
	return nil
}

func (s *service) CreateSubscriptionTier(context.Context) error {
	return nil
}

func (s *service) GetSubscriptionTier(context.Context) error {
	return nil
}
