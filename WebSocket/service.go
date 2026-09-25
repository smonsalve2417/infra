package main

import "context"

type service struct {
}

func NewService() *service {
	return &service{}
}

func (s *service) CreateChat(context.Context) error {
	return nil
}

func (s *service) CreateRoom(context.Context) error {
	return nil
}

func (s *service) GetLatestsChats(context.Context) error {
	return nil
}

func (s *service) GetLatestsRequestChats(context.Context) error {
	return nil
}

func (s *service) GetChat(context.Context) error {
	return nil
}

func (s *service) GetLatestsMessages(context.Context) error {
	return nil
}

func (s *service) AcceptChatReq(context.Context) error {
	return nil
}

func (s *service) RejectChatReq(context.Context) error {
	return nil
}

func (s *service) AcceptChatInsideReq(context.Context) error {
	return nil
}

func (s *service) RejectChatInsideReq(context.Context) error {
	return nil
}
