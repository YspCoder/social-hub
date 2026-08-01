package bluesky

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxSessionResponseBytes int64 = 1 << 20

// SessionClient implements official com.atproto.server legacy sessions. It is
// intended for headless integrations and bots using app passwords.
type SessionClient struct {
	ServiceURL string
	Identifier string
	Password   string
	HTTPClient *http.Client
}

// Create exchanges an identifier and app password for rotating session JWTs.
// authFactorToken is optional and is used when the PDS requests a second factor.
func (c *SessionClient) Create(ctx context.Context, authFactorToken string) (*SessionInfo, error) {
	if strings.TrimSpace(c.Identifier) == "" || strings.TrimSpace(c.Password) == "" {
		return nil, invalidArgument("create_session", "identifier and app password are required")
	}
	input := struct {
		Identifier      string `json:"identifier"`
		Password        string `json:"password"`
		AuthFactorToken string `json:"authFactorToken,omitempty"`
	}{c.Identifier, c.Password, authFactorToken}
	var response sessionResponse
	if err := c.do(ctx, http.MethodPost, "com.atproto.server.createSession", input, "", &response); err != nil {
		return nil, err
	}
	return mapSession(response)
}

// Refresh rotates a session using the refresh JWT, as required by the Lexicon.
func (c *SessionClient) Refresh(ctx context.Context, refreshToken string) (*SessionInfo, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, invalidArgument("refresh_session", "refresh token is required")
	}
	var response sessionResponse
	if err := c.do(ctx, http.MethodPost, "com.atproto.server.refreshSession", nil, refreshToken, &response); err != nil {
		return nil, err
	}
	return mapSession(response)
}

// Delete revokes the current legacy session using its refresh JWT.
func (c *SessionClient) Delete(ctx context.Context, refreshToken string) error {
	if strings.TrimSpace(refreshToken) == "" {
		return invalidArgument("delete_session", "refresh token is required")
	}
	return c.do(ctx, http.MethodPost, "com.atproto.server.deleteSession", nil, refreshToken, nil)
}

type sessionResponse struct {
	AccessJWT      string `json:"accessJwt"`
	RefreshJWT     string `json:"refreshJwt"`
	Handle         string `json:"handle"`
	DID            string `json:"did"`
	Email          string `json:"email"`
	EmailConfirmed bool   `json:"emailConfirmed"`
	Active         *bool  `json:"active"`
	Status         string `json:"status"`
}

func mapSession(response sessionResponse) (*SessionInfo, error) {
	if response.AccessJWT == "" || response.RefreshJWT == "" || response.Handle == "" || !validDID(response.DID) {
		return nil, platformError("session", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	active := response.Active == nil || *response.Active
	if !active {
		return nil, &socialhub.Error{
			Code: socialhub.CodePermissionDenied, Class: socialhub.ClassUserAction, Platform: "bluesky", Product: productName,
			Op: "session", PlatformCode: xrpcErrorCode(response.Status), PlatformMessage: "account is not active",
		}
	}
	return &SessionInfo{
		DID: response.DID, Handle: response.Handle, Email: response.Email, EmailConfirmed: response.EmailConfirmed,
		Active: active, Status: response.Status,
		Token: socialhub.Token{
			AccessToken: response.AccessJWT, RefreshToken: response.RefreshJWT, TokenType: "Bearer", ExpiresAt: jwtExpiry(response.AccessJWT),
		},
	}, nil
}

func (c *SessionClient) do(ctx context.Context, method, xrpcMethod string, input any, authorization string, output any) error {
	if !validServiceURL(c.ServiceURL) || c.HTTPClient == nil {
		return invalidArgument("session", "service URL and HTTP client are required")
	}
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return platformError("session", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, normalizeServiceURL(c.ServiceURL)+"/xrpc/"+xrpcMethod, body)
	if err != nil {
		return platformError("session", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if authorization != "" {
		request.Header.Set("Authorization", "Bearer "+authorization)
	}
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return platformError("session", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxSessionResponseBytes+1))
	if err != nil {
		return platformError("session", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(payload)) > maxSessionResponseBytes {
		return platformError("session", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return decodeHTTPError(response.StatusCode, response.Header, payload)
	}
	if output == nil || len(payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, output); err != nil {
		return platformError("session", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return nil
}

func jwtExpiry(token string) time.Time {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}
	}
	var claims struct {
		ExpiresAt json.Number `json:"exp"`
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if decoder.Decode(&claims) != nil {
		return time.Time{}
	}
	expiresAt, err := claims.ExpiresAt.Int64()
	if err != nil || expiresAt <= 0 {
		return time.Time{}
	}
	return time.Unix(expiresAt, 0).UTC()
}
