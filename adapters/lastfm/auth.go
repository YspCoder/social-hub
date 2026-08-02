package lastfm

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

// Session is a user-authorized Last.fm session. Key must be stored as a secret.
type Session struct {
	Name       string `json:"name"`
	Key        string `json:"key"`
	Subscriber bool   `json:"subscriber"`
}

// RequestToken obtains a single-use token that expires after 60 minutes.
func (c *Client) RequestToken(ctx context.Context, options ...socialhub.CallOption) (string, error) {
	var response struct {
		Token string `json:"token"`
	}
	if err := c.get(ctx, "auth.getToken", nil, true, &response, options...); err != nil {
		return "", err
	}
	if !validCredential(response.Token) {
		return "", platformError("auth.getToken", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return response.Token, nil
}

// AuthorizationURL builds the Last.fm browser approval URL for a request token.
func (c *Client) AuthorizationURL(token, callback string) (string, error) {
	if !validCredential(token) || !validCallback(callback) {
		return "", invalidArgument("auth.authorize", "token or callback URL is invalid")
	}
	parsed := *c.authURL
	query := parsed.Query()
	query.Set("api_key", c.apiKey)
	query.Set("token", token)
	if callback != "" {
		query.Set("cb", callback)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// ExchangeSession consumes an approved request token and returns a revocable session key.
func (c *Client) ExchangeSession(ctx context.Context, token string, options ...socialhub.CallOption) (*Session, error) {
	if !validCredential(token) {
		return nil, invalidArgument("auth.getSession", "token is required")
	}
	var response struct {
		Session struct {
			Name       string       `json:"name"`
			Key        string       `json:"key"`
			Subscriber flexibleBool `json:"subscriber"`
		} `json:"session"`
	}
	if err := c.get(ctx, "auth.getSession", url.Values{"token": {token}}, true, &response, options...); err != nil {
		return nil, err
	}
	if !validText(response.Session.Name, 255) || !validCredential(response.Session.Key) {
		return nil, platformError("auth.getSession", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &Session{Name: response.Session.Name, Key: response.Session.Key, Subscriber: bool(response.Session.Subscriber)}, nil
}
