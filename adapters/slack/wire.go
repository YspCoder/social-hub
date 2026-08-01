package slack

import "encoding/json"

type wireUserProfile struct {
	DisplayName string `json:"display_name"`
	RealName    string `json:"real_name"`
	Image192    string `json:"image_192"`
}

type wireUser struct {
	ID        string          `json:"id"`
	TeamID    string          `json:"team_id"`
	Name      string          `json:"name"`
	RealName  string          `json:"real_name"`
	Deleted   bool            `json:"deleted"`
	IsBot     bool            `json:"is_bot"`
	IsAppUser bool            `json:"is_app_user"`
	Profile   wireUserProfile `json:"profile"`
}

type wireReaction struct {
	Name  string   `json:"name"`
	Count int      `json:"count"`
	Users []string `json:"users"`
}

type wireFile struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Title      string `json:"title"`
	Mimetype   string `json:"mimetype"`
	Filetype   string `json:"filetype"`
	Size       int64  `json:"size"`
	URLPrivate string `json:"url_private"`
	Thumb360   string `json:"thumb_360"`
	Width      int    `json:"original_w"`
	Height     int    `json:"original_h"`
	DurationMS int64  `json:"duration_ms"`
	Mode       string `json:"mode"`
}

type wireMessage struct {
	Type       string         `json:"type"`
	Subtype    string         `json:"subtype"`
	User       string         `json:"user"`
	BotID      string         `json:"bot_id"`
	Text       string         `json:"text"`
	TS         string         `json:"ts"`
	ThreadTS   string         `json:"thread_ts"`
	DeletedTS  string         `json:"deleted_ts"`
	ReplyCount int            `json:"reply_count"`
	Files      []wireFile     `json:"files"`
	Reactions  []wireReaction `json:"reactions"`
}

type eventsEnvelope struct {
	Type              string          `json:"type"`
	Token             string          `json:"token"`
	Challenge         string          `json:"challenge"`
	TeamID            string          `json:"team_id"`
	APIAppID          string          `json:"api_app_id"`
	EventID           string          `json:"event_id"`
	EventTime         int64           `json:"event_time"`
	EventContext      string          `json:"event_context"`
	Event             json.RawMessage `json:"event"`
	MinuteRateLimited int64           `json:"minute_rate_limited"`
}

type wireReactionEvent struct {
	Type     string `json:"type"`
	User     string `json:"user"`
	Reaction string `json:"reaction"`
	ItemUser string `json:"item_user"`
	EventTS  string `json:"event_ts"`
	Item     struct {
		Type    string `json:"type"`
		Channel string `json:"channel"`
		TS      string `json:"ts"`
	} `json:"item"`
}

type wireFileEvent struct {
	Type      string `json:"type"`
	FileID    string `json:"file_id"`
	UserID    string `json:"user_id"`
	ChannelID string `json:"channel_id"`
	EventTS   string `json:"event_ts"`
}
