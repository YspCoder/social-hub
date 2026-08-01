package pinterest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestPinterestFetchAndPinWorkflow(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/signed-upload" {
			if request.Header.Get("Authorization") != "" || request.Method != http.MethodPost || request.ParseMultipartForm(1<<20) != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			file, header, err := request.FormFile("file")
			if err != nil || header.Filename != "video.mp4" || request.FormValue("policy") != "signed-policy" || request.FormValue("key") != "uploads/video" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			defer file.Close()
			body, _ := io.ReadAll(file)
			if string(body) != "video-data" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		if request.Header.Get("Authorization") != "Bearer access-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v5/user_account":
			writeJSON(writer, `{"id":"123","username":"pinner","business_name":"Pinner Inc","account_type":"BUSINESS","profile_image":"https://cdn.example/avatar.jpg","follower_count":12}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v5/pins/111":
			writeJSON(writer, imagePinJSON("111"))
		case request.Method == http.MethodGet && request.URL.Path == "/v5/pins":
			if request.URL.Query().Get("bookmark") != "cursor" || request.URL.Query().Get("page_size") != "250" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"bookmark":"next","items":[`+imagePinJSON("222")+`]}`)
		case request.Method == http.MethodPost && request.URL.Path == "/v5/media":
			writeJSON(writer, `{"media_id":"444","media_type":"video","upload_url":"`+server.URL+`/signed-upload?signature=secret","upload_parameters":{"key":"uploads/video","policy":"signed-policy"}}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v5/media/444":
			writeJSON(writer, `{"media_id":"444","media_type":"video","status":"succeeded"}`)
		case request.Method == http.MethodPost && request.URL.Path == "/v5/pins":
			var input PinCreateRequest
			if json.NewDecoder(request.Body).Decode(&input) != nil || input.BoardID != "999" || input.MediaSource.MediaID != "444" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"id":"555","title":"Video Pin","description":"description","board_id":"999","created_at":"2026-08-01T00:00:00Z","media":{"media_type":"video","video_url":"https://cdn.example/video.mp4","width":720,"height":1280,"duration":1500}}`)
		case request.Method == http.MethodDelete && request.URL.Path == "/v5/pins/555":
			writer.WriteHeader(http.StatusNoContent)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{"user_accounts:read", "boards:read", "boards:write", "pins:read", "pins:write"})

	user, err := client.GetUser(context.Background(), "123")
	if err != nil || user.ID != "123" || user.DisplayName == nil || *user.DisplayName != "Pinner Inc" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	post, err := client.GetPost(context.Background(), "111")
	if err != nil || len(post.Media) != 1 || post.Media[0].Width == nil || *post.Media[0].Width != 1200 || len(post.Metrics) != 1 {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "123", Cursor: "cursor", MaxResults: 300})
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil || *page.NextCursor != "next" || !page.HasMore {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "111"}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("comments error=%v", err)
	}

	workflow := client.PinWorkflow()
	upload, err := workflow.RegisterVideo(context.Background())
	if err != nil || upload.MediaID != "444" || upload.State != socialhub.MediaStateCreated {
		t.Fatalf("upload=%#v err=%v", upload, err)
	}
	if err := workflow.UploadVideo(context.Background(), upload.MediaID, "video.mp4", strings.NewReader("video-data")); err != nil {
		t.Fatal(err)
	}
	if err := workflow.UploadVideo(context.Background(), upload.MediaID, "video.mp4", strings.NewReader("video-data")); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("repeat upload error=%v", err)
	}
	status, err := workflow.MediaStatus(context.Background(), upload.MediaID)
	if err != nil || status.State != socialhub.MediaStateReady {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	created, err := workflow.Create(context.Background(), PinCreateRequest{
		BoardID: "999", Title: "Video Pin", Description: "description",
		MediaSource: PinMediaSource{SourceType: MediaSourceVideoID, MediaID: upload.MediaID, CoverImageURL: "https://cdn.example/cover.jpg"},
	})
	if err != nil || created.ID != "555" || created.Media[0].Type != socialhub.MediaTypeVideo || created.Media[0].Duration == nil || *created.Media[0].Duration != 1500*time.Millisecond {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	if err := workflow.Delete(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
}

func TestPinValidation(t *testing.T) {
	invalid := []PinCreateRequest{
		{BoardID: "board", MediaSource: PinMediaSource{SourceType: MediaSourceImageURL, URL: "https://cdn.example/image.jpg"}},
		{BoardID: "1", MediaSource: PinMediaSource{SourceType: MediaSourceImageURL, URL: "file:///image.jpg"}},
		{BoardID: "1", MediaSource: PinMediaSource{SourceType: MediaSourceVideoID, MediaID: "media"}},
	}
	for _, input := range invalid {
		if err := validatePinCreate(input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("input=%#v error=%v", input, err)
		}
	}
}

func imagePinJSON(id string) string {
	return `{"id":"` + id + `","title":"Image Pin","description":"hello","board_id":"999","created_at":"2026-08-01T00:00:00Z","media":{"media_type":"image","images":{"600x":{"url":"https://cdn.example/small.jpg","width":600,"height":400},"1200x":{"url":"https://cdn.example/large.jpg","width":1200,"height":800}}},"pin_metrics":{"90d":{"impression":4}}}`
}

func writeJSON(writer http.ResponseWriter, value string) {
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(value))
}
