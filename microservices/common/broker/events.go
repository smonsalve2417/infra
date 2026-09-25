package broker

const (
	SendCodemailCreatedEvent = "sendCodeMail.created"
	SendCodemailSentEvent    = "sendCodeMail.sent"

	//Phone notifications
	SendPostsNotificationCreatedEvent = "SendPostsNotification.created"
	SendPostsNotificationSentEvent    = "SendPostsNotification.sent"

	SendLikeNotificationCreatedEvent = "SendLikeNotification.created"
	SendLikeNotificationSentEvent    = "SendLikeNotification.sent"

	SendCommentNotificationCreatedEvent = "SendCommentNotification.created"
	SendCommentNotificationSentEvent    = "SendCommentNotification.sent"

	SendFollowNotificationCreatedEvent = "SendFollowNotification.created"
	SendFollowNotificationSentEvent    = "SendFollowNotification.sent"

	SendChatNotificationCreatedEvent = "SendChatNotification.created"
	SendChatNotificationSentEvent    = "SendChatNotification.sent"

	//Payments
	SendDonationCommentCreatedEvent = "SendDonationComment.created"
	SendDonationCommentSentEvent    = "SendDonationComment.sent"

	SendCreateChatCreatedEvent = "SendCreateChat.created"
	SendCreateChatSentEvent    = "SendCreateChat.sent"

	SendAcceptChatCreatedEvent = "SendAcceptChat.created"
	SendAcceptChatSentEvent    = "SendAcceptChat.sent"

	SendDenyChatCreatedEvent = "SendDenyChat.created"
	SendDenyChatSentEvent    = "SendDenyChat.sent"

	SendMessageGemsCreatedEvent = "SendMessageGems.created"
	SendMessageGemsSentEvent    = "SendMessageGems.sent"
)
