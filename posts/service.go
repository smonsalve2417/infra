package main

import "context"

type service struct {
}

func NewService() *service {
	return &service{}
}

func (s *service) CreatePost(context.Context) error {
	return nil
}

func (s *service) DeletePost(context.Context) error {
	return nil
}

func (s *service) LikePost(context.Context) error {
	return nil
}

func (s *service) UnlikePost(context.Context) error {
	return nil
}

func (s *service) CreateComment(context.Context) error {
	return nil
}

func (s *service) DeleteComment(context.Context) error {
	return nil
}

func (s *service) LikeComment(context.Context) error {
	return nil
}

func (s *service) UnLikeComment(context.Context) error {
	return nil
}

func (s *service) GetUserCommentLike(context.Context) error {
	return nil
}

func (s *service) CreateReply(context.Context) error {
	return nil
}

func (s *service) DeleteReply(context.Context) error {
	return nil
}

func (s *service) GetLatestPosts(context.Context) error {
	return nil
}

func (s *service) GetLatestUserPosts(context.Context) error {
	return nil
}

func (s *service) GetUserLike(context.Context) error {
	return nil
}

func (s *service) GetLatestComments(context.Context) error {
	return nil
}

func (s *service) GetLatestReplies(context.Context) error {
	return nil
}

func (s *service) GetPost(context.Context) error {
	return nil
}

func (s *service) GetRecommended(context.Context) error {
	return nil
}

func (s *service) SharePost(context.Context) error {
	return nil
}
