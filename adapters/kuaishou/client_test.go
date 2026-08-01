package kuaishou

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestGetUserMapsPublicProfile(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/openapi/user_info" || request.URL.Query().Get("app_id") != "app-id" || request.URL.Query().Get("access_token") != "user-token" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = writer.Write([]byte(`{"result":1,"user_info":{"name":"Creator","sex":"F","fan":12,"follow":17,"head":"https://img/small.jpg","bigHead":"https://img/big.jpg","city":"Shanghai"}}`))
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	user, err := client.GetUser(context.Background(), "open-id-1")
	if err != nil || user.ID != "open-id-1" || user.DisplayName == nil || *user.DisplayName != "Creator" || user.AvatarURL == nil || *user.AvatarURL != "https://img/big.jpg" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	var extension map[string]any
	if err := json.Unmarshal(user.Extensions["kuaishou.user"], &extension); err != nil || extension["fan"] != float64(12) {
		t.Fatalf("extension=%#v err=%v", extension, err)
	}
}

func TestMissingScopeReturnsApprovalRequired(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server)
	client.scopes = []string{"user_info"}
	_, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "video.mp4", MIME: "video/mp4", Type: socialhub.MediaTypeVideo, Size: 5})
	if !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("error=%v", err)
	}
}

func TestPostAndCommentReadsAreExplicitlyUnsupported(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server)
	if _, err := client.GetPost(context.Background(), "photo-1"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("get post error=%v", err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "photo-1"}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("list comments error=%v", err)
	}
}
