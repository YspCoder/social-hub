package yahoodisplayads

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// OAuthClient implements LINE Yahoo's OAuth 2.0 authorization-code flow.
type OAuthClient struct {
	clientID     string
	clientSecret string
	httpClient   *http.Client
	clock        socialhub.Clock
	requestIDs   *requestIDFilter
}

func (client *OAuthClient) AuthorizationURL(redirectURI, state string) (string, error) {
	if !validOpaque(client.clientID, 1_024) || !validCallbackURL(redirectURI) || !validOpaque(state, 1_024) {
		return "", invalidArgument("oauth_authorize", "client ID, redirect URI, state, or authorization endpoint is invalid")
	}
	parsed, _ := url.Parse(defaultAuthURL)
	query := parsed.Query()
	query.Set("client_id", client.clientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("scope", oauthScope)
	query.Set("state", state)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (client *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	if !validOpaque(code, 4096) || !validCallbackURL(redirectURI) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "authorization code and redirect URI are required")
	}
	values := url.Values{
		"client_id": {client.clientID}, "client_secret": {client.clientSecret}, "code": {code},
		"grant_type": {"authorization_code"}, "redirect_uri": {redirectURI},
	}
	return client.token(ctx, "oauth_exchange", values, "", code)
}

func (client *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if !validOpaque(refreshToken, 16_384) {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is required")
	}
	values := url.Values{
		"client_id": {client.clientID}, "client_secret": {client.clientSecret},
		"grant_type": {"refresh_token"}, "refresh_token": {refreshToken},
	}
	return client.token(ctx, "oauth_refresh", values, refreshToken, refreshToken)
}

func (client *OAuthClient) token(
	ctx context.Context,
	operation string,
	values url.Values,
	existingRefreshToken string,
	redactions ...string,
) (socialhub.Token, error) {
	if !validOpaque(client.clientID, 1_024) || !validOpaque(client.clientSecret, 16_384) ||
		client.httpClient == nil || client.clock == nil || client.requestIDs == nil {
		return socialhub.Token{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	client.requestIDs.add(redactions...)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, defaultTokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return socialhub.Token{}, platformContractError(operation, "OAuth response exceeded 1 MiB")
	}
	var payload struct {
		AccessToken  string          `json:"access_token"`
		ExpiresIn    durationSeconds `json:"expires_in"`
		RefreshToken string          `json:"refresh_token"`
		Scope        string          `json:"scope"`
		TokenType    string          `json:"token_type"`
		Error        string          `json:"error"`
	}
	decodeErr := json.Unmarshal(body, &payload)
	if response.StatusCode < 200 || response.StatusCode >= 300 || payload.Error != "" {
		platformCode := ""
		if decodeErr == nil {
			platformCode = payload.Error
		}
		return socialhub.Token{}, oauthError(
			operation, response.StatusCode, response.Header, platformCode, client.clock.Now(), client.requestIDs,
		)
	}
	if decodeErr != nil {
		return socialhub.Token{}, platformContractError(operation, "OAuth token response was not valid JSON")
	}
	refreshToken := payload.RefreshToken
	if refreshToken == "" {
		refreshToken = existingRefreshToken
	}
	seconds := int64(payload.ExpiresIn)
	if !validOpaque(payload.AccessToken, 16_384) || seconds <= 0 || seconds > int64((24*time.Hour)/time.Second) ||
		!strings.EqualFold(payload.TokenType, "bearer") {
		return socialhub.Token{}, platformContractError(operation, "OAuth token response fields are invalid")
	}
	if operation == "oauth_exchange" && !validOpaque(refreshToken, 16_384) {
		return socialhub.Token{}, platformContractError(operation, "authorization-code response omitted refresh_token")
	}
	scopes := strings.Fields(payload.Scope)
	if len(scopes) == 0 {
		scopes = []string{oauthScope}
	}
	if !validReturnedScopes(scopes) {
		return socialhub.Token{}, platformContractError(operation, "OAuth token response scope metadata is invalid")
	}
	now := client.clock.Now()
	if now.Unix() <= 0 {
		return socialhub.Token{}, invalidArgument(operation, "clock must return a time after the Unix epoch")
	}
	client.requestIDs.add(payload.AccessToken, refreshToken)
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: refreshToken, TokenType: "Bearer",
		ExpiresAt: now.Add(time.Duration(seconds) * time.Second), Scopes: scopes,
	}, nil
}

type durationSeconds int64

func (seconds *durationSeconds) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty duration")
	}
	value := strings.TrimSpace(string(data))
	if strings.HasPrefix(value, "\"") {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		value = text
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return err
	}
	*seconds = durationSeconds(parsed)
	return nil
}

func oauthError(
	operation string,
	status int,
	header http.Header,
	platformCode string,
	now time.Time,
	requestIDs *requestIDFilter,
) error {
	platformCode = validPlatformCode(platformCode)
	code, class := classifyError(status, "")
	switch platformCode {
	case "invalid_client", "invalid_grant", "unauthorized_client":
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "access_denied", "invalid_scope":
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "invalid_request", "unsupported_grant_type":
		code, class = socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "temporarily_unavailable", "server_error":
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: platformCode,
		PlatformMessage: "LINE Yahoo OAuth token request failed",
		RequestID:       requestIDs.safe(header.Get("x-z-rid")),
		RetryAfter:      retryDelay(header, now),
	}
}
