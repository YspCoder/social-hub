package bilibililive

import (
	"context"
	"encoding/json"

	"social-hub/pkg/socialhub"
)

type startProjectRequest struct {
	Code  string `json:"code"`
	AppID int64  `json:"app_id"`
}

type endProjectRequest struct {
	AppID  int64  `json:"app_id"`
	GameID string `json:"game_id"`
}

type heartbeatRequest struct {
	GameID string `json:"game_id"`
}

type batchHeartbeatRequest struct {
	GameIDs []string `json:"game_ids"`
}

// GameInfo identifies a started Bilibili project session.
type GameInfo struct {
	GameID string `json:"game_id"`
}

// WebSocketInfo contains the opaque authorization body and cluster links
// returned by StartProject. Pass the unchanged value back to the same Client.
type WebSocketInfo struct {
	AuthBody string   `json:"auth_body"`
	Links    []string `json:"wss_link"`

	client      *Client
	fingerprint [32]byte
}

// AnchorInfo identifies the authorized broadcaster and room.
type AnchorInfo struct {
	RoomID  int64  `json:"room_id"`
	Name    string `json:"uname"`
	FaceURL string `json:"uface"`
	UID     int64  `json:"uid"`
	OpenID  string `json:"open_id"`
	UnionID string `json:"union_id"`
}

// ProjectSession is the lifecycle and message-stream material returned by
// StartProject. End must be called when the project finishes.
type ProjectSession struct {
	GameInfo      GameInfo      `json:"game_info"`
	WebSocketInfo WebSocketInfo `json:"websocket_info"`
	AnchorInfo    AnchorInfo    `json:"anchor_info"`

	client *Client
}

// BatchHeartbeatResult reports game IDs that the platform could not renew.
type BatchHeartbeatResult struct {
	FailedGameIDs []string `json:"failed_game_ids"`
}

// Command identifies a documented Live Open Platform event.
type Command string

const (
	CommandDanmaku         Command = "LIVE_OPEN_PLATFORM_DM"
	CommandMirrorDanmaku   Command = "LIVE_OPEN_PLATFORM_DM_MIRROR"
	CommandGift            Command = "LIVE_OPEN_PLATFORM_SEND_GIFT"
	CommandSuperChat       Command = "LIVE_OPEN_PLATFORM_SUPER_CHAT"
	CommandSuperChatDelete Command = "LIVE_OPEN_PLATFORM_SUPER_CHAT_DEL"
	CommandGuard           Command = "LIVE_OPEN_PLATFORM_GUARD"
	CommandLike            Command = "LIVE_OPEN_PLATFORM_LIKE"
	CommandRoomEnter       Command = "LIVE_OPEN_PLATFORM_LIVE_ROOM_ENTER"
	CommandLiveStart       Command = "LIVE_OPEN_PLATFORM_LIVE_START"
	CommandLiveEnd         Command = "LIVE_OPEN_PLATFORM_LIVE_END"
	CommandInteractionEnd  Command = "LIVE_OPEN_PLATFORM_INTERACTION_END"
)

// Message preserves raw command data while exposing known commands as typed
// payloads through Data.
type Message struct {
	Command Command
	ID      string
	Data    any
	RawData json.RawMessage
	Raw     json.RawMessage
}

// SocialEvent maps a live command to the SDK's common event envelope.
func (message Message) SocialEvent(accountID socialhub.AccountID) socialhub.Event {
	return socialhub.Event{
		ID: message.ID, Type: string(message.Command), Platform: "bilibili", AccountID: accountID, Payload: message,
	}
}

// ProjectSessionWorkflow describes the convenience methods attached to a
// started project session.
type ProjectSessionWorkflow interface {
	End(context.Context, ...socialhub.CallOption) error
	Heartbeat(context.Context, ...socialhub.CallOption) error
	ConnectMessages(context.Context, ...StreamOption) (*MessageStream, error)
}

// UserInfo is the app-scoped identity carried by live events. UID is retained
// for wire compatibility but is deprecated by Bilibili and normally zero.
type UserInfo struct {
	UID     int64  `json:"uid"`
	OpenID  string `json:"open_id"`
	UnionID string `json:"union_id"`
	Name    string `json:"uname"`
	FaceURL string `json:"uface"`
}

// DanmakuData is a typed LIVE_OPEN_PLATFORM_DM payload.
type DanmakuData struct {
	RoomID                 int64  `json:"room_id"`
	UID                    int64  `json:"uid"`
	OpenID                 string `json:"open_id"`
	UnionID                string `json:"union_id"`
	Name                   string `json:"uname"`
	FaceURL                string `json:"uface"`
	Message                string `json:"msg"`
	MessageID              string `json:"msg_id"`
	Timestamp              int64  `json:"timestamp"`
	GuardLevel             int64  `json:"guard_level"`
	FansMedalWearingStatus bool   `json:"fans_medal_wearing_status"`
	FansMedalName          string `json:"fans_medal_name"`
	FansMedalLevel         int64  `json:"fans_medal_level"`
	EmojiImageURL          string `json:"emoji_img_url"`
	DanmakuType            int64  `json:"dm_type"`
	GloryLevel             int64  `json:"glory_level"`
	ReplyOpenID            string `json:"reply_open_id"`
	ReplyName              string `json:"reply_uname"`
	IsAdmin                int64  `json:"is_admin"`
}

// MirrorDanmakuData is the privacy-reduced cross-room danmaku payload.
type MirrorDanmakuData struct {
	RoomID        int64  `json:"room_id"`
	Message       string `json:"msg"`
	MessageID     string `json:"msg_id"`
	Timestamp     int64  `json:"timestamp"`
	EmojiImageURL string `json:"emoji_img_url"`
	DanmakuType   int64  `json:"dm_type"`
}

// ComboInfo describes a repeated gift combo.
type ComboInfo struct {
	BaseNumber int64  `json:"combo_base_num"`
	Count      int64  `json:"combo_count"`
	ID         string `json:"combo_id"`
	Timeout    int64  `json:"combo_timeout"`
}

// BlindGiftInfo identifies a blind-box gift result.
type BlindGiftInfo struct {
	ID     int64 `json:"blind_gift_id"`
	Status bool  `json:"status"`
}

// GiftData is a typed LIVE_OPEN_PLATFORM_SEND_GIFT payload.
type GiftData struct {
	RoomID                 int64         `json:"room_id"`
	UID                    int64         `json:"uid"`
	OpenID                 string        `json:"open_id"`
	UnionID                string        `json:"union_id"`
	Name                   string        `json:"uname"`
	FaceURL                string        `json:"uface"`
	GiftID                 int64         `json:"gift_id"`
	GiftName               string        `json:"gift_name"`
	GiftNumber             int64         `json:"gift_num"`
	Price                  int64         `json:"price"`
	RealPrice              int64         `json:"r_price"`
	Paid                   bool          `json:"paid"`
	FansMedalLevel         int64         `json:"fans_medal_level"`
	FansMedalName          string        `json:"fans_medal_name"`
	FansMedalWearingStatus bool          `json:"fans_medal_wearing_status"`
	GuardLevel             int64         `json:"guard_level"`
	Timestamp              int64         `json:"timestamp"`
	MessageID              string        `json:"msg_id"`
	Anchor                 UserInfo      `json:"anchor_info"`
	GiftIconURL            string        `json:"gift_icon"`
	ComboGift              bool          `json:"combo_gift"`
	Combo                  ComboInfo     `json:"combo_info"`
	BlindGift              BlindGiftInfo `json:"blind_gift"`
}

// SuperChatData is a typed LIVE_OPEN_PLATFORM_SUPER_CHAT payload.
type SuperChatData struct {
	RoomID                 int64  `json:"room_id"`
	UID                    int64  `json:"uid"`
	OpenID                 string `json:"open_id"`
	UnionID                string `json:"union_id"`
	Name                   string `json:"uname"`
	FaceURL                string `json:"uface"`
	MessageID              int64  `json:"message_id"`
	Message                string `json:"message"`
	EventID                string `json:"msg_id"`
	RMB                    int64  `json:"rmb"`
	Timestamp              int64  `json:"timestamp"`
	StartTime              int64  `json:"start_time"`
	EndTime                int64  `json:"end_time"`
	GuardLevel             int64  `json:"guard_level"`
	FansMedalLevel         int64  `json:"fans_medal_level"`
	FansMedalName          string `json:"fans_medal_name"`
	FansMedalWearingStatus bool   `json:"fans_medal_wearing_status"`
}

// SuperChatDeleteData identifies paid messages removed by moderation.
type SuperChatDeleteData struct {
	RoomID     int64   `json:"room_id"`
	MessageIDs []int64 `json:"message_ids"`
	MessageID  string  `json:"msg_id"`
}

// GuardData is a typed LIVE_OPEN_PLATFORM_GUARD payload.
type GuardData struct {
	User                   UserInfo `json:"user_info"`
	GuardLevel             int64    `json:"guard_level"`
	GuardNumber            int64    `json:"guard_num"`
	GuardUnit              string   `json:"guard_unit"`
	Price                  int64    `json:"price"`
	FansMedalLevel         int64    `json:"fans_medal_level"`
	FansMedalName          string   `json:"fans_medal_name"`
	FansMedalWearingStatus bool     `json:"fans_medal_wearing_status"`
	RoomID                 int64    `json:"room_id"`
	MessageID              string   `json:"msg_id"`
	Timestamp              int64    `json:"timestamp"`
}

// LikeData is a typed LIVE_OPEN_PLATFORM_LIKE payload.
type LikeData struct {
	RoomID                 int64  `json:"room_id"`
	UID                    int64  `json:"uid"`
	OpenID                 string `json:"open_id"`
	UnionID                string `json:"union_id"`
	Name                   string `json:"uname"`
	FaceURL                string `json:"uface"`
	Timestamp              int64  `json:"timestamp"`
	Text                   string `json:"like_text"`
	Count                  int64  `json:"like_count"`
	MessageID              string `json:"msg_id"`
	FansMedalWearingStatus bool   `json:"fans_medal_wearing_status"`
	FansMedalName          string `json:"fans_medal_name"`
	FansMedalLevel         int64  `json:"fans_medal_level"`
}

// RoomEnterData is a typed LIVE_OPEN_PLATFORM_LIVE_ROOM_ENTER payload.
type RoomEnterData struct {
	RoomID    int64  `json:"room_id"`
	OpenID    string `json:"open_id"`
	UnionID   string `json:"union_id"`
	Name      string `json:"uname"`
	FaceURL   string `json:"uface"`
	Timestamp int64  `json:"timestamp"`
}

// LiveStatusData is shared by start and end notifications.
type LiveStatusData struct {
	RoomID    int64  `json:"room_id"`
	OpenID    string `json:"open_id"`
	UnionID   string `json:"union_id"`
	Timestamp int64  `json:"timestamp"`
	AreaName  string `json:"area_name"`
	Title     string `json:"title"`
}

// InteractionEndData indicates that a game ID will receive no more events.
type InteractionEndData struct {
	GameID    string `json:"game_id"`
	Timestamp int64  `json:"timestamp"`
}

var _ ProjectSessionWorkflow = (*ProjectSession)(nil)
