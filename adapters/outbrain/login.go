package outbrain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxLoginResponseBytes int64 = 1 << 20

// LoginClient exchanges Outbrain API credentials for a 30-day OB-TOKEN-V1 token.
type LoginClient struct {
	Username   string
	Password   string
	BaseURL    string
	HTTPClient *http.Client
	Clock      socialhub.Clock
}

// Token calls GET /login using HTTP Basic authentication.
func (client *LoginClient) Token(ctx context.Context) (socialhub.Token, error) {
	if !validOpaque(client.Username, 1024) || !validOpaque(client.Password, 8192) || client.HTTPClient == nil || client.Clock == nil ||
		!validEndpoint(client.BaseURL) || strings.HasSuffix(client.BaseURL, "/") {
		return socialhub.Token{}, invalidArgument("login", "username, password, base URL without trailing slash, HTTP client, and clock are required")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.BaseURL+"/login", nil)
	if err != nil {
		return socialhub.Token{}, platformError("login", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.SetBasicAuth(client.Username, client.Password)
	request.Header.Set("Accept", "application/json")
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError("login", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxLoginResponseBytes+1))
	if err != nil {
		return socialhub.Token{}, platformError("login", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxLoginResponseBytes {
		return socialhub.Token{}, platformError("login", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("login response exceeded size limit"))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return socialhub.Token{}, withOperation(decodeHTTPError(response.StatusCode, response.Header, body), "login")
	}
	var payload struct {
		Token string `json:"OB-TOKEN-V1"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, platformError("login", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if !validOpaque(payload.Token, 8192) {
		return socialhub.Token{}, platformContractError("login", "invalid OB-TOKEN-V1 response")
	}
	return socialhub.Token{
		AccessToken: payload.Token, TokenType: "OB-TOKEN-V1",
		ExpiresAt: client.Clock.Now().Add(30 * 24 * time.Hour),
	}, nil
}
