package xiaomiglobalreporting

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"social-hub/pkg/socialhub"
)

const maxTokenResponseBytes int64 = 1 << 20

// TokenClient creates and rotates Xiaomi Global tokens. It never stores
// returned credentials; callers must persist them in an encrypted store.
type TokenClient struct {
	appID      string
	appKey     string
	httpClient *http.Client
	clock      socialhub.Clock
}

func (*TokenClient) String() string {
	return "xiaomiglobalreporting.TokenClient(<redacted credentials>)"
}
func (*TokenClient) GoString() string {
	return "xiaomiglobalreporting.TokenClient(<redacted credentials>)"
}

func (client *TokenClient) Create(ctx context.Context, options ...socialhub.CallOption) (TokenBundle, error) {
	const operation = "token_create"
	if client == nil || !validOpaque(client.appID, 256) || !validOpaque(client.appKey, 16_384) {
		return TokenBundle{}, invalidArgument(operation, "token client appId or appKey is invalid")
	}
	return client.request(ctx, operation, "/foreign/token/createToken", struct {
		AppID  string `json:"appId"`
		AppKey string `json:"appKey"`
	}{AppID: client.appID, AppKey: client.appKey}, []string{client.appKey}, options...)
}

func (client *TokenClient) Refresh(ctx context.Context, refreshToken string, options ...socialhub.CallOption) (TokenBundle, error) {
	const operation = "token_refresh"
	if client == nil || !validCookieValue(refreshToken) {
		return TokenBundle{}, invalidArgument(operation, "refresh token is invalid")
	}
	return client.request(ctx, operation, "/foreign/token/refreshToken", struct {
		RefreshToken string `json:"refreshToken"`
	}{RefreshToken: refreshToken}, []string{client.appKey, refreshToken}, options...)
}

func (client *TokenClient) request(
	ctx context.Context,
	operation string,
	path string,
	input any,
	secrets []string,
	options ...socialhub.CallOption,
) (TokenBundle, error) {
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return TokenBundle{}, err
	}
	if client.httpClient == nil || client.clock == nil {
		return TokenBundle{}, invalidArgument(operation, "token client is incomplete")
	}
	callOptions, err := socialhub.ResolveCallOptions(prepared...)
	if err != nil {
		return TokenBundle{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		defer cancel()
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return TokenBundle{}, credentialError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err, secrets...)
	}
	if len(encoded) > maxRequestBytes {
		return TokenBundle{}, invalidArgument(operation, "token request JSON exceeds 1 MiB")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(encoded))
	if err != nil {
		return TokenBundle{}, credentialError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err, secrets...)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return TokenBundle{}, credentialError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err, secrets...)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxTokenResponseBytes+1))
	if err != nil {
		return TokenBundle{}, credentialError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err, secrets...)
	}
	if int64(len(body)) > maxTokenResponseBytes {
		return TokenBundle{}, platformContractError(operation, "Xiaomi token response exceeded the size limit", response.StatusCode)
	}
	var envelope apiEnvelope
	decodeErr := json.Unmarshal(body, &envelope)
	platformCode := scalarCode(envelope.Code)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if decodeErr == nil && platformCode != "" {
			return TokenBundle{}, businessError(
				operation, response.StatusCode, response.Header, platformCode, envelope.Message,
				envelope.TraceID, "", client.clock.Now(), secrets...,
			)
		}
		return TokenBundle{}, withOperationAndRequestID(
			decodeHTTPError(response.StatusCode, response.Header, body, client.clock.Now(), secrets...),
			operation, "", secrets...,
		)
	}
	if response.StatusCode != http.StatusOK {
		return TokenBundle{}, platformContractError(operation, "Xiaomi returned an unexpected successful token status", response.StatusCode)
	}
	if !validJSONContentType(response.Header.Get("Content-Type")) {
		return TokenBundle{}, platformContractError(operation, "Xiaomi returned a non-JSON token response", response.StatusCode)
	}
	if decodeErr != nil || platformCode == "" {
		return TokenBundle{}, platformContractError(operation, "Xiaomi returned an invalid token response envelope", response.StatusCode)
	}
	if platformCode != "0" {
		return TokenBundle{}, businessError(
			operation, response.StatusCode, response.Header, platformCode, envelope.Message,
			envelope.TraceID, "", client.clock.Now(), secrets...,
		)
	}
	if len(envelope.Result) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Result), []byte("null")) {
		return TokenBundle{}, platformContractError(operation, "Xiaomi token success response omitted its result", response.StatusCode)
	}
	return client.decodeTokenResult(operation, envelope.Result)
}

func (client *TokenClient) decodeTokenResult(operation string, raw json.RawMessage) (TokenBundle, error) {
	var result struct {
		AccessToken       string `json:"accessToken"`
		ExpireDate        string `json:"expireDate"`
		RefreshToken      string `json:"refreshToken"`
		RefreshExpireDate string `json:"refreshExpireDate"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return TokenBundle{}, platformContractError(operation, "Xiaomi returned an invalid token result")
	}
	if len(result.ExpireDate) == 0 || len(result.ExpireDate) > 64 ||
		len(result.RefreshExpireDate) == 0 || len(result.RefreshExpireDate) > 64 {
		return TokenBundle{}, platformContractError(operation, "Xiaomi returned invalid token expiry fields")
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, result.ExpireDate)
	if err != nil {
		return TokenBundle{}, platformContractError(operation, "Xiaomi returned an invalid access-token expiry")
	}
	refreshExpiresAt, err := time.Parse(time.RFC3339Nano, result.RefreshExpireDate)
	if err != nil {
		return TokenBundle{}, platformContractError(operation, "Xiaomi returned an invalid refresh-token expiry")
	}
	if !validCookieValue(result.AccessToken) || !validCookieValue(result.RefreshToken) ||
		!expiresAt.After(client.clock.Now()) || !refreshExpiresAt.After(expiresAt) {
		return TokenBundle{}, platformContractError(operation, "Xiaomi returned incomplete token credentials")
	}
	return TokenBundle{
		Token: socialhub.Token{
			AccessToken: result.AccessToken, RefreshToken: result.RefreshToken,
			TokenType: "MiAdsCookie", ExpiresAt: expiresAt.UTC(),
		},
		ExpireDate: result.ExpireDate, RefreshExpireDate: result.RefreshExpireDate,
		RefreshExpiresAt: refreshExpiresAt.UTC(),
	}, nil
}
