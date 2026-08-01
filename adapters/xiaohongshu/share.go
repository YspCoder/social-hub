package xiaohongshu

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

// ShareType is the media form accepted by xhs.share.
type ShareType string

const (
	ShareTypeNormal ShareType = "normal"
	ShareTypeVideo  ShareType = "video"
)

// ShareRequest contains only media fields still supported by the official SDK.
// Xiaohongshu no longer allows automatic title, copy, or topic prefill.
type ShareRequest struct {
	Type     ShareType
	Images   []string
	VideoURL string
	CoverURL string
}

// ShareInfo maps directly to the media-only xhs.share shareInfo object.
type ShareInfo struct {
	Type   ShareType `json:"type"`
	Images []string  `json:"images,omitempty"`
	Video  string    `json:"video,omitempty"`
	Cover  string    `json:"cover,omitempty"`
}

// VerifyConfig contains the short-lived server-generated JS SDK signature.
type VerifyConfig struct {
	AppKey    string `json:"appKey"`
	Nonce     string `json:"nonce"`
	Timestamp string `json:"timestamp"`
	Signature string `json:"signature"`
}

// SharePayload is returned to an approved application's web frontend.
type SharePayload struct {
	ShareInfo    ShareInfo    `json:"shareInfo"`
	VerifyConfig VerifyConfig `json:"verifyConfig"`
}

// ShareWorkflow prepares signed handoff data for the official client SDK.
type ShareWorkflow interface {
	Prepare(context.Context, ShareRequest) (*SharePayload, error)
}

// ShareService implements ShareWorkflow.
type ShareService struct{ client *Client }

func (s *ShareService) Prepare(ctx context.Context, input ShareRequest) (*SharePayload, error) {
	if !s.client.approved {
		return nil, approvalError("share_prepare")
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}
	token, err := s.client.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	nonce, err := newNonce()
	if err != nil {
		return nil, err
	}
	timestamp := strconv.FormatInt(s.client.clock.Now().UnixMilli(), 10)
	signature := buildSignature(s.client.appKey, nonce, timestamp, token.AccessToken)
	return &SharePayload{
		ShareInfo:    ShareInfo{Type: input.Type, Images: append([]string(nil), input.Images...), Video: input.VideoURL, Cover: input.CoverURL},
		VerifyConfig: VerifyConfig{AppKey: s.client.appKey, Nonce: nonce, Timestamp: timestamp, Signature: signature},
	}, nil
}

// Validate enforces the official media-only share shapes.
func (r ShareRequest) Validate() error {
	switch r.Type {
	case ShareTypeNormal:
		if len(r.Images) == 0 || r.VideoURL != "" || r.CoverURL != "" {
			return invalidArgument("share_validate", "normal shares require images and cannot contain video fields")
		}
		for _, value := range r.Images {
			if err := validateMediaURL(value); err != nil {
				return err
			}
		}
	case ShareTypeVideo:
		if len(r.Images) != 0 || r.VideoURL == "" {
			return invalidArgument("share_validate", "video shares require video_url and cannot contain images")
		}
		if err := validateMediaURL(r.VideoURL); err != nil {
			return err
		}
		if r.CoverURL != "" {
			if err := validateMediaURL(r.CoverURL); err != nil {
				return err
			}
		}
	default:
		return invalidArgument("share_validate", "type must be normal or video")
	}
	return nil
}

func validateMediaURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return invalidArgument("share_validate", "media URLs must be absolute HTTPS URLs without user information")
	}
	return nil
}

func (c *Client) accessToken(ctx context.Context) (socialhub.Token, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	now := c.clock.Now()
	if tokenUsable(c.token, now) {
		return c.token, nil
	}
	key := socialhub.TokenKey{Platform: "xiaohongshu", Product: "share-js", Account: string(c.accountID), Subject: c.appKey}
	if c.tokenStore != nil {
		stored, err := c.tokenStore.Get(ctx, key)
		if err == nil && tokenUsable(stored, now) {
			c.token = stored
			return stored, nil
		}
		if err != nil && !errors.Is(err, socialhub.ErrNotFound) {
			return socialhub.Token{}, wrapError("token_store_get", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		}
	}
	token, err := c.requestAccessToken(ctx)
	if err != nil {
		return socialhub.Token{}, err
	}
	if c.tokenStore != nil {
		if err := c.tokenStore.Put(ctx, key, token); err != nil {
			return socialhub.Token{}, wrapError("token_store_put", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		}
	}
	c.token = token
	return token, nil
}

func tokenUsable(token socialhub.Token, now time.Time) bool {
	return token.AccessToken != "" && (token.ExpiresAt.IsZero() || now.Add(time.Minute).Before(token.ExpiresAt))
}

func (c *Client) requestAccessToken(ctx context.Context) (socialhub.Token, error) {
	nonce, err := newNonce()
	if err != nil {
		return socialhub.Token{}, err
	}
	timestampMillis := c.clock.Now().UnixMilli()
	timestamp := strconv.FormatInt(timestampMillis, 10)
	body := struct {
		AppKey    string `json:"app_key"`
		Nonce     string `json:"nonce"`
		Timestamp int64  `json:"timestamp"`
		Signature string `json:"signature"`
	}{AppKey: c.appKey, Nonce: nonce, Timestamp: timestampMillis, Signature: buildSignature(c.appKey, nonce, timestamp, c.appSecret)}
	encoded, err := json.Marshal(body)
	if err != nil {
		return socialhub.Token{}, wrapError("access_token", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	endpoint := strings.TrimRight(c.baseURL, "/") + "/api/sns/v1/ext/access/token"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return socialhub.Token{}, wrapError("access_token", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return socialhub.Token{}, wrapError("access_token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil {
		return socialhub.Token{}, wrapError("access_token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if len(responseBody) > 1<<20 {
		return socialhub.Token{}, wrapError("access_token", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("response exceeded size limit"))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return socialhub.Token{}, decodeHTTPError(response.StatusCode, response.Header, responseBody)
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    int64  `json:"expires_in"`
		Code         int    `json:"code"`
		Message      string `json:"message"`
		ErrorCode    int    `json:"error_code"`
		ErrorMessage string `json:"error_msg"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return socialhub.Token{}, wrapError("access_token", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	platformCode := payload.Code
	if platformCode == 0 {
		platformCode = payload.ErrorCode
	}
	if platformCode != 0 {
		return socialhub.Token{}, platformError("access_token", platformCode, firstNonEmpty(payload.Message, payload.ErrorMessage), response.StatusCode, response.Header)
	}
	if payload.AccessToken == "" || payload.ExpiresIn <= 0 {
		return socialhub.Token{}, wrapError("access_token", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("missing access token fields"))
	}
	return socialhub.Token{AccessToken: payload.AccessToken, TokenType: "XHSShare", ExpiresAt: time.UnixMilli(payload.ExpiresIn)}, nil
}

func buildSignature(appKey, nonce, timestamp, secret string) string {
	value := "appKey=" + appKey + "&nonce=" + nonce + "&timeStamp=" + timestamp + secret
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func newNonce() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", wrapError("nonce", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	return hex.EncodeToString(value), nil
}

var _ ShareWorkflow = (*ShareService)(nil)
