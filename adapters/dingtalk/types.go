package dingtalk

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

// UserDetail preserves the contact fields returned by DingTalk.
type UserDetail struct {
	AvatarURL  string `json:"avatarUrl"`
	Email      string `json:"email"`
	LoginEmail string `json:"loginEmail"`
	Mobile     string `json:"mobile"`
	Nick       string `json:"nick"`
	OpenID     string `json:"openId"`
	StateCode  string `json:"stateCode"`
	UnionID    string `json:"unionId"`
	Visitor    bool   `json:"visitor"`
}

// RobotMessage is DingTalk's msgKey plus JSON-object msgParam pair.
type RobotMessage struct {
	Key   string
	Param json.RawMessage
}

// GroupMessageRequest sends one message to an open conversation.
type GroupMessageRequest struct {
	OpenConversationID string
	Message            RobotMessage
}

// BatchOTORequest sends one message to at most 100 staff IDs.
type BatchOTORequest struct {
	UserIDs []string
	Message RobotMessage
}

// RobotSendResult preserves asynchronous and partial-delivery diagnostics.
type RobotSendResult struct {
	ProcessQueryKey           string   `json:"processQueryKey"`
	FilteredStaffIDList       []string `json:"filteredStaffIdList,omitempty"`
	FlowControlledStaffIDList []string `json:"flowControlledStaffIdList,omitempty"`
	InvalidStaffIDList        []string `json:"invalidStaffIdList,omitempty"`
}

// ContactWorkflow exposes UnionID-based contact reads.
type ContactWorkflow interface {
	GetUserByUnionID(context.Context, string, ...socialhub.CallOption) (*UserDetail, error)
}

// RobotWorkflow exposes application bot sends without claiming the common
// Messenger contract, which also requires arbitrary message lookup.
type RobotWorkflow interface {
	SendGroupMessage(context.Context, GroupMessageRequest, ...socialhub.CallOption) (*RobotSendResult, error)
	BatchSendOTO(context.Context, BatchOTORequest, ...socialhub.CallOption) (*RobotSendResult, error)
}

// AuthWorkflow exposes explicit refresh for dynamically managed app tokens.
type AuthWorkflow interface {
	RefreshAppToken(context.Context) (socialhub.Token, error)
}

func validOpaque(value string, maximum int) bool {
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

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}

func jsonExtension(key string, value any) map[string]json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return map[string]json.RawMessage{key: encoded}
}
