package line

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestProfileContentAndQuotaContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer channel-token" {
			http.Error(writer, "missing bearer token", http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/v2/bot/profile/" + testUserID:
			writeTestJSON(t, writer, Profile{DisplayName: "Alice", UserID: testUserID, Language: "en"})
		case "/v2/bot/group/" + testGroupID + "/member/" + testUserID:
			writeTestJSON(t, writer, Profile{DisplayName: "Group Alice", UserID: testUserID})
		case "/v2/bot/room/" + testRoomID + "/member/" + testUserID:
			writeTestJSON(t, writer, Profile{DisplayName: "Room Alice", UserID: testUserID})
		case "/v2/bot/message/quota":
			writeTestJSON(t, writer, map[string]any{"type": "limited", "value": 1000})
		case "/v2/bot/message/quota/consumption":
			writeTestJSON(t, writer, map[string]any{"totalUsage": 123})
		case "/v2/bot/message/message-1/content":
			writer.Header().Set("Content-Type", "image/jpeg")
			writer.Header().Set("Content-Disposition", `attachment; filename="image.jpg"`)
			writer.Header().Set("Content-Length", "7")
			_, _ = writer.Write([]byte("content"))
		case "/v2/bot/message/message-1/content/preview":
			writer.Header().Set("Content-Type", "image/jpeg")
			_, _ = writer.Write([]byte("preview"))
		case "/v2/bot/message/message-1/content/transcoding":
			writeTestJSON(t, writer, map[string]string{"status": "succeeded"})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, true)

	profile, err := client.GetProfile(context.Background(), testUserID)
	if err != nil || profile.DisplayName != "Alice" || profile.Language != "en" {
		t.Fatalf("profile=%#v error=%v", profile, err)
	}
	groupProfile, err := client.GetGroupMemberProfile(context.Background(), testGroupID, testUserID)
	if err != nil || groupProfile.DisplayName != "Group Alice" {
		t.Fatalf("group profile=%#v error=%v", groupProfile, err)
	}
	roomProfile, err := client.GetRoomMemberProfile(context.Background(), testRoomID, testUserID)
	if err != nil || roomProfile.DisplayName != "Room Alice" {
		t.Fatalf("room profile=%#v error=%v", roomProfile, err)
	}
	quota, err := client.GetMessageQuota(context.Background())
	if err != nil || quota.Type != "limited" || quota.Value == nil || *quota.Value != 1000 {
		t.Fatalf("quota=%#v error=%v", quota, err)
	}
	consumption, err := client.GetQuotaConsumption(context.Background())
	if err != nil || consumption.TotalUsage != 123 {
		t.Fatalf("consumption=%#v error=%v", consumption, err)
	}
	content, err := client.DownloadContent(context.Background(), "message-1", socialhub.WithRequestID("request-1"))
	if err != nil {
		t.Fatal(err)
	}
	data, readErr := io.ReadAll(content.Body)
	closeErr := content.Body.Close()
	if readErr != nil || closeErr != nil || string(data) != "content" || content.ContentType != "image/jpeg" || content.ContentLength != 7 || !strings.Contains(content.ContentDisposition, "image.jpg") {
		t.Fatalf("content=%#v data=%q read=%v close=%v", content, data, readErr, closeErr)
	}
	preview, err := client.DownloadPreview(context.Background(), "message-1")
	if err != nil {
		t.Fatal(err)
	}
	previewData, _ := io.ReadAll(preview.Body)
	_ = preview.Body.Close()
	if string(previewData) != "preview" {
		t.Fatalf("preview=%q", previewData)
	}
	status, err := client.GetTranscodingStatus(context.Background(), "message-1")
	if err != nil || status != TranscodingSucceeded {
		t.Fatalf("transcoding=%q error=%v", status, err)
	}
}

func TestProfileContentAndQuotaValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v2/bot/profile/" + testUserID:
			writeTestJSON(t, writer, Profile{DisplayName: "Alice", UserID: testUserID2})
		case "/v2/bot/message/quota":
			writeTestJSON(t, writer, map[string]any{"type": "limited", "value": -1})
		case "/v2/bot/message/quota/consumption":
			writeTestJSON(t, writer, map[string]any{"totalUsage": -1})
		case "/v2/bot/message/message-1/content/transcoding":
			writeTestJSON(t, writer, map[string]string{"status": "unknown"})
		case "/v2/bot/message/message-1/content":
			writer.Header().Set("X-Line-Request-Id", "line-request")
			writer.Header().Set("Retry-After", "2")
			writer.WriteHeader(http.StatusTooManyRequests)
			_, _ = writer.Write([]byte(`{"message":"quota exceeded"}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, true)

	invalidCalls := []func() error{
		func() error { _, err := client.GetProfile(context.Background(), "bad"); return err },
		func() error {
			_, err := client.GetGroupMemberProfile(context.Background(), "bad", testUserID)
			return err
		},
		func() error {
			_, err := client.GetRoomMemberProfile(context.Background(), testRoomID, "bad")
			return err
		},
		func() error { _, err := client.DownloadContent(context.Background(), " "); return err },
		func() error { _, err := client.DownloadPreview(context.Background(), "bad id"); return err },
		func() error { _, err := client.GetTranscodingStatus(context.Background(), ""); return err },
	}
	for index, call := range invalidCalls {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("validation %d=%v", index, err)
		}
	}
	if _, err := client.GetProfile(context.Background(), testUserID); !hasErrorCode(err, socialhub.CodePlatformError) {
		t.Fatalf("mismatched profile=%v", err)
	}
	if _, err := client.GetMessageQuota(context.Background()); !hasErrorCode(err, socialhub.CodePlatformError) {
		t.Fatalf("malformed quota=%v", err)
	}
	if _, err := client.GetQuotaConsumption(context.Background()); !hasErrorCode(err, socialhub.CodePlatformError) {
		t.Fatalf("malformed consumption=%v", err)
	}
	if _, err := client.GetTranscodingStatus(context.Background(), "message-1"); !hasErrorCode(err, socialhub.CodePlatformError) {
		t.Fatalf("malformed transcoding=%v", err)
	}
	_, err := client.DownloadContent(context.Background(), "message-1")
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeRateLimited || platformErr.RequestID != "line-request" || platformErr.RetryAfter.Seconds() != 2 {
		t.Fatalf("content error=%#v", err)
	}
}
