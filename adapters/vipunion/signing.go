package vipunion

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxRequestBytes = 1 << 20

type vipAuthenticator struct {
	appKey      string
	accessToken string
	clock       socialhub.Clock
}

func (authenticator *vipAuthenticator) Authenticate(request *http.Request, token socialhub.Token) error {
	if request.Body == nil || token.AccessToken == "" {
		return fmt.Errorf("vipunion: request body and app secret are required")
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, maxRequestBytes+1))
	_ = request.Body.Close()
	if err != nil {
		return fmt.Errorf("vipunion: read request body: %w", err)
	}
	if len(body) == 0 || len(body) > maxRequestBytes {
		return fmt.Errorf("vipunion: request body is empty or exceeds 1 MiB")
	}
	query := request.URL.Query()
	if query.Get("service") == "" || query.Get("method") == "" || query.Get("version") == "" {
		return fmt.Errorf("vipunion: service, method, and version are required")
	}
	query.Set("appKey", authenticator.appKey)
	query.Set("accessToken", authenticator.accessToken)
	query.Set("timestamp", fmt.Sprintf("%d", authenticator.clock.Now().Unix()))
	query.Set("format", "JSON")
	query.Set("language", "zh")
	query.Set("sign", signHMACMD5(query, body, token.AccessToken))
	request.URL.RawQuery = query.Encode()
	request.Body = io.NopCloser(bytes.NewReader(body))
	request.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	request.ContentLength = int64(len(body))
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	return nil
}

func signHMACMD5(values url.Values, body []byte, secret string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		if key != "sign" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	builder := strings.Builder{}
	builder.Grow(len(body) + len(values)*32)
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteString(values.Get(key))
	}
	builder.Write(body)
	digest := hmac.New(md5.New, []byte(secret))
	_, _ = digest.Write([]byte(builder.String()))
	return strings.ToUpper(hex.EncodeToString(digest.Sum(nil)))
}
