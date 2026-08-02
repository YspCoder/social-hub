package tmdb

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

type authTokenResponse struct {
	Success        bool   `json:"success"`
	ExpiresAt      string `json:"expires_at"`
	RequestToken   string `json:"request_token"`
	GuestSessionID string `json:"guest_session_id"`
}

func (c *Client) RequestToken(ctx context.Context) (*RequestToken, error) {
	var response authTokenResponse
	if err := c.requestJSON(ctx, http.MethodGet, "/authentication/token/new", nil, nil, &response); err != nil {
		return nil, err
	}
	expiresAt, err := parseTMDBTime(response.ExpiresAt)
	if !response.Success || !validToken(response.RequestToken) || err != nil {
		return nil, platformError("request_token", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return &RequestToken{Token: response.RequestToken, ExpiresAt: expiresAt}, nil
}

func (c *Client) ApprovalURL(requestToken, redirectTo string) (string, error) {
	if !validToken(requestToken) || (redirectTo != "" && !validRedirectURI(redirectTo)) {
		return "", invalidArgument("approval_url", "request token or redirect URI is invalid")
	}
	parsed, _ := url.Parse(c.authURL)
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + requestToken
	if redirectTo != "" {
		query := parsed.Query()
		query.Set("redirect_to", redirectTo)
		parsed.RawQuery = query.Encode()
	}
	return parsed.String(), nil
}

func (c *Client) CreateSession(ctx context.Context, requestToken string) (string, error) {
	if !validToken(requestToken) {
		return "", invalidArgument("create_session", "request token is required")
	}
	var response struct {
		Success   bool   `json:"success"`
		SessionID string `json:"session_id"`
	}
	if err := c.requestJSON(ctx, http.MethodPost, "/authentication/session/new", nil, map[string]string{"request_token": requestToken}, &response); err != nil {
		return "", err
	}
	if !response.Success || !validCredential(response.SessionID) {
		return "", platformError("create_session", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return response.SessionID, nil
}

func (c *Client) DeleteSession(ctx context.Context, sessionID string) error {
	if !validCredential(sessionID) {
		return invalidArgument("delete_session", "session ID is required")
	}
	var response struct {
		Success bool `json:"success"`
	}
	if err := c.requestJSON(ctx, http.MethodDelete, "/authentication/session", nil, map[string]string{"session_id": sessionID}, &response); err != nil {
		return err
	}
	if !response.Success {
		return platformError("delete_session", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (c *Client) CreateGuestSession(ctx context.Context) (*GuestSession, error) {
	var response authTokenResponse
	if err := c.requestJSON(ctx, http.MethodGet, "/authentication/guest_session/new", nil, nil, &response); err != nil {
		return nil, err
	}
	expiresAt, err := parseTMDBTime(response.ExpiresAt)
	if !response.Success || !validCredential(response.GuestSessionID) || err != nil {
		return nil, platformError("create_guest_session", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return &GuestSession{ID: response.GuestSessionID, ExpiresAt: expiresAt}, nil
}

func parseTMDBTime(value string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02 15:04:05 MST", time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, &time.ParseError{Layout: "TMDB timestamp", Value: value}
}

func validToken(value string) bool {
	return validCredential(value) && !strings.ContainsAny(value, "/\\?#")
}
