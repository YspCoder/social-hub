package admitadpublisher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// OAuthClient implements Admitad's OAuth2 Client Credentials and refresh-token
// contracts.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	TokenURL     string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
	Scopes       []string
}

// ClientCredentials obtains an application bearer token and its refresh token.
func (client *OAuthClient) ClientCredentials(ctx context.Context) (socialhub.Token, error) {
	values := url.Values{
		"grant_type": {"client_credentials"},
		"client_id":  {client.ClientID},
		"scope":      {strings.Join(client.Scopes, " ")},
	}
	return client.token(ctx, "oauth_client_credentials", values, "", true)
}

// Refresh exchanges a refresh token for a replacement credential bundle.
func (client *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if !validOpaque(refreshToken, 16_384) {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is required")
	}
	values := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {client.ClientID},
		"client_secret": {client.ClientSecret},
		"refresh_token": {refreshToken},
	}
	return client.token(ctx, "oauth_refresh", values, refreshToken, false)
}

func (client *OAuthClient) token(ctx context.Context, operation string, values url.Values, existingRefreshToken string, basicAuth bool) (socialhub.Token, error) {
	if !validOpaque(client.ClientID, 1024) || !validOpaque(client.ClientSecret, 16_384) ||
		client.HTTPClient == nil || client.Clock == nil || !validEndpoint(client.TokenURL) || !validOAuthScopes(client.Scopes) {
		return socialhub.Token{}, invalidArgument(operation, "OAuth client credentials, scopes, token URL, HTTP client, and clock are required")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if basicAuth {
		request.SetBasicAuth(client.ClientID, client.ClientSecret)
	}
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("OAuth response exceeded size limit"))
	}
	var payload struct {
		AccessToken      string     `json:"access_token"`
		RefreshToken     string     `json:"refresh_token"`
		TokenType        string     `json:"token_type"`
		ExpiresIn        int64      `json:"expires_in"`
		Scope            string     `json:"scope"`
		Error            string     `json:"error"`
		ErrorDescription string     `json:"error_description"`
		ErrorCode        ExactValue `json:"error_code"`
	}
	decodeErr := json.Unmarshal(body, &payload)
	if response.StatusCode < 200 || response.StatusCode >= 300 || payload.Error != "" || payload.ErrorCode.IsSet() {
		return socialhub.Token{}, withOperationAndScope(
			decodeProviderError(response.StatusCode, response.Header, body, client.Clock.Now()), operation, strings.Join(client.Scopes, " "),
		)
	}
	if decodeErr != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, decodeErr)
	}
	refreshToken := payload.RefreshToken
	if refreshToken == "" {
		refreshToken = existingRefreshToken
	}
	if !validOpaque(payload.AccessToken, 16_384) || !validOpaque(refreshToken, 16_384) ||
		payload.ExpiresIn <= 0 || payload.ExpiresIn > int64((31*24*time.Hour)/time.Second) ||
		(payload.TokenType != "" && !strings.EqualFold(payload.TokenType, "bearer")) {
		return socialhub.Token{}, platformContractError(operation, "Admitad returned invalid token response fields")
	}
	scopes := strings.Fields(payload.Scope)
	if len(scopes) == 0 {
		scopes = append([]string(nil), client.Scopes...)
	}
	if !validOAuthScopes(scopes) {
		return socialhub.Token{}, platformContractError(operation, "Admitad returned invalid OAuth scopes")
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: refreshToken, TokenType: "Bearer",
		ExpiresAt: client.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second), Scopes: scopes,
	}, nil
}
