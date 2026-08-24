package steam

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

const maxProviderObjectBytes = 8 << 20

// SteamID preserves Steam's 64-bit decimal identity exactly as text.
type SteamID string

// ResponseMeta contains response metadata with a documented retry meaning.
type ResponseMeta struct {
	StatusCode         int
	RetryAfter         string
	RetryAfterDuration time.Duration
}

type GetPlayerSummariesRequest struct {
	SteamIDs []SteamID
}

type PlayerSummariesResponse struct {
	Players []Player        `json:"players"`
	Meta    ResponseMeta    `json:"-"`
	Raw     json.RawMessage `json:"-"`
}

type Player struct {
	SteamID                  SteamID         `json:"steamid"`
	CommunityVisibilityState int             `json:"communityvisibilitystate"`
	ProfileState             *int            `json:"profilestate"`
	PersonaName              string          `json:"personaname"`
	LastLogoff               *int64          `json:"lastlogoff"`
	ProfileURL               string          `json:"profileurl"`
	Avatar                   string          `json:"avatar"`
	AvatarMedium             string          `json:"avatarmedium"`
	AvatarFull               string          `json:"avatarfull"`
	Raw                      json.RawMessage `json:"-"`
}

func (value *Player) UnmarshalJSON(data []byte) error {
	type wire Player
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Player(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type GetNewsForAppRequest struct {
	AppID     uint32
	MaxLength uint32
	EndDate   *time.Time
	Count     uint32
	Feeds     []string
	Tags      []string
}

type AppNewsResponse struct {
	AppID     uint32          `json:"appid"`
	NewsItems []NewsItem      `json:"newsitems"`
	Count     int             `json:"count"`
	Meta      ResponseMeta    `json:"-"`
	Raw       json.RawMessage `json:"-"`
}

type NewsItem struct {
	GID           string          `json:"gid"`
	Title         string          `json:"title"`
	URL           string          `json:"url"`
	IsExternalURL bool            `json:"is_external_url"`
	Author        string          `json:"author"`
	Contents      string          `json:"contents"`
	FeedLabel     string          `json:"feedlabel"`
	Date          int64           `json:"date"`
	FeedName      string          `json:"feedname"`
	FeedType      int             `json:"feed_type"`
	AppID         uint32          `json:"appid"`
	Tags          []string        `json:"tags"`
	Raw           json.RawMessage `json:"-"`
}

func (value *NewsItem) UnmarshalJSON(data []byte) error {
	type wire NewsItem
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = NewsItem(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// ReadWorkflow is the verified read-only Steam Web API surface.
type ReadWorkflow interface {
	GetPlayerSummaries(context.Context, GetPlayerSummariesRequest, ...socialhub.CallOption) (PlayerSummariesResponse, error)
	GetNewsForApp(context.Context, GetNewsForAppRequest, ...socialhub.CallOption) (AppNewsResponse, error)
}

var _ ReadWorkflow = (*Client)(nil)
