package wecom

import (
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

type corpTokenSource struct {
	mu         sync.Mutex
	baseURL    string
	corpID     string
	secret     string
	httpClient *http.Client
	clock      socialhub.Clock
	store      socialhub.TokenStore
	key        socialhub.TokenKey
	token      socialhub.Token
}

func (s *corpTokenSource) Token(ctx context.Context) (socialhub.Token, error) {
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

	endpoint := strings.TrimRight(s.baseURL, "/") + "/cgi-bin/gettoken"
	query := url.Values{"corpid": {s.corpID}, "corpsecret": {s.secret}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+query.Encode(), nil)
	if err != nil {
		return socialhub.Token{}, platformError("token", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
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
		APIResponse
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, platformError("token", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if err := payload.APIResponse.Err("token"); err != nil {
		return socialhub.Token{}, err
	}
	if payload.AccessToken == "" || len(payload.AccessToken) > 512 || payload.ExpiresIn <= 0 {
		return socialhub.Token{}, platformError("token", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid token response fields"))
	}
	token := socialhub.Token{AccessToken: payload.AccessToken, ExpiresAt: now.Add(time.Duration(payload.ExpiresIn) * time.Second)}
	if s.store != nil {
		if err := s.store.Put(ctx, s.key, token); err != nil {
			return socialhub.Token{}, platformError("token_cache_put", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
		}
	}
	s.token = token
	return token, nil
}

func (s *corpTokenSource) Invalidate(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = socialhub.Token{}
	if s.store != nil {
		_ = s.store.Delete(ctx, s.key)
	}
}
