package wechatminiprogram

import (
	"context"
	"time"

	"social-hub/pkg/socialhub"
)

// Session is the sensitive login state returned by code2Session. SessionKey
// must remain on the developer's server and must not be used as an application session ID.
type Session struct {
	OpenID     string
	UnionID    string
	SessionKey string
}

func (Session) String() string   { return "wechatminiprogram.Session{REDACTED}" }
func (Session) GoString() string { return "wechatminiprogram.Session{REDACTED}" }

// StableAccessToken is a sensitive server credential and its local expiry.
type StableAccessToken struct {
	Value     string
	ExpiresAt time.Time
	ExpiresIn time.Duration
}

func (StableAccessToken) String() string   { return "wechatminiprogram.StableAccessToken{REDACTED}" }
func (StableAccessToken) GoString() string { return "wechatminiprogram.StableAccessToken{REDACTED}" }

type MiniProgramState string

const (
	StateDeveloper MiniProgramState = "developer"
	StateTrial     MiniProgramState = "trial"
	StateFormal    MiniProgramState = "formal"
)

type Language string

const (
	LanguageSimplifiedChinese   Language = "zh_CN"
	LanguageEnglish             Language = "en_US"
	LanguageTraditionalHongKong Language = "zh_HK"
	LanguageTraditionalTaiwan   Language = "zh_TW"
)

// TemplateValue is one typed template keyword value.
type TemplateValue struct {
	Value string `json:"value"`
}

func (TemplateValue) String() string   { return "wechatminiprogram.TemplateValue{REDACTED}" }
func (TemplateValue) GoString() string { return "wechatminiprogram.TemplateValue{REDACTED}" }

// SubscriptionMessage is a user-authorized Mini Program subscription message.
// ToUser and Data may contain personal information and are redacted by fmt.
type SubscriptionMessage struct {
	ToUser           string                   `json:"touser"`
	TemplateID       string                   `json:"template_id"`
	Page             string                   `json:"page,omitempty"`
	Data             map[string]TemplateValue `json:"data"`
	MiniProgramState MiniProgramState         `json:"miniprogram_state"`
	Language         Language                 `json:"lang"`
}

func (SubscriptionMessage) String() string { return "wechatminiprogram.SubscriptionMessage{REDACTED}" }
func (SubscriptionMessage) GoString() string {
	return "wechatminiprogram.SubscriptionMessage{REDACTED}"
}

// PhoneNumberRequest consumes one short-lived phone-number code. Code and
// OpenID are sensitive and the code can only be used once.
type PhoneNumberRequest struct {
	Code   string `json:"code"`
	OpenID string `json:"openid,omitempty"`
}

func (PhoneNumberRequest) String() string   { return "wechatminiprogram.PhoneNumberRequest{REDACTED}" }
func (PhoneNumberRequest) GoString() string { return "wechatminiprogram.PhoneNumberRequest{REDACTED}" }

type PhoneWatermark struct {
	Timestamp int64  `json:"timestamp"`
	AppID     string `json:"appid"`
}

func (PhoneWatermark) String() string   { return "wechatminiprogram.PhoneWatermark{REDACTED}" }
func (PhoneWatermark) GoString() string { return "wechatminiprogram.PhoneWatermark{REDACTED}" }

// PhoneInfo contains personal information. The adapter never places it in an
// error, log, or Raw field, but callers remain responsible for secure handling.
type PhoneInfo struct {
	PhoneNumber     string         `json:"phoneNumber"`
	PurePhoneNumber string         `json:"purePhoneNumber"`
	CountryCode     string         `json:"countryCode"`
	Watermark       PhoneWatermark `json:"watermark"`
}

func (PhoneInfo) String() string   { return "wechatminiprogram.PhoneInfo{REDACTED}" }
func (PhoneInfo) GoString() string { return "wechatminiprogram.PhoneInfo{REDACTED}" }

type LoginWorkflow interface {
	Code2Session(context.Context, string, ...socialhub.CallOption) (*Session, error)
}

type CredentialsWorkflow interface {
	GetStableAccessToken(context.Context, ...socialhub.CallOption) (*StableAccessToken, error)
	ForceRefreshStableAccessToken(context.Context, ...socialhub.CallOption) (*StableAccessToken, error)
}

type SubscriptionWorkflow interface {
	Send(context.Context, SubscriptionMessage, ...socialhub.CallOption) error
}

type PhoneNumberWorkflow interface {
	Exchange(context.Context, PhoneNumberRequest, ...socialhub.CallOption) (*PhoneInfo, error)
}

var (
	_ LoginWorkflow        = (*Client)(nil)
	_ CredentialsWorkflow  = (*Client)(nil)
	_ SubscriptionWorkflow = (*Client)(nil)
	_ PhoneNumberWorkflow  = (*Client)(nil)
)
