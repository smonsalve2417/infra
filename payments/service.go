package main

import (
	"context"
	"fmt"
	"log"
	"time"

	common "github.com/Eskiwi-Organization/infra/commons"
)

type service struct {
	store PaymentsStore
}

func NewService(store PaymentsStore) *service {
	return &service{store: store}
}

func (s *service) RegisterComment(ctx context.Context, comment common.Comment) error {
	log.Printf("New RegisterComment received! Notification")

	//Se busca la persona en MONGO
	user, err := s.store.GetUserByID(comment.UserID.Hex())
	if err != nil {
		log.Printf("error getting user from post: %v", err)
		return err
	}
	//Se verifica si ya existe en SQL
	exists, err := s.store.UserExists(user.ID.Hex())
	if err != nil {
		log.Fatalf("error checking user existence: %v", err)
	}
	if !exists {
		err := s.store.CreateUser(user)
		if err != nil {
			log.Fatalf("could not create user: %v", err)
		}
	}

	//Se saca la info de SQL
	usersql, err := s.store.GetUserByIDsql(comment.UserID.Hex())
	if err != nil {
		log.Printf("error getting user from sql: %v", err)
		return err
	}
	//se verifica si tiene las gemas necesarias
	if usersql.Gems < int(comment.Gems) {
		log.Printf("less than requiered gems in wallet")
		err = fmt.Errorf("less than requiered gems in wallet")
		return err
	}

	//se le quitan las gemas
	newUserGems := usersql.Gems - int(comment.Gems)

	log.Printf("gems of user %v", usersql.Gems)

	err = s.store.UpdateUserGems(usersql.ID.Hex(), newUserGems)
	if err != nil {
		log.Printf("error updating user gems in sql: %v", err)
		return err
	}
	log.Printf("gems updated %v", newUserGems)

	err = s.store.UpdateUserGemsMongo(user.ID, newUserGems)
	if err != nil {
		log.Printf("error updating user gems in mongo: %v", err)
		return err
	}
	//se busca el post para sacar al creador
	post, err := s.store.GetPostByID(comment.PostID)
	if err != nil {
		log.Printf("error getting user from post: %v", err)
		return err
	}

	//se le dan las gemas al creador
	creator, err := s.store.GetUserByID(post.User_id.Hex())
	if err != nil {
		log.Printf("error getting creator from mongo: %v", err)
		return err
	}

	//se verifica si ya existe
	exists, err = s.store.UserExists(creator.ID.Hex())
	if err != nil {
		log.Fatalf("error checking creator existence: %v", err)
	}
	if !exists {
		err := s.store.CreateUser(creator)
		if err != nil {
			log.Fatalf("could not create user: %v", err)
		}
	}

	donation := &GemDonation{
		SenderID:   comment.UserID.Hex(),
		ReceiverID: post.User_id.Hex(),
		PostID:     comment.PostID.Hex(),
		CommentID:  comment.ID.Hex(),
		CreatedAt:  time.Now(),
		Valid:      true,
		GemAmount:  int(comment.Gems),
	}

	err = s.store.CreateGemDonation(donation)
	if err != nil {
		log.Printf("error creating donation in sql: %v", err)
		return err
	}

	err = s.store.UpdatePostGemsMongo(post.ID, int(comment.Gems))
	if err != nil {
		log.Printf("error creating donation in sql: %v", err)
		return err
	}

	return nil
}

func (s *service) RegisterChat(ctx context.Context, ChatCreation common.CreateChatDonationPayload) error {
	log.Printf("New RegisterChat received! Notification")
	log.Printf("amount of gems in function: %v", ChatCreation.Gems)

	exists, err := s.store.ChatGemExists(ChatCreation.User_id.Hex(), ChatCreation.Creator_id.Hex())
	if err != nil {
		log.Printf("Error checking chat gem existence: %v", err)
		return err
	} else if exists {
		log.Println("Chat gem already exists between sender and receiver")
		return err
	}

	//Se busca la persona en MONGO
	user, err := s.store.GetUserByID(ChatCreation.User_id.Hex())
	if err != nil {
		log.Printf("error getting user from post: %v", err)
		return err
	}
	log.Print("first user found")
	//Se verifica si ya existe en SQL
	exists, err = s.store.UserExists(user.ID.Hex())
	if err != nil {
		log.Fatalf("error checking user existence: %v", err)
	}
	if !exists {
		err := s.store.CreateUser(user)
		if err != nil {
			log.Fatalf("could not create user: %v", err)
		}
	}

	//Se saca la info de SQL
	usersql, err := s.store.GetUserByIDsql(ChatCreation.User_id.Hex())
	if err != nil {
		log.Printf("error getting user from sql: %v", err)
		return err
	}
	//se verifica si tiene las gemas necesarias
	if usersql.Gems < int(ChatCreation.Gems) {
		log.Printf("less than requiered gems in wallet")
		err = fmt.Errorf("less than requiered gems in wallet")
		return err
	}
	log.Print("checked he has the gems")

	//se le quitan las gemas
	newUserGems := usersql.Gems - int(ChatCreation.Gems)

	log.Printf("gems of user %v", usersql.Gems)

	err = s.store.UpdateUserGems(usersql.ID.Hex(), newUserGems)
	if err != nil {
		log.Printf("error updating user gems: %v", err)
		return err
	}
	log.Printf("gems updated %v", newUserGems)

	err = s.store.UpdateUserGemsMongo(user.ID, newUserGems)
	if err != nil {
		log.Printf("error updating user gems: %v", err)
		return err
	}

	creator, err := s.store.GetUserByID(ChatCreation.Creator_id.Hex())
	if err != nil {
		log.Printf("error getting creator from mongo: %v", err)
		return err
	}

	log.Print("creator found in mongo")
	//se verifica si ya existe
	exists, err = s.store.UserExists(creator.ID.Hex())
	if err != nil {
		log.Fatalf("error checking creator existence: %v", err)
	}
	if !exists {
		err := s.store.CreateUser(creator)
		if err != nil {
			log.Fatalf("could not create user: %v", err)
		}
	}

	chatRequest := &ChatGem{
		ChatID:     ChatCreation.ChatID.Hex(),
		SenderID:   ChatCreation.User_id.Hex(),
		ReceiverID: ChatCreation.Creator_id.Hex(),
		CreatedAt:  time.Now(),
		Valid:      true,
		Denied:     false,
		GemAmount:  int(ChatCreation.Gems),
		Accepted:   false,
	}

	err = s.store.CreateChatGem(chatRequest)
	if err != nil {
		log.Printf("error creating Chat Request in sql: %v", err)
		return err
	}

	log.Print("termino el proceso")

	return nil
}

func (s *service) AcceptChat(ctx context.Context, ChatCreation common.AcceptChatPayload) error {
	log.Printf("New AcceptChat received! Notification")

	creatorID, userID, err := s.store.GetParticipantsByChatID(ChatCreation.ChatID)
	if err != nil {
		log.Printf("Error retrieving creator ID: %v", err)
		return err
	}
	//Se busca la persona en MONGO
	user, err := s.store.GetUserByID(creatorID.Hex())
	if err != nil {
		log.Printf("error getting user from post: %v", err)
		return err
	}
	log.Print("first user found")
	//Se verifica si ya existe en SQL
	exists, err := s.store.UserExists(user.ID.Hex())
	if err != nil {
		log.Fatalf("error checking user existence: %v", err)
		return err
	}
	if !exists {
		err := s.store.CreateUser(user)
		if err != nil {
			log.Fatalf("could not create user: %v", err)
			return err
		}
	}

	_, err = s.store.FindAndAcceptChatGem(userID.Hex(), creatorID.Hex())
	if err != nil {
		log.Printf("Error: %v", err)
		return err
	}

	log.Print("termino el proceso")

	return nil
}

func (s *service) DenyChat(ctx context.Context, ChatCreation common.AcceptChatPayload) error {
	log.Printf("New DenyChat received! Notification")

	creatorID, userID, err := s.store.GetParticipantsByChatID(ChatCreation.ChatID)
	if err != nil {
		log.Printf("Error retrieving creator ID: %v", err)
		return err
	}
	//Se busca la persona en MONGO
	user, err := s.store.GetUserByID(userID.Hex())
	if err != nil {
		log.Printf("error getting user from post: %v", err)
		return err
	}
	log.Print("first user found")
	//Se verifica si ya existe en SQL
	exists, err := s.store.UserExists(user.ID.Hex())
	if err != nil {
		log.Fatalf("error checking user existence: %v", err)
		return err
	}
	if !exists {
		err := s.store.CreateUser(user)
		if err != nil {
			log.Fatalf("could not create user: %v", err)
			return err
		}
	}

	gemAmount, err := s.store.GetChatGemAmount(userID.Hex(), creatorID.Hex())
	if err != nil {
		log.Printf("Error retrieving gem amount: %v", err)
		return err
	}

	//Se saca la info de SQL
	usersql, err := s.store.GetUserByIDsql(userID.Hex())
	if err != nil {
		log.Printf("error getting user from sql: %v", err)
		return err
	}

	//se le devuelven las gemas
	newUserGems := usersql.Gems + gemAmount

	log.Printf("gems of user %v", usersql.Gems)

	err = s.store.UpdateUserGems(usersql.ID.Hex(), newUserGems)
	if err != nil {
		log.Printf("error updating user gems: %v", err)
		return err
	}
	log.Printf("gems updated %v", newUserGems)

	log.Print("------------------------------------------------")
	log.Printf("userid to update mongo: %v", user.ID)
	log.Printf("userid to update by sqlid: %v", usersql.ID)
	log.Printf("gems to add mongo: %v", newUserGems)

	err = s.store.UpdateUserGemsMongo(usersql.ID, newUserGems)
	if err != nil {
		log.Printf("error updating user gems: %v", err)
		return err
	}

	err = s.store.DenyChatGem(ChatCreation.ChatID.Hex())
	if err != nil {
		log.Printf("Error: %v", err)
		return err
	}

	log.Print("termino el proceso")

	return nil
}

func (s *service) RegisterChatMessage(ctx context.Context, chatMessage common.GemsOnChatPayload) (int, error) {
	log.Printf("New RegisterChatMessage received! Notification")

	//Se busca la persona en MONGO
	user, err := s.store.GetUserByID(chatMessage.UserID.Hex())
	if err != nil {
		log.Printf("error getting user from post: %v", err)
		return 0, err
	}
	log.Print("first user found")

	//Se saca la info de SQL
	usersql, err := s.store.GetUserByIDsql(chatMessage.UserID.Hex())
	if err != nil {
		log.Printf("error getting user from sql: %v", err)
		return 0, err
	}
	//se verifica si tiene las gemas necesarias
	if usersql.Gems < int(chatMessage.Gems) {
		log.Printf("less than requiered gems in wallet")
		err = fmt.Errorf("less than requiered gems in wallet")
		return 0, err
	}
	log.Print("checked he has the gems")

	//se le quitan las gemas
	newUserGems := usersql.Gems - int(chatMessage.Gems)

	log.Printf("gems of user :%v", usersql.Gems)

	err = s.store.UpdateUserGems(usersql.ID.Hex(), newUserGems)
	if err != nil {
		log.Printf("error updating user gems: %v", err)
		return 0, err
	}
	log.Printf("gems updated to :%v", newUserGems)

	err = s.store.UpdateUserGemsMongo(user.ID, newUserGems)
	if err != nil {
		log.Printf("error updating user gems: %v", err)
		return 0, err
	}

	var valid = true

	if chatMessage.Type != "message" {
		valid = false
	}

	messageGem := &MessageGem{
		ChatID:     chatMessage.ChatID.Hex(), // The chat where the gem transaction happens
		SenderID:   chatMessage.UserID.Hex(),
		ReceiverID: chatMessage.CreatorID.Hex(),
		Valid:      valid, // Default is true, but can be set manually
		GemAmount:  chatMessage.Gems,
	}

	transactionID, err := s.store.CreateMessageGem(context.Background(), messageGem)
	if err != nil {
		log.Printf("Error creating message gem: %v", err)
	}

	log.Print("termino el proceso")

	return transactionID, nil
}

func (s *service) AcceptReq(ctx context.Context, transaction common.AcceptMessageReqPayload) error {
	log.Printf("New AcceptReq received! Notification")

	err := s.store.SetMessageGemValid(int64(transaction.TransID))
	if err != nil {
		log.Fatalf("error setting valid message: %v", err)
		return err
	}
	log.Print("termino el proceso")

	return nil
}

func (s *service) RejectReq(ctx context.Context, transaction common.AcceptMessageReqPayload) error {
	log.Printf("New AcceptReq received! Notification")

	senderID, gemAmount, err := s.store.DeleteMessageGemAndGetSenderIDAndGemAmount(int64(transaction.TransID))
	if err != nil {
		log.Fatalf("error deleting request message: %v", err)
		return err
	}

	user, err := s.store.GetUserByID(senderID)
	if err != nil {
		log.Printf("error getting user from post: %v", err)
		return err
	}
	log.Print("first user found")
	//Se verifica si ya existe en SQL
	exists, err := s.store.UserExists(user.ID.Hex())
	if err != nil {
		log.Fatalf("error checking user existence: %v", err)
		return err
	}
	if !exists {
		err := s.store.CreateUser(user)
		if err != nil {
			log.Fatalf("could not create user: %v", err)
			return err
		}
	}

	//Se saca la info de SQL
	usersql, err := s.store.GetUserByIDsql(senderID)
	if err != nil {
		log.Printf("error getting user from sql: %v", err)
		return err
	}

	//se le devuelven las gemas
	newUserGems := usersql.Gems + gemAmount

	log.Printf("gems of user %v", usersql.Gems)

	err = s.store.UpdateUserGems(usersql.ID.Hex(), newUserGems)
	if err != nil {
		log.Printf("error updating user gems: %v", err)
		return err
	}
	log.Printf("gems updated %v", newUserGems)

	err = s.store.UpdateUserGemsMongo(usersql.ID, newUserGems)
	if err != nil {
		log.Printf("error updating user gems: %v", err)
		return err
	}

	log.Print("termino el proceso")

	return nil
}

func (s *service) GetAvailableSubscriptionGroup(context.Context) error {
	return nil
}

func (s *service) CreateSubscribeToCreator(context.Context) error {
	return nil
}
