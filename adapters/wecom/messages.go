package wecom

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

type applicationMessagePayload struct {
	ToUser                 string         `json:"touser,omitempty"`
	ToParty                string         `json:"toparty,omitempty"`
	ToTag                  string         `json:"totag,omitempty"`
	MessageType            string         `json:"msgtype"`
	AgentID                int64          `json:"agentid"`
	Text                   *textBlock     `json:"text,omitempty"`
	Markdown               *markdownBlock `json:"markdown,omitempty"`
	Image                  *mediaBlock    `json:"image,omitempty"`
	Voice                  *mediaBlock    `json:"voice,omitempty"`
	Video                  *videoBlock    `json:"video,omitempty"`
	File                   *mediaBlock    `json:"file,omitempty"`
	Safe                   int            `json:"safe,omitempty"`
	EnableIDTranslation    int            `json:"enable_id_trans,omitempty"`
	EnableDuplicateCheck   int            `json:"enable_duplicate_check,omitempty"`
	DuplicateCheckInterval int64          `json:"duplicate_check_interval,omitempty"`
}

type textBlock struct {
	Content string `json:"content"`
}

type markdownBlock struct {
	Content string `json:"content"`
}

type mediaBlock struct {
	MediaID string `json:"media_id"`
}

type videoBlock struct {
	MediaID     string `json:"media_id"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

type applicationMessageResponse struct {
	APIResponse
	InvalidUser    string `json:"invaliduser"`
	InvalidParty   string `json:"invalidparty"`
	InvalidTag     string `json:"invalidtag"`
	UnlicensedUser string `json:"unlicenseduser"`
	MessageID      string `json:"msgid"`
}

func (c *Client) SendApplicationMessage(ctx context.Context, input ApplicationMessageRequest, options ...socialhub.CallOption) (*SendResult, error) {
	recipients, err := c.resolveRecipients(input.Recipients)
	if err != nil {
		return nil, err
	}
	if input.Content == nil {
		return nil, invalidArgument("send_application_message", "message content is required")
	}
	if input.DuplicateCheckInterval < 0 || input.DuplicateCheckInterval > 4*time.Hour || input.DuplicateCheckInterval%time.Second != 0 {
		return nil, invalidArgument("send_application_message", "duplicate check interval must be whole seconds up to four hours")
	}
	if !input.EnableDuplicateCheck && input.DuplicateCheckInterval != 0 {
		return nil, invalidArgument("send_application_message", "duplicate check interval requires duplicate checking")
	}
	payload := applicationMessagePayload{
		ToUser: strings.Join(recipients.UserIDs, "|"), ToParty: joinInt64(recipients.PartyIDs),
		ToTag: joinInt64(recipients.TagIDs), AgentID: c.agentID,
	}
	if err := input.Content.apply(&payload); err != nil {
		return nil, err
	}
	if input.Safe {
		payload.Safe = 1
	}
	if input.EnableIDTranslation {
		payload.EnableIDTranslation = 1
	}
	if input.EnableDuplicateCheck {
		payload.EnableDuplicateCheck = 1
		payload.DuplicateCheckInterval = int64(input.DuplicateCheckInterval / time.Second)
	}
	var response applicationMessageResponse
	if err := c.api.JSON(ctx, http.MethodPost, "/cgi-bin/message/send", nil, payload, &response, options...); err != nil {
		return nil, err
	}
	if err := c.responseError(ctx, "send_application_message", response.APIResponse); err != nil {
		return nil, err
	}
	return &SendResult{
		MessageID: response.MessageID, Recipients: recipients,
		InvalidUserIDs:    splitRecipientResponse(response.InvalidUser),
		InvalidPartyIDs:   splitRecipientResponse(response.InvalidParty),
		InvalidTagIDs:     splitRecipientResponse(response.InvalidTag),
		UnlicensedUserIDs: splitRecipientResponse(response.UnlicensedUser),
	}, nil
}

func (c *Client) SendMessage(ctx context.Context, input socialhub.SendMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	if input.ReplyToID != nil {
		return nil, unsupported("send_message", "WeCom application messages do not implement the common reply contract")
	}
	if len(input.MediaIDs) != 0 {
		return nil, unsupported("send_message", "use ApplicationMessages with typed image, voice, video, or file content")
	}
	if input.Text == nil || *input.Text == "" {
		return nil, invalidArgument("send_message", "text is required")
	}
	userIDs := append([]string(nil), input.RecipientIDs...)
	if len(userIDs) == 0 && input.ConversationID != "" {
		userIDs = []string{input.ConversationID}
	}
	result, err := c.SendApplicationMessage(ctx, ApplicationMessageRequest{
		Recipients: RecipientSet{UserIDs: userIDs}, Content: TextContent{Content: *input.Text},
	}, options...)
	if err != nil {
		return nil, err
	}
	encoded, _ := json.Marshal(result)
	extensions := map[string]json.RawMessage{"wecom.delivery": encoded}
	return &socialhub.Message{
		Platform: "wecom", AccountID: c.accountID, ID: result.MessageID,
		ConversationID: strings.Join(result.Recipients.UserIDs, "|"),
		RecipientIDs:   append([]string(nil), result.Recipients.UserIDs...), Text: input.Text,
		Direction: socialhub.DirectionOutbound, Extensions: extensions,
	}, nil
}

func (c *Client) GetMessage(context.Context, string, ...socialhub.CallOption) (*socialhub.Message, error) {
	return nil, unsupported("get_message", "WeCom does not expose sent application messages through this API")
}

func (c *Client) resolveRecipients(input RecipientSet) (RecipientSet, error) {
	resolved := RecipientSet{
		UserIDs: uniqueStrings(input.UserIDs), PartyIDs: uniqueInt64(input.PartyIDs),
		TagIDs: uniqueInt64(input.TagIDs), ToAll: input.ToAll,
	}
	if resolved.ToAll {
		if err := validateRecipientSet(resolved, false); err != nil {
			return RecipientSet{}, err
		}
		resolved.UserIDs = []string{"@all"}
		return resolved, nil
	}
	if len(resolved.UserIDs) == 0 && len(resolved.PartyIDs) == 0 && len(resolved.TagIDs) == 0 {
		resolved = RecipientSet{
			UserIDs: uniqueStrings(c.defaults.UserIDs), PartyIDs: uniqueInt64(c.defaults.PartyIDs),
			TagIDs: uniqueInt64(c.defaults.TagIDs),
		}
	}
	if err := validateRecipientSet(resolved, false); err != nil {
		return RecipientSet{}, err
	}
	return resolved, nil
}

func validateRecipientSet(recipients RecipientSet, allowEmpty bool) error {
	if recipients.ToAll && (len(recipients.UserIDs) != 0 || len(recipients.PartyIDs) != 0 || len(recipients.TagIDs) != 0) {
		return invalidArgument("recipients", "ToAll cannot be combined with explicit recipients")
	}
	if len(recipients.UserIDs) > 1000 || len(recipients.PartyIDs) > 100 || len(recipients.TagIDs) > 100 {
		return invalidArgument("recipients", "recipient count exceeds WeCom limits")
	}
	if !allowEmpty && !recipients.ToAll && len(recipients.UserIDs) == 0 && len(recipients.PartyIDs) == 0 && len(recipients.TagIDs) == 0 {
		return invalidArgument("recipients", "at least one member, department, or tag is required")
	}
	for _, userID := range recipients.UserIDs {
		if userID == "@all" {
			if len(recipients.UserIDs) != 1 || len(recipients.PartyIDs) != 0 || len(recipients.TagIDs) != 0 {
				return invalidArgument("recipients", "@all must be the only recipient")
			}
			continue
		}
		if !validBoundedValue(userID, 64, false) {
			return invalidArgument("recipients", "member IDs must contain 1 to 64 bytes without separators")
		}
	}
	for _, value := range append(append([]int64(nil), recipients.PartyIDs...), recipients.TagIDs...) {
		if value <= 0 {
			return invalidArgument("recipients", "department and tag IDs must be positive")
		}
	}
	return nil
}

func uniqueStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !slices.Contains(result, value) {
			result = append(result, value)
		}
	}
	return result
}

func uniqueInt64(values []int64) []int64 {
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if !slices.Contains(result, value) {
			result = append(result, value)
		}
	}
	return result
}

func joinInt64(values []int64) string {
	stringsValues := make([]string, len(values))
	for index, value := range values {
		stringsValues[index] = strconv.FormatInt(value, 10)
	}
	return strings.Join(stringsValues, "|")
}

func splitRecipientResponse(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(character rune) bool { return character == '|' || character == ',' })
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
