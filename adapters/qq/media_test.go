package qq

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestURLMediaUploadContract(t *testing.T) {
	type exchange struct {
		path    string
		payload map[string]any
	}
	exchanges := []exchange{
		{
			path: "/v2/users/user-id/files",
			payload: map[string]any{
				"file_type": float64(MediaFileImage), "url": "https://cdn.example.test/image.png",
				"srv_send_msg": false, "file_name": "image.png",
			},
		},
		{
			path: "/v2/groups/group%2Fid/files",
			payload: map[string]any{
				"file_type": float64(MediaFileVideo), "url": "http://cdn.example.test/video.mp4", "srv_send_msg": false,
			},
		},
	}
	index := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		want := exchanges[index]
		index++
		if request.Method != http.MethodPost || request.URL.Path != want.path || request.Header.Get("Authorization") != "QQBot access-token" {
			t.Errorf("request=%s %s auth=%q", request.Method, request.URL.Path, request.Header.Get("Authorization"))
		}
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || !reflect.DeepEqual(payload, want.payload) {
			t.Errorf("payload=%#v want=%#v err=%v", payload, want.payload, err)
		}
		writeTestJSON(t, writer, map[string]any{
			"file_uuid": "file-uuid", "file_info": "file-info", "ttl": 120, "raw_url": "https://cdn.example.test/resolved",
		})
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false)

	asset, err := client.UploadURL(context.Background(), UploadURLRequest{
		Target: Target{Scene: SceneC2C, ID: "user-id"}, Type: MediaFileImage,
		URL: "https://cdn.example.test/image.png", Filename: "image.png",
	})
	if err != nil || asset.FileUUID != "file-uuid" || asset.FileInfo != "file-info" || asset.TTL != 2*time.Minute || asset.ExpiresAt == nil || !asset.ExpiresAt.Equal(testNow.Add(2*time.Minute)) {
		t.Fatalf("asset=%#v err=%v", asset, err)
	}
	if _, err := client.UploadURL(context.Background(), UploadURLRequest{
		Target: Target{Scene: SceneGroup, ID: "group%2Fid"}, Type: MediaFileVideo, URL: "http://cdn.example.test/video.mp4",
	}); err != nil {
		t.Fatal(err)
	}
	if index != len(exchanges) {
		t.Fatalf("requests=%d want=%d", index, len(exchanges))
	}
}

func TestURLMediaValidationAndBusinessErrors(t *testing.T) {
	responses := []map[string]any{
		{"code": 100001, "message": "rate limited"},
		{"file_uuid": "", "file_info": "bad", "ttl": 1},
		{"file_uuid": "id", "file_info": "info", "ttl": -1},
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		response := responses[0]
		responses = responses[1:]
		writeTestJSON(t, writer, response)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false)
	valid := UploadURLRequest{Target: Target{Scene: SceneC2C, ID: "user"}, Type: MediaFileImage, URL: "https://example.test/image.png"}
	invalid := []UploadURLRequest{
		{Target: Target{Scene: SceneChannel, ID: "channel"}, Type: MediaFileImage, URL: valid.URL},
		{Target: valid.Target, Type: 0, URL: valid.URL},
		{Target: valid.Target, Type: MediaFileImage, URL: "file:///tmp/image.png"},
		{Target: valid.Target, Type: MediaFileImage, URL: valid.URL, Filename: " bad"},
	}
	for index, input := range invalid {
		if _, err := client.UploadURL(context.Background(), input); err == nil {
			t.Fatalf("invalid request %d unexpectedly succeeded", index)
		}
	}
	if _, err := client.UploadURL(context.Background(), valid); !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("rate error=%v", err)
	}
	if _, err := client.UploadURL(context.Background(), valid); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("malformed response=%v", err)
	}
	if _, err := client.UploadURL(context.Background(), valid); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("negative TTL=%v", err)
	}
}
