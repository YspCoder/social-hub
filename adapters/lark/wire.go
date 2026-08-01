package lark

import "encoding/json"

type wireSender struct {
	ID         string `json:"id"`
	IDType     string `json:"id_type"`
	SenderType string `json:"sender_type"`
	TenantKey  string `json:"tenant_key"`
}

type wireMessageBody struct {
	Content string `json:"content"`
}

type wireMessage struct {
	MessageID      string          `json:"message_id"`
	RootID         string          `json:"root_id"`
	ParentID       string          `json:"parent_id"`
	ThreadID       string          `json:"thread_id"`
	MessageType    string          `json:"msg_type"`
	CreateTime     string          `json:"create_time"`
	UpdateTime     string          `json:"update_time"`
	Deleted        bool            `json:"deleted"`
	Updated        bool            `json:"updated"`
	ChatID         string          `json:"chat_id"`
	Sender         wireSender      `json:"sender"`
	Body           wireMessageBody `json:"body"`
	MessageAppLink string          `json:"message_app_link"`
}

type wireAvatar struct {
	AvatarOrigin string `json:"avatar_origin"`
	Avatar640    string `json:"avatar_640"`
	Avatar240    string `json:"avatar_240"`
	Avatar72     string `json:"avatar_72"`
}

type wireUser struct {
	OpenID   string          `json:"open_id"`
	UnionID  string          `json:"union_id"`
	UserID   string          `json:"user_id"`
	Name     string          `json:"name"`
	EnName   string          `json:"en_name"`
	Nickname string          `json:"nickname"`
	Status   json.RawMessage `json:"status"`
	Avatar   wireAvatar      `json:"avatar"`
}

type wireReaction struct {
	ReactionID string `json:"reaction_id"`
	ActionTime string `json:"action_time"`
	Operator   struct {
		OperatorID   string `json:"operator_id"`
		OperatorType string `json:"operator_type"`
	} `json:"operator"`
	ReactionType struct {
		EmojiType string `json:"emoji_type"`
	} `json:"reaction_type"`
}

type eventHeader struct {
	EventID    string `json:"event_id"`
	EventType  string `json:"event_type"`
	AppID      string `json:"app_id"`
	TenantKey  string `json:"tenant_key"`
	CreateTime string `json:"create_time"`
	Token      string `json:"token"`
}

type eventEnvelope struct {
	Schema    string          `json:"schema"`
	Type      string          `json:"type"`
	UUID      string          `json:"uuid"`
	Token     string          `json:"token"`
	Challenge string          `json:"challenge"`
	Header    *eventHeader    `json:"header"`
	Event     json.RawMessage `json:"event"`
	Encrypt   string          `json:"encrypt"`
}

type eventUserID struct {
	OpenID  string `json:"open_id"`
	UnionID string `json:"union_id"`
	UserID  string `json:"user_id"`
}

type messageEvent struct {
	Sender struct {
		SenderID   eventUserID `json:"sender_id"`
		SenderType string      `json:"sender_type"`
		TenantKey  string      `json:"tenant_key"`
	} `json:"sender"`
	Message struct {
		MessageID   string `json:"message_id"`
		RootID      string `json:"root_id"`
		ParentID    string `json:"parent_id"`
		ThreadID    string `json:"thread_id"`
		CreateTime  string `json:"create_time"`
		UpdateTime  string `json:"update_time"`
		ChatID      string `json:"chat_id"`
		ChatType    string `json:"chat_type"`
		MessageType string `json:"message_type"`
		Content     string `json:"content"`
	} `json:"message"`
}

type reactionEvent struct {
	MessageID    string      `json:"message_id"`
	UserID       eventUserID `json:"user_id"`
	OperatorType string      `json:"operator_type"`
	ActionTime   string      `json:"action_time"`
	ReactionType struct {
		EmojiType string `json:"emoji_type"`
	} `json:"reaction_type"`
}
