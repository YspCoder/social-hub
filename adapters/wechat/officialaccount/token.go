package officialaccount

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

type appTokenSource struct {
	mu         sync.Mutex
	baseURL    string
	appID      string
	secret     string
	httpClient *http.Client
	clock      socialhub.Clock
	token      socialhub.Token
}

func (s *appTokenSource) Token(ctx context.Context) (socialhub.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock.Now()
	if s.token.Valid(now.Add(5 * time.Minute)) {
		return s.token, nil
	}
	endpoint := strings.TrimRight(s.baseURL, "/") + "/cgi-bin/token"
	query := url.Values{"grant_type": {"client_credential"}, "appid": {s.appID}, "secret": {s.secret}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+query.Encode(), nil)
	if err != nil {
		return socialhub.Token{}, wrapError("token", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	response, err := s.httpClient.Do(request)
	if err != nil {
		var urlError *url.Error
		if errors.As(err, &urlError) && urlError.Err != nil {
			err = urlError.Err
		}
		return socialhub.Token{}, wrapError("token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil {
		return socialhub.Token{}, wrapError("token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if len(body) > 1<<20 {
		return socialhub.Token{}, wrapError("token", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("response exceeded size limit"))
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
		return socialhub.Token{}, wrapError("token", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if err := payload.APIResponse.Err("token"); err != nil {
		return socialhub.Token{}, err
	}
	if payload.AccessToken == "" || payload.ExpiresIn <= 0 {
		return socialhub.Token{}, wrapError("token", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("missing token fields"))
	}
	s.token = socialhub.Token{AccessToken: payload.AccessToken, ExpiresAt: now.Add(time.Duration(payload.ExpiresIn) * time.Second)}
	return s.token, nil
}
