package dingtalk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

const maxTokenResponseBytes = 1 << 20

type appTokenSource struct {
	mu         sync.Mutex
	baseURL    string
	corpID     string
	clientID   string
	secret     string
	httpClient *http.Client
	clock      socialhub.Clock
	store      socialhub.TokenStore
	key        socialhub.TokenKey
	token      socialhub.Token
}

func (s *appTokenSource) Token(ctx context.Context) (socialhub.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock.Now()
	if s.token.Valid(now.Add(5 * time.Minute)) {
		return s.token, nil
	}
	if s.store != nil {
		stored, err := s.store.Get(ctx, s.key)
		if err == nil && stored.Valid(now.Add(5*time.Minute)) {
			s.token = stored
			return stored, nil
		}
		if err != nil && !errors.Is(err, socialhub.ErrNotFound) {
			return socialhub.Token{}, platformError("token_cache_get", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
		}
	}

	encoded, err := json.Marshal(struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		GrantType    string `json:"grant_type"`
	}{ClientID: s.clientID, ClientSecret: s.secret, GrantType: "client_credentials"})
	if err != nil {
		return socialhub.Token{}, platformError("token", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	endpoint := strings.TrimRight(s.baseURL, "/") + "/v1.0/oauth2/" + url.PathEscape(s.corpID) + "/token"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return socialhub.Token{}, platformError("token", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := s.httpClient.Do(request)
	if err != nil {
		var urlError *url.Error
		if errors.As(err, &urlError) && urlError.Err != nil {
			err = urlError.Err
		}
		return socialhub.Token{}, platformError("token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxTokenResponseBytes+1))
	if err != nil {
		return socialhub.Token{}, platformError("token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if len(body) > maxTokenResponseBytes {
		return socialhub.Token{}, platformError("token", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("response exceeded size limit"))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return socialhub.Token{}, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	var payload struct {
		apiError
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, platformError("token", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if err := payload.apiError.Err("token", response.StatusCode, response.Header); err != nil {
		return socialhub.Token{}, err
	}
	if !validOpaque(payload.AccessToken, 4096) || payload.ExpiresIn <= 0 || payload.ExpiresIn > int64((30*24*time.Hour)/time.Second) {
		return socialhub.Token{}, platformError("token", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid token response fields"))
	}
	token := socialhub.Token{
		AccessToken: payload.AccessToken, TokenType: "DingTalkApp",
		ExpiresAt: now.Add(time.Duration(payload.ExpiresIn) * time.Second),
	}
	if s.store != nil {
		if err := s.store.Put(ctx, s.key, token); err != nil {
			return socialhub.Token{}, platformError("token_cache_put", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
		}
	}
	s.token = token
	return token, nil
}

func (s *appTokenSource) Invalidate(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = socialhub.Token{}
	if s.store != nil {
		_ = s.store.Delete(ctx, s.key)
	}
}
