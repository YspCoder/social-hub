package officialaccount

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"social-hub/extensions/material"
	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

// Client implements the supported Official Account capability interfaces.
type Client struct {
	accountID    socialhub.AccountID
	appID        string
	transport    *transport.Client
	webhookToken string
	aesKey       string
	clock        socialhub.Clock

	uploadMu sync.Mutex
	uploads  map[string]*uploadState
	assets   map[string]*materialAsset

	materials *MaterialService
	drafts    *DraftService
}

func (c *Client) Platform() socialhub.Platform { return "wechat" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	webhookSupported := c.webhookToken != ""
	webhookReason := ""
	if !webhookSupported {
		webhookReason = "webhook.token_ref is not configured"
	}
	return socialhub.Capabilities{
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "use the typed DraftService for multi-article drafts and publication", DocURL: "https://developers.weixin.qq.com/doc/offiaccount/Draft_Box/Add_draft.html"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: true, Approval: socialhub.ApprovalUnknown, Scopes: []string{"user.info"}, Reason: "initial common fetcher supports follower profiles only", DocURL: "https://developers.weixin.qq.com/doc/offiaccount/User_Management/Get_users_basic_information_UnionID.html"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "common uploader creates temporary media; MaterialService also supports permanent material", DocURL: "https://developers.weixin.qq.com/doc/offiaccount/Asset_Management/New_temporary_materials.html"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "customer-service message window and account rules still apply", DocURL: "https://developers.weixin.qq.com/doc/offiaccount/Message_Management/Service_Center_messages.html"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: webhookSupported, Approval: socialhub.ApprovalUnknown, Reason: webhookReason, DocURL: "https://developers.weixin.qq.com/doc/offiaccount/Basic_Information/Access_Overview.html"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Official Account API does not expose a common like/comment mutation contract"},
	}, nil
}

func (c *Client) Publisher() (socialhub.Publisher, bool) { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)     { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool) {
	return c, true
}
func (c *Client) Reactor() (socialhub.Reactor, bool)     { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool) { return c, true }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	if c.webhookToken == "" {
		return nil, false
	}
	return c, true
}
func (c *Client) Close() error { return nil }

// MaterialManager returns WeChat temporary and permanent material operations.
func (c *Client) MaterialManager() material.Manager { return c.materials }

// Drafts returns the typed multi-article draft and publication service.
func (c *Client) Drafts() *DraftService { return c.drafts }

func (c *Client) GetUser(ctx context.Context, openID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if openID == "" {
		return nil, invalidArgument("get_user", "openid is required")
	}
	query := url.Values{"openid": {openID}, "lang": {"zh_CN"}}
	var response userResponse
	if err := c.transport.JSON(ctx, http.MethodGet, "/cgi-bin/user/info", query, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := response.APIResponse.Err("get_user"); err != nil {
		return nil, err
	}
	extensions, _ := json.Marshal(map[string]any{
		"unionid":         response.UnionID,
		"subscribe":       response.Subscribe,
		"subscribe_time":  response.SubscribeTime,
		"subscribe_scene": response.SubscribeScene,
		"remark":          response.Remark,
		"group_id":        response.GroupID,
		"tag_ids":         response.TagIDs,
	})
	accountType := "official_account_follower"
	return &socialhub.User{
		Platform:    "wechat",
		AccountID:   c.accountID,
		ID:          response.OpenID,
		DisplayName: stringPointer(response.Nickname),
		AvatarURL:   stringPointer(response.HeadImageURL),
		AccountType: &accountType,
		Extensions:  map[string]json.RawMessage{"wechat.official_account": extensions},
	}, nil
}

func (c *Client) GetPost(context.Context, string, ...socialhub.CallOption) (*socialhub.Post, error) {
	return nil, socialhub.UnsupportedError("wechat", socialhub.CapFetch)
}

func (c *Client) ListPosts(context.Context, socialhub.ListPostsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	return socialhub.Page[socialhub.Post]{}, socialhub.UnsupportedError("wechat", socialhub.CapFetch)
}

func (c *Client) ListComments(context.Context, socialhub.ListCommentsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	return socialhub.Page[socialhub.Comment]{}, socialhub.UnsupportedError("wechat", socialhub.CapFetch)
}

func (c *Client) SendMessage(ctx context.Context, input socialhub.SendMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	if len(input.RecipientIDs) != 1 || input.RecipientIDs[0] == "" {
		return nil, invalidArgument("send_message", "exactly one recipient openid is required")
	}
	if input.ReplyToID != nil {
		return nil, socialhub.UnsupportedError("wechat", socialhub.CapMessage)
	}
	recipient := input.RecipientIDs[0]
	request := customerServiceRequest{ToUser: recipient}
	switch {
	case input.Text != nil && *input.Text != "" && len(input.MediaIDs) == 0:
		request.MessageType = "text"
		request.Text = &textMessage{Content: *input.Text}
	case input.Text == nil && len(input.MediaIDs) == 1:
		request.MessageType = "image"
		request.Image = &mediaMessage{MediaID: input.MediaIDs[0]}
	default:
		return nil, invalidArgument("send_message", "provide either text or one image media ID")
	}
	var response customerServiceResponse
	if err := c.transport.JSON(ctx, http.MethodPost, "/cgi-bin/message/custom/send", nil, request, &response, options...); err != nil {
		return nil, err
	}
	if err := response.APIResponse.Err("send_message"); err != nil {
		return nil, err
	}
	messageID := ""
	if response.MessageID != 0 {
		messageID = strconv.FormatInt(response.MessageID, 10)
	}
	return &socialhub.Message{Platform: "wechat", AccountID: c.accountID, ID: messageID, ConversationID: recipient, RecipientIDs: []string{recipient}, Text: input.Text, Media: mediaReferences(input.MediaIDs), Direction: socialhub.DirectionOutbound}, nil
}

func (c *Client) GetMessage(context.Context, string, ...socialhub.CallOption) (*socialhub.Message, error) {
	return nil, socialhub.UnsupportedError("wechat", socialhub.CapMessage)
}

type userResponse struct {
	APIResponse
	Subscribe      int     `json:"subscribe"`
	OpenID         string  `json:"openid"`
	Nickname       string  `json:"nickname"`
	HeadImageURL   string  `json:"headimgurl"`
	SubscribeTime  int64   `json:"subscribe_time"`
	UnionID        string  `json:"unionid"`
	Remark         string  `json:"remark"`
	GroupID        int64   `json:"groupid"`
	TagIDs         []int64 `json:"tagid_list"`
	SubscribeScene string  `json:"subscribe_scene"`
}

type customerServiceRequest struct {
	ToUser      string        `json:"touser"`
	MessageType string        `json:"msgtype"`
	Text        *textMessage  `json:"text,omitempty"`
	Image       *mediaMessage `json:"image,omitempty"`
}

type textMessage struct {
	Content string `json:"content"`
}

type mediaMessage struct {
	MediaID string `json:"media_id"`
}

type customerServiceResponse struct {
	APIResponse
	MessageID int64 `json:"msgid"`
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}

func mediaReferences(ids []string) []socialhub.Media {
	media := make([]socialhub.Media, 0, len(ids))
	for _, id := range ids {
		media = append(media, socialhub.Media{ID: id, State: socialhub.MediaStateReady})
	}
	return media
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.MediaUploader = (*Client)(nil)
var _ socialhub.Messenger = (*Client)(nil)
var _ material.Provider = (*Client)(nil)
