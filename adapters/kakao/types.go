package kakao

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

// UserWorkflow exposes the currently authorized Kakao Login user.
type UserWorkflow interface {
	Me(context.Context, ...socialhub.CallOption) (*socialhub.User, error)
}

// FriendOrder controls friend-list sorting direction.
type FriendOrder string

const (
	FriendOrderAscending  FriendOrder = "asc"
	FriendOrderDescending FriendOrder = "desc"
)

// FriendSort controls whether favorites or nickname lead sorting.
type FriendSort string

const (
	FriendSortFavorite FriendSort = "favorite"
	FriendSortNickname FriendSort = "nickname"
)

type ListFriendsRequest struct {
	Offset int
	Limit  int
	Order  FriendOrder
	Sort   FriendSort
}

type Friend struct {
	ID           string `json:"id"`
	UUID         string `json:"uuid"`
	Nickname     string `json:"nickname,omitempty"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	Favorite     bool   `json:"favorite,omitempty"`
}

type FriendPage struct {
	Items         []Friend `json:"items"`
	TotalCount    int      `json:"total_count"`
	FavoriteCount int      `json:"favorite_count,omitempty"`
	NextOffset    *int     `json:"next_offset,omitempty"`
	HasMore       bool     `json:"has_more"`
}

type FriendWorkflow interface {
	ListFriends(context.Context, ListFriendsRequest, ...socialhub.CallOption) (FriendPage, error)
}

// MessageTarget selects the authorized user's My Chatroom or approved friends.
type MessageTarget string

const (
	MessageTargetSelf    MessageTarget = "self"
	MessageTargetFriends MessageTarget = "friends"
)

// Link is Kakao's platform-aware Product Link object.
type Link struct {
	WebURL                 string `json:"web_url,omitempty"`
	MobileWebURL           string `json:"mobile_web_url,omitempty"`
	AndroidExecutionParams string `json:"android_execution_params,omitempty"`
	IOSExecutionParams     string `json:"ios_execution_params,omitempty"`
}

// Button is one custom Kakao message-template action.
type Button struct {
	Title string `json:"title"`
	Link  Link   `json:"link"`
}

// TextTemplate is Kakao's default text template. Text is limited to 200
// Unicode code points and at least one Product Link field is required.
type TextTemplate struct {
	Text        string   `json:"text"`
	Link        Link     `json:"link"`
	ButtonTitle string   `json:"button_title,omitempty"`
	Buttons     []Button `json:"buttons,omitempty"`
}

type DefaultMessageRequest struct {
	Target        MessageTarget
	ReceiverUUIDs []string
	Template      TextTemplate
}

type CustomMessageRequest struct {
	Target        MessageTarget
	ReceiverUUIDs []string
	TemplateID    int64
	Arguments     map[string]string
}

type MessageFailure struct {
	Code          int      `json:"code"`
	Message       string   `json:"message"`
	ReceiverUUIDs []string `json:"receiver_uuids"`
}

type MessageResult struct {
	Target                  MessageTarget    `json:"target"`
	ResultCode              int              `json:"result_code"`
	SuccessfulReceiverUUIDs []string         `json:"successful_receiver_uuids,omitempty"`
	Failures                []MessageFailure `json:"failures,omitempty"`
}

type MessageWorkflow interface {
	SendDefault(context.Context, DefaultMessageRequest, ...socialhub.CallOption) (*MessageResult, error)
	SendCustom(context.Context, CustomMessageRequest, ...socialhub.CallOption) (*MessageResult, error)
}

func (template TextTemplate) validate() error {
	if strings.TrimSpace(template.Text) == "" || utf8.RuneCountInString(template.Text) > 200 || !utf8.ValidString(template.Text) {
		return invalidArgument("send_default_message", "text must be non-empty valid UTF-8 with at most 200 Unicode code points")
	}
	if err := template.Link.validate("send_default_message"); err != nil {
		return err
	}
	if template.ButtonTitle != "" && !validBoundedString(template.ButtonTitle, 1024) {
		return invalidArgument("send_default_message", "button title is invalid")
	}
	if len(template.Buttons) > 2 {
		return invalidArgument("send_default_message", "at most two buttons are allowed")
	}
	for _, button := range template.Buttons {
		if !validBoundedString(button.Title, 1024) {
			return invalidArgument("send_default_message", "button title is invalid")
		}
		if err := button.Link.validate("send_default_message"); err != nil {
			return err
		}
	}
	return nil
}

func (link Link) validate(operation string) error {
	if link.WebURL == "" && link.MobileWebURL == "" && link.AndroidExecutionParams == "" && link.IOSExecutionParams == "" {
		return invalidArgument(operation, "at least one Product Link field is required")
	}
	for _, value := range []string{link.WebURL, link.MobileWebURL} {
		if value != "" && !validHTTPURL(value) {
			return invalidArgument(operation, "web links must be absolute HTTP(S) URLs without credentials or fragments")
		}
	}
	for _, value := range []string{link.AndroidExecutionParams, link.IOSExecutionParams} {
		if value != "" && !validBoundedString(value, 4096) {
			return invalidArgument(operation, "app execution parameters are invalid")
		}
	}
	return nil
}

func validServiceUserID(value string) bool {
	if value == "" || strings.TrimSpace(value) != value {
		return false
	}
	id, err := strconv.ParseInt(value, 10, 64)
	return err == nil && id > 0
}

func validBoundedString(value string, maximum int) bool {
	if value == "" || strings.TrimSpace(value) != value || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func validOptionalString(value string, maximum int) bool {
	if len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func validHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

var _ UserWorkflow = (*Client)(nil)
var _ FriendWorkflow = (*Client)(nil)
var _ MessageWorkflow = (*Client)(nil)
