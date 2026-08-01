package imgur

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// OAuthClient implements Imgur's current implicit authorization URL and refresh grant.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
}

// AuthorizationURL returns the official response_type=token flow URL.
func (client *OAuthClient) AuthorizationURL(state string) (string, error) {
	if !validOpaque(client.ClientID, 512) || !validOpaque(state, 1024) || !validEndpoint(client.AuthURL) {
		return "", invalidArgument("oauth_authorize", "client ID, state, or authorization endpoint is invalid")
	}
	parsed, _ := url.Parse(client.AuthURL)
	query := parsed.Query()
	query.Set("client_id", client.ClientID)
	query.Set("response_type", "token")
	query.Set("state", state)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// Refresh exchanges an Imgur refresh token for a current access token.
func (client *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if !validOpaque(refreshToken, maxOpaqueLength) {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is required")
	}
	return client.token(ctx, url.Values{
		"refresh_token": {refreshToken}, "client_id": {client.ClientID},
		"client_secret": {client.ClientSecret}, "grant_type": {"refresh_token"},
	})
}

func (client *OAuthClient) token(ctx context.Context, values url.Values) (socialhub.Token, error) {
	if !validOpaque(client.ClientID, 512) || !validOpaque(client.ClientSecret, maxOpaqueLength) || client.HTTPClient == nil || client.Clock == nil || !validEndpoint(client.TokenURL) {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "OAuth client is incomplete")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError("oauth_refresh", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError("oauth_refresh", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, sanitizeTransportError(err))
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return socialhub.Token{}, platformError("oauth_refresh", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return socialhub.Token{}, platformError("oauth_refresh", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var payload struct {
			Error       string `json:"error"`
			Description string `json:"error_description"`
		}
		_ = json.Unmarshal(body, &payload)
		code, class := socialhub.CodeUnauthenticated, socialhub.ClassUserAction
		if payload.Error == "invalid_request" || payload.Error == "unsupported_grant_type" {
			code, class = socialhub.CodeInvalidArgument, socialhub.ClassPermanent
		}
		return socialhub.Token{}, &socialhub.Error{
			Code: code, Class: class, Platform: "imgur", Product: productName, Op: "oauth_refresh",
			HTTPStatus: response.StatusCode, PlatformCode: boundedMessage(payload.Error, 128), PlatformMessage: boundedMessage(payload.Description, 512),
		}
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || !validOpaque(payload.AccessToken, maxOpaqueLength) || payload.ExpiresIn < 0 || payload.ExpiresIn > int64((365*24*time.Hour)/time.Second) {
		return socialhub.Token{}, platformError("oauth_refresh", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	tokenType := payload.TokenType
	if tokenType == "" || strings.EqualFold(tokenType, "bearer") {
		tokenType = "Bearer"
	}
	refreshToken := firstNonEmpty(payload.RefreshToken, values.Get("refresh_token"))
	token := socialhub.Token{AccessToken: payload.AccessToken, RefreshToken: refreshToken, TokenType: tokenType}
	if payload.ExpiresIn > 0 {
		token.ExpiresAt = client.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	}
	return token, nil
}

func sanitizeTransportError(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
