package baiduads

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// OAuthClient implements Baidu Marketing's browser authorization, auth-code
// exchange, and refresh-token operations.
type OAuthClient struct {
	AppID      string
	SecretKey  string
	BaseURL    string
	HTTPClient *http.Client
	Clock      socialhub.Clock
}

func (client *OAuthClient) AuthorizationURL(input AuthorizationRequest) (string, error) {
	const operation = "oauth_authorize"
	if !validOpaque(client.AppID, 128) || !validEndpoint(client.BaseURL) || !validOpaque(input.Scope, 4096) ||
		!validOpaque(input.State, 512) {
		return "", invalidArgument(operation, "OAuth app, scope, state, or endpoint is invalid")
	}
	if err := validateCallback(input.Callback); err != nil {
		return "", err
	}
	parsed, _ := url.Parse(strings.TrimRight(client.BaseURL, "/") + "/oauth/page/index")
	query := parsed.Query()
	query.Set("platformId", platformID)
	query.Set("appId", client.AppID)
	query.Set("scope", input.Scope)
	query.Set("state", input.State)
	query.Set("callback", input.Callback)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (client *OAuthClient) Exchange(ctx context.Context, userID int64, authCode string) (OAuthToken, error) {
	const operation = "oauth_exchange"
	if userID <= 0 || !validOpaque(authCode, 16384) {
		return OAuthToken{}, invalidArgument(operation, "user ID and authorization code are required")
	}
	return client.token(ctx, operation, "/oauth/accessToken", map[string]any{
		"appId": client.AppID, "authCode": authCode, "secretKey": client.SecretKey,
		"grantType": "auth_code", "userId": userID,
	}, authCode, "")
}

func (client *OAuthClient) Refresh(ctx context.Context, userID int64, refreshToken string) (OAuthToken, error) {
	const operation = "oauth_refresh"
	if userID <= 0 || !validOpaque(refreshToken, 16384) {
		return OAuthToken{}, invalidArgument(operation, "user ID and refresh token are required")
	}
	return client.token(ctx, operation, "/oauth/refreshToken", map[string]any{
		"appId": client.AppID, "refreshToken": refreshToken, "secretKey": client.SecretKey, "userId": userID,
	}, refreshToken, refreshToken)
}

func (client *OAuthClient) token(ctx context.Context, operation, path string, input map[string]any, sensitive, existingRefreshToken string) (OAuthToken, error) {
	if !validOpaque(client.AppID, 128) || !validOpaque(client.SecretKey, 16384) || !validEndpoint(client.BaseURL) ||
		client.HTTPClient == nil || client.Clock == nil {
		return OAuthToken{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return OAuthToken{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(client.BaseURL, "/")+path, bytes.NewReader(encoded))
	if err != nil {
		return OAuthToken{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json;charset=UTF-8")
	request.Header.Set("Content-Type", "application/json;charset=UTF-8")
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return OAuthToken{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return OAuthToken{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return OAuthToken{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("OAuth response exceeded size limit"))
	}
	var payload struct {
		Code    *int64 `json:"code"`
		Message string `json:"message"`
		Data    *struct {
			AccessToken        string            `json:"accessToken"`
			RefreshToken       string            `json:"refreshToken"`
			ExpiresTime        string            `json:"expiresTime"`
			RefreshExpiresTime string            `json:"refreshExpiresTime"`
			ExpiresIn          int64             `json:"expiresIn"`
			RefreshExpiresIn   int64             `json:"refreshExpiresIn"`
			Scope              map[string]string `json:"scope"`
			OpenID             string            `json:"openId"`
			UserID             int64             `json:"userId"`
		} `json:"data"`
	}
	decodeErr := json.Unmarshal(body, &payload)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if decodeErr == nil && payload.Code != nil {
			return OAuthToken{}, oauthError(operation, response.StatusCode, response.Header, *payload.Code,
				redactKnown(payload.Message, client.SecretKey, sensitive))
		}
		return OAuthToken{}, withOperation(decodeHTTPError(response.StatusCode, response.Header, body), operation)
	}
	if decodeErr != nil || payload.Code == nil {
		if decodeErr == nil {
			decodeErr = fmt.Errorf("OAuth response omitted code")
		}
		return OAuthToken{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, decodeErr)
	}
	if *payload.Code != 0 {
		return OAuthToken{}, oauthError(operation, response.StatusCode, response.Header, *payload.Code,
			redactKnown(payload.Message, client.SecretKey, sensitive))
	}
	if payload.Data == nil {
		return OAuthToken{}, platformContractError(operation, "Baidu OAuth success response omitted data")
	}
	data := payload.Data
	expectedUserID, _ := input["userId"].(int64)
	if data.UserID == 0 && operation == "oauth_refresh" {
		data.UserID = expectedUserID
	}
	refreshToken := data.RefreshToken
	if refreshToken == "" {
		refreshToken = existingRefreshToken
	}
	if !validOpaque(data.AccessToken, 16384) || !validOpaque(refreshToken, 16384) || data.UserID <= 0 || data.UserID != expectedUserID ||
		data.ExpiresIn <= 0 || data.ExpiresIn > int64((365*24*time.Hour)/time.Second) ||
		data.RefreshExpiresIn <= 0 || data.RefreshExpiresIn > int64((10*365*24*time.Hour)/time.Second) {
		return OAuthToken{}, platformContractError(operation, "Baidu OAuth returned invalid token fields")
	}
	scopes := make([]string, 0, len(data.Scope))
	for key := range data.Scope {
		if !validOpaque(key, 256) {
			return OAuthToken{}, platformContractError(operation, "Baidu OAuth returned an invalid scope identifier")
		}
		scopes = append(scopes, key)
	}
	sort.Strings(scopes)
	now := client.Clock.Now()
	return OAuthToken{
		Token: socialhub.Token{
			AccessToken: data.AccessToken, RefreshToken: refreshToken, TokenType: "BaiduAds",
			ExpiresAt: now.Add(time.Duration(data.ExpiresIn) * time.Second), Scopes: scopes,
		},
		OpenID: data.OpenID, UserID: data.UserID, Scope: data.Scope, ExpiresTime: data.ExpiresTime,
		RefreshExpiresAt: now.Add(time.Duration(data.RefreshExpiresIn) * time.Second), RefreshExpiresTime: data.RefreshExpiresTime,
	}, nil
}

func oauthError(operation string, status int, header http.Header, platformCode int64, message string) error {
	code, class := socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	if status < 200 || status >= 300 {
		code, class = classifyHTTPError(status)
	} else if platformCode == 600011 {
		code, class = socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation, HTTPStatus: status,
		PlatformCode: strconv.FormatInt(platformCode, 10), PlatformMessage: boundedMessage(message, 512),
		RequestID: boundedMessage(header.Get("X-B3-Traceid"), 256), RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func redactKnown(value string, secrets ...string) string {
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return value
}
