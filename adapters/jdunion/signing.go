package jdunion

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxRequestBytes = 1 << 20

var jdLocation = time.FixedZone("UTC+8", 8*60*60)

type jdAuthenticator struct {
	appKey      string
	accessToken string
	clock       socialhub.Clock
}

func (authenticator *jdAuthenticator) Authenticate(request *http.Request, token socialhub.Token) error {
	if request.Body == nil || token.AccessToken == "" {
		return fmt.Errorf("jdunion: request body and app secret are required")
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, maxRequestBytes+1))
	_ = request.Body.Close()
	if err != nil {
		return fmt.Errorf("jdunion: read request form: %w", err)
	}
	if len(body) > maxRequestBytes {
		return fmt.Errorf("jdunion: request form exceeds 1 MiB")
	}
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return fmt.Errorf("jdunion: parse request form: %w", err)
	}
	values.Set("app_key", authenticator.appKey)
	values.Set("format", "json")
	values.Set("sign_method", "md5")
	values.Set("timestamp", authenticator.clock.Now().In(jdLocation).Format("2006-01-02 15:04:05"))
	values.Set("v", apiVersion)
	if authenticator.accessToken != "" {
		values.Set("access_token", authenticator.accessToken)
	}
	values.Set("sign", signMD5(values, token.AccessToken))
	payload := values.Encode()
	if len(payload) > maxRequestBytes {
		return fmt.Errorf("jdunion: signed request form exceeds 1 MiB")
	}
	request.Body = io.NopCloser(strings.NewReader(payload))
	request.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(payload)), nil }
	request.ContentLength = int64(len(payload))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	return nil
}

func signMD5(values url.Values, secret string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		if key != "sign" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	builder := strings.Builder{}
	builder.Grow(len(secret)*2 + len(values)*32)
	builder.WriteString(secret)
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteString(values.Get(key))
	}
	builder.WriteString(secret)
	digest := md5.Sum([]byte(builder.String()))
	return strings.ToUpper(hex.EncodeToString(digest[:]))
}
