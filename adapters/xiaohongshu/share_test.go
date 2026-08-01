package xiaohongshu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestPrepareSignsTokenRequestAndCachesAbsoluteExpiry(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	expiresAt := start.Add(24 * time.Hour)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if request.Method != http.MethodPost || request.URL.Path != "/api/sns/v1/ext/access/token" || request.Header.Get("Content-Type") != "application/json" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		var body struct {
			AppKey    string `json:"app_key"`
			Nonce     string `json:"nonce"`
			Timestamp int64  `json:"timestamp"`
			Signature string `json:"signature"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode token request: %v", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		wantSignature := buildSignature(body.AppKey, body.Nonce, strconv.FormatInt(body.Timestamp, 10), "app-secret")
		if body.AppKey != "app-key" || body.Nonce == "" || body.Signature != wantSignature {
			t.Errorf("token request=%#v want_signature=%q", body, wantSignature)
		}
		_, _ = fmt.Fprintf(writer, `{"access_token":"share-token","expires_in":%d}`, expiresAt.UnixMilli())
	}))
	defer server.Close()
	clock := &steppingClock{now: start, step: time.Millisecond}
	_, client := newTestAdapter(t, server, true, "", nil, clock)

	token, err := client.accessToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "share-token" || !token.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("token=%#v", token)
	}
	payload, err := client.ShareWorkflow().Prepare(context.Background(), ShareRequest{
		Type:   ShareTypeNormal,
		Images: []string{"https://cdn.example/one.jpg", "https://cdn.example/two.jpg"},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantHandoffSignature := buildSignature("app-key", payload.VerifyConfig.Nonce, payload.VerifyConfig.Timestamp, "share-token")
	if payload.VerifyConfig.AppKey != "app-key" || payload.VerifyConfig.Signature != wantHandoffSignature || len(payload.ShareInfo.Images) != 2 {
		t.Fatalf("payload=%#v want_signature=%q", payload, wantHandoffSignature)
	}
	if _, err := client.ShareWorkflow().Prepare(context.Background(), ShareRequest{Type: ShareTypeVideo, VideoURL: "https://cdn.example/video.mp4", CoverURL: "https://cdn.example/cover.jpg"}); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("token requests=%d want=1", requests)
	}
}

func TestStaticAccessTokenSkipsTokenEndpoint(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests++
	}))
	defer server.Close()
	clock := &steppingClock{now: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}
	_, client := newTestAdapter(t, server, true, "test://share-token", nil, clock)
	payload, err := client.ShareWorkflow().Prepare(context.Background(), ShareRequest{Type: ShareTypeNormal, Images: []string{"https://cdn.example/note.jpg"}})
	if err != nil {
		t.Fatal(err)
	}
	want := buildSignature("app-key", payload.VerifyConfig.Nonce, payload.VerifyConfig.Timestamp, "static-share-token")
	if payload.VerifyConfig.Signature != want || requests != 0 {
		t.Fatalf("payload=%#v requests=%d", payload, requests)
	}
}

type wrappedMissStore struct{ puts int }

func (*wrappedMissStore) Get(context.Context, socialhub.TokenKey) (socialhub.Token, error) {
	return socialhub.Token{}, fmt.Errorf("cache miss: %w", socialhub.ErrNotFound)
}

func (s *wrappedMissStore) Put(_ context.Context, _ socialhub.TokenKey, _ socialhub.Token) error {
	s.puts++
	return nil
}

func (*wrappedMissStore) Delete(context.Context, socialhub.TokenKey) error { return nil }

func TestWrappedTokenStoreMissFallsBackToTokenEndpoint(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(writer, `{"access_token":"share-token","expires_in":%d}`, start.Add(time.Hour).UnixMilli())
	}))
	defer server.Close()
	store := &wrappedMissStore{}
	clock := &steppingClock{now: start}
	_, client := newTestAdapter(t, server, true, "", store, clock)
	if _, err := client.accessToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.puts != 1 {
		t.Fatalf("token store puts=%d want=1", store.puts)
	}
}

func TestShareRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		request ShareRequest
		valid   bool
	}{
		{name: "normal", request: ShareRequest{Type: ShareTypeNormal, Images: []string{"https://cdn.example/note.jpg"}}, valid: true},
		{name: "normal HTTP URL", request: ShareRequest{Type: ShareTypeNormal, Images: []string{"http://cdn.example/note.jpg"}}},
		{name: "normal with video", request: ShareRequest{Type: ShareTypeNormal, Images: []string{"https://cdn.example/note.jpg"}, VideoURL: "https://cdn.example/video.mp4"}},
		{name: "video", request: ShareRequest{Type: ShareTypeVideo, VideoURL: "https://cdn.example/video.mp4", CoverURL: "https://cdn.example/cover.jpg"}, valid: true},
		{name: "video HTTP URL", request: ShareRequest{Type: ShareTypeVideo, VideoURL: "http://cdn.example/video.mp4"}},
		{name: "video without URL", request: ShareRequest{Type: ShareTypeVideo}},
		{name: "unknown type", request: ShareRequest{Type: "live"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.request.Validate()
			if test.valid && err != nil {
				t.Fatalf("error=%v", err)
			}
			if !test.valid && !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
