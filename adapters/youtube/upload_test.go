package youtube

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestYouTubeVideoUploadWorkflow(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/upload/youtube/v3/videos":
			var body map[string]any
			if request.Method != http.MethodPost || request.URL.Query().Get("uploadType") != "resumable" || request.Header.Get("X-Upload-Content-Length") != "4" || json.NewDecoder(request.Body).Decode(&body) != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writer.Header().Set("Location", server.URL+"/resumable?upload_id=signed")
			writer.WriteHeader(http.StatusOK)
		case "/resumable":
			body, _ := io.ReadAll(request.Body)
			if request.Method != http.MethodPut || request.Header.Get("Authorization") != "Bearer access-token" || request.Header.Get("Content-Type") != "video/mp4" || request.Header.Get("X-Request-ID") != "upload-request" || string(body) != "data" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"id":"video-3","snippet":{"channelId":"channel-1","title":"Upload"},"status":{"uploadStatus":"uploaded","privacyStatus":"private"}}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{"https://www.googleapis.com/auth/youtube.upload"})
	session, err := client.VideoUploadWorkflow().Initialize(context.Background(), VideoUploadRequest{Title: "Upload", PrivacyStatus: "private", ContainsSyntheticMedia: true, MIME: "video/mp4", Size: 4})
	if err != nil || session.ID == "" || session.Size != 4 {
		t.Fatalf("session=%#v err=%v", session, err)
	}
	post, err := client.VideoUploadWorkflow().Upload(context.Background(), session.ID, strings.NewReader("data"), socialhub.WithRequestID("upload-request"))
	if err != nil || post.ID != "video-3" || post.Status.State != socialhub.PublishStatePending {
		t.Fatalf("post=%#v err=%v", post, err)
	}
}
