package moloco

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	tokenLifetime     = 16 * time.Hour
	tokenRefreshSkew  = 5 * time.Minute
	maxTokenBodyBytes = 1 << 20
)

type tokenSource struct {
	mu         sync.Mutex
	apiKey     string
	httpClient *http.Client
	clock      socialhub.Clock
	token      socialhub.Token
	closed     bool
}

func (source *tokenSource) Token(ctx context.Context) (socialhub.Token, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	if source.closed || !validSecret(source.apiKey) {
		return socialhub.Token{}, socialhub.ErrUnauthenticated
	}
	now := source.clock.Now()
	if source.token.Valid(now.Add(tokenRefreshSkew)) {
		return source.token, nil
	}
	payload, err := json.Marshal(struct {
		APIKey string `json:"api_key"`
	}{APIKey: source.apiKey})
	if err != nil {
		return socialhub.Token{}, platformError("issue_token", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, defaultBaseURL+"/cm/v1/auth/tokens", bytes.NewReader(payload),
	)
	if err != nil {
		return socialhub.Token{}, platformError("issue_token", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Moloco-Cloud-Api-Version", apiVersion)
	response, err := source.httpClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError("issue_token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxTokenBodyBytes+1))
	if err != nil {
		return socialhub.Token{}, withHTTPStatus(
			platformError("issue_token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err), response.StatusCode,
		)
	}
	if len(body) > maxTokenBodyBytes {
		return socialhub.Token{}, platformContractError(
			"issue_token", "Moloco token response exceeded 1 MiB", response.StatusCode,
		)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return socialhub.Token{}, withOperation(
			decodeHTTPError(response.StatusCode, response.Header, body, now, source.apiKey), "issue_token",
		)
	}
	if response.StatusCode != http.StatusOK {
		return socialhub.Token{}, platformContractError(
			"issue_token", "Moloco returned an unexpected token success status", response.StatusCode,
		)
	}
	if !jsonContentType(response.Header.Get("Content-Type")) {
		return socialhub.Token{}, platformContractError(
			"issue_token", "Moloco returned a non-JSON token response", response.StatusCode,
		)
	}
	var output struct {
		Token     string `json:"token"`
		TokenType string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &output); err != nil {
		return socialhub.Token{}, withHTTPStatus(
			platformError("issue_token", socialhub.CodePlatformError, socialhub.ClassPermanent, err), response.StatusCode,
		)
	}
	if output.TokenType == "UPDATE_PASSWORD_TOKEN" {
		return socialhub.Token{}, &socialhub.Error{
			Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
			Platform: platformName, Product: productName, Op: "issue_token",
			PlatformCode:    "UPDATE_PASSWORD_TOKEN",
			PlatformMessage: "Moloco requires the user password to be updated before issuing an API auth token",
			ApprovalURL:     gettingStartedURL,
		}
	}
	if !validSecret(output.Token) || output.TokenType != "" && output.TokenType != "AUTH_TOKEN" {
		return socialhub.Token{}, platformContractError(
			"issue_token", "Moloco returned an invalid API auth token", response.StatusCode,
		)
	}
	source.token = socialhub.Token{AccessToken: output.Token, TokenType: "Bearer", ExpiresAt: now.Add(tokenLifetime)}
	return source.token, nil
}

func (source *tokenSource) redactionSecrets() []string {
	source.mu.Lock()
	defer source.mu.Unlock()
	return []string{source.apiKey, source.token.AccessToken}
}

func (source *tokenSource) close() {
	source.mu.Lock()
	defer source.mu.Unlock()
	source.closed = true
	source.apiKey = ""
	source.token = socialhub.Token{}
}

func jsonContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json"))
}

var _ socialhub.TokenSource = (*tokenSource)(nil)
