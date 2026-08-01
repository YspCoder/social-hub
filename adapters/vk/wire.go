package vk

import "encoding/json"

type wireUser struct {
	ID              int64  `json:"id"`
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name"`
	ScreenName      string `json:"screen_name"`
	Domain          string `json:"domain"`
	Photo200        string `json:"photo_200"`
	Deactivated     string `json:"deactivated"`
	IsClosed        int    `json:"is_closed"`
	CanAccessClosed int    `json:"can_access_closed"`
}

type wireGroup struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	ScreenName   string `json:"screen_name"`
	Type         string `json:"type"`
	Photo200     string `json:"photo_200"`
	Deactivated  string `json:"deactivated"`
	Description  string `json:"description"`
	MembersCount int    `json:"members_count"`
}

type wireCount struct {
	Count int `json:"count"`
}

type wirePhotoSize struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type wirePhoto struct {
	ID        int64           `json:"id"`
	OwnerID   int64           `json:"owner_id"`
	Date      int64           `json:"date"`
	Text      string          `json:"text"`
	Width     int             `json:"width"`
	Height    int             `json:"height"`
	AccessKey string          `json:"access_key"`
	Sizes     []wirePhotoSize `json:"sizes"`
}

type wireVideoImage struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type wireVideo struct {
	ID       int64            `json:"id"`
	OwnerID  int64            `json:"owner_id"`
	Duration int64            `json:"duration"`
	Image    []wireVideoImage `json:"image"`
}

type wireAudio struct {
	ID       int64  `json:"id"`
	OwnerID  int64  `json:"owner_id"`
	Duration int64  `json:"duration"`
	URL      string `json:"url"`
}

type wireDocument struct {
	ID      int64  `json:"id"`
	OwnerID int64  `json:"owner_id"`
	Size    int64  `json:"size"`
	Ext     string `json:"ext"`
	URL     string `json:"url"`
}

type wireAttachment struct {
	Type  string       `json:"type"`
	Photo wirePhoto    `json:"photo"`
	Video wireVideo    `json:"video"`
	Audio wireAudio    `json:"audio"`
	Doc   wireDocument `json:"doc"`
}

type wirePost struct {
	ID          int64            `json:"id"`
	OwnerID     int64            `json:"owner_id"`
	FromID      int64            `json:"from_id"`
	Date        int64            `json:"date"`
	Text        string           `json:"text"`
	FriendsOnly int              `json:"friends_only"`
	PostType    string           `json:"post_type"`
	Attachments []wireAttachment `json:"attachments"`
	Comments    wireCount        `json:"comments"`
	Likes       wireCount        `json:"likes"`
	Reposts     wireCount        `json:"reposts"`
	Views       wireCount        `json:"views"`
	CopyHistory []wirePost       `json:"copy_history"`
}

type wireComment struct {
	ID             int64     `json:"id"`
	FromID         int64     `json:"from_id"`
	Date           int64     `json:"date"`
	Text           string    `json:"text"`
	ReplyToComment int64     `json:"reply_to_comment"`
	PostID         int64     `json:"post_id"`
	PostOwnerID    int64     `json:"post_owner_id"`
	OwnerID        int64     `json:"owner_id"`
	Likes          wireCount `json:"likes"`
	Thread         wireCount `json:"thread"`
}

type wireMessage struct {
	ID                    int64            `json:"id"`
	ConversationMessageID int64            `json:"conversation_message_id"`
	Date                  int64            `json:"date"`
	FromID                int64            `json:"from_id"`
	PeerID                int64            `json:"peer_id"`
	Text                  string           `json:"text"`
	Out                   int              `json:"out"`
	Attachments           []wireAttachment `json:"attachments"`
	ReplyMessage          *wireMessage     `json:"reply_message"`
}

type callbackEnvelope struct {
	Type    string          `json:"type"`
	Object  json.RawMessage `json:"object"`
	GroupID int64           `json:"group_id"`
	EventID string          `json:"event_id"`
	Version string          `json:"v"`
	Secret  string          `json:"secret"`
}
