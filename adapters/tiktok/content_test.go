package tiktok

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

func TestContentPostingWorkflow(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v2/post/publish/creator_info/query/":
			writeJSON(writer, `{"data":{"creator_username":"creator","creator_nickname":"Creator","privacy_level_options":["SELF_ONLY"],"max_video_post_duration_sec":300},"error":{"code":"ok"}}`)
		case "/v2/post/publish/video/init/":
			writeJSON(writer, `{"data":{"publish_id":"publish-1","upload_url":"`+server.URL+`/upload?token=signed"},"error":{"code":"ok"}}`)
		case "/upload":
			body, _ := io.ReadAll(request.Body)
			if request.Method != http.MethodPut || request.Header.Get("Authorization") != "" || request.Header.Get("Content-Type") != "video/mp4" || request.Header.Get("Content-Range") != "bytes 0-3/4" || request.Header.Get("X-Request-ID") != "chunk-request" || string(body) != "data" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writer.Header().Set("Content-Range", "bytes 0-3/4")
			writer.WriteHeader(http.StatusCreated)
		case "/v2/post/publish/status/fetch/":
			writeJSON(writer, `{"data":{"status":"PUBLISH_COMPLETE","publicaly_available_post_id":["video-1"],"uploaded_bytes":4},"error":{"code":"ok"}}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{"video.publish"}, false)
	workflow := client.ContentWorkflow()
	creator, err := workflow.CreatorInfo(context.Background())
	if err != nil || len(creator.PrivacyLevelOptions) != 1 || creator.PrivacyLevelOptions[0] != PrivacySelfOnly {
		t.Fatalf("creator=%#v err=%v", creator, err)
	}
	task, err := workflow.InitVideo(context.Background(), VideoPostRequest{
		Title: "hello", PrivacyLevel: PrivacySelfOnly, Source: SourceFileUpload,
		VideoSize: 4, ChunkSize: 4, TotalChunks: 1, MIME: "video/mp4",
	})
	if err != nil || task.ID != "publish-1" || task.State != socialhub.PublishStatePending {
		t.Fatalf("task=%#v err=%v", task, err)
	}
	part, err := workflow.UploadChunk(context.Background(), task.ID, 0, strings.NewReader("data"), socialhub.WithRequestID("chunk-request"))
	if err != nil || part.Size != 4 || part.ETag != "bytes 0-3/4" {
		t.Fatalf("part=%#v err=%v", part, err)
	}
	if _, err := workflow.UploadChunk(context.Background(), task.ID, 0, strings.NewReader("data")); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("repeated chunk error=%v", err)
	}
	status, err := workflow.Status(context.Background(), task.ID)
	if err != nil || status.State != socialhub.PublishStatePublished || len(status.PublicPostIDs) != 1 {
		t.Fatalf("status=%#v err=%v", status, err)
	}
}

func TestVideoRequestValidation(t *testing.T) {
	invalid := []VideoPostRequest{
		{PrivacyLevel: "INVALID", Source: SourcePullFromURL, VideoURL: "https://cdn.example/video.mp4"},
		{PrivacyLevel: PrivacySelfOnly, Source: SourcePullFromURL, VideoURL: "http://cdn.example/video.mp4"},
		{PrivacyLevel: PrivacySelfOnly, Source: SourceFileUpload, VideoSize: 4, ChunkSize: 2, TotalChunks: 2, MIME: "video/mp4"},
		{PrivacyLevel: PrivacySelfOnly, Source: SourceFileUpload, VideoSize: 4, ChunkSize: 4, TotalChunks: 1, MIME: "application/octet-stream"},
	}
	for _, input := range invalid {
		if err := validateVideoRequest(input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("input=%#v error=%v", input, err)
		}
	}
}
