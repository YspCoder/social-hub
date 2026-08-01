package qq

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

type appTokenSource struct {
	mu         sync.Mutex
	tokenURL   string
	appID      string
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
	if s.token.Valid(now.Add(time.Minute)) {
		return s.token, nil
	}
	if s.store != nil {
		stored, err := s.store.Get(ctx, s.key)
		if err == nil && stored.Valid(now.Add(time.Minute)) {
			s.token = stored
			return stored, nil
		}
		if err != nil && !errors.Is(err, socialhub.ErrNotFound) {
			return socialhub.Token{}, platformError("token_cache_get", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
		}
	}

	encoded, err := json.Marshal(struct {
		AppID        string `json:"appId"`
		ClientSecret string `json:"clientSecret"`
	}{AppID: s.appID, ClientSecret: s.secret})
	if err != nil {
		return socialhub.Token{}, platformError("token", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.tokenURL, bytes.NewReader(encoded))
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
	body, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil {
		return socialhub.Token{}, platformError("token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if len(body) > 1<<20 {
		return socialhub.Token{}, platformError("token", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("response exceeded size limit"))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return socialhub.Token{}, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	var payload struct {
		APIError
		AccessToken string          `json:"access_token"`
		ExpiresIn   flexibleSeconds `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, platformError("token", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if err := payload.APIError.Err("token"); err != nil {
		return socialhub.Token{}, err
	}
	if payload.AccessToken == "" || len(payload.AccessToken) > 2048 || payload.ExpiresIn <= 0 || int64(payload.ExpiresIn) > math.MaxInt64/int64(time.Second) {
		return socialhub.Token{}, platformError("token", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid token response fields"))
	}
	token := socialhub.Token{
		AccessToken: payload.AccessToken, TokenType: "QQBot",
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

type flexibleSeconds int64

func (value *flexibleSeconds) UnmarshalJSON(data []byte) error {
	text := strings.Trim(string(data), `"`)
	seconds, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return err
	}
	*value = flexibleSeconds(seconds)
	return nil
}
