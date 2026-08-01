package vimeo

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestVimeoTUSVideoWorkflow(t *testing.T) {
	var server *httptest.Server
	var offset int64
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/api/me/videos":
			var body struct {
				Name   string `json:"name"`
				Upload struct {
					Approach string `json:"approach"`
					Size     int64  `json:"size"`
				} `json:"upload"`
			}
			if request.Header.Get("Authorization") != "Bearer access-token" || json.NewDecoder(request.Body).Decode(&body) != nil || body.Upload.Approach != "tus" || body.Upload.Size != 4 {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			link := server.URL + "/tus/signed?token=opaque"
			if body.Name == "evil" {
				link = "https://uploads.example.net/tus/signed"
			}
			if body.Name == "bad-offset" {
				link = server.URL + "/tus/bad-offset"
			}
			writeJSON(writer, `{"uri":"/videos/video-3","upload":{"approach":"tus","upload_link":"`+link+`"}}`)
		case request.Method == http.MethodPatch && request.URL.Path == "/tus/signed":
			if request.Header.Get("Authorization") != "" || request.Header.Get("Tus-Resumable") != "1.0.0" || request.Header.Get("Content-Type") != "application/offset+octet-stream" || request.Header.Get("Upload-Offset") != strconvInt(offset) {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			body, _ := io.ReadAll(request.Body)
			if offset == 0 && (request.Header.Get("X-Request-ID") != "tus-part" || string(body) != "da") {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			offset += int64(len(body))
			writer.Header().Set("Upload-Offset", strconvInt(offset))
			writer.Header().Set("ETag", "part-"+strconvInt(offset))
			writer.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodPatch && request.URL.Path == "/tus/bad-offset":
			_, _ = io.Copy(io.Discard, request.Body)
			writer.Header().Set("Upload-Offset", "99")
			writer.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodGet && request.URL.Path == "/api/videos/video-3":
			writeJSON(writer, videoJSON("video-3", "transcoding"))
		case request.Method == http.MethodPatch && request.URL.Path == "/api/videos/video-3":
			var body map[string]any
			if json.NewDecoder(request.Body).Decode(&body) != nil || body["name"] != "Renamed" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, videoJSON("video-3", "available"))
		case request.Method == http.MethodDelete && request.URL.Path == "/api/videos/video-3":
			writer.WriteHeader(http.StatusNoContent)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{"public", "upload", "edit", "delete"})
	workflow := client.VideoUploadWorkflow()
	session, err := workflow.Initialize(context.Background(), VideoUploadRequest{Name: "Video", Description: "Description", Size: 4, PrivacyView: "nobody"})
	if err != nil || session.VideoID != "video-3" || session.Offset != 0 || session.PartSize != 4 {
		t.Fatalf("session=%#v err=%v", session, err)
	}
	if _, err := workflow.UploadPart(context.Background(), session.ID, 1, strings.NewReader("bad")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("part order error=%v", err)
	}
	part0, err := workflow.UploadPart(context.Background(), session.ID, 0, strings.NewReader("da"), socialhub.WithRequestID("tus-part"))
	if err != nil || part0.Number != 0 || part0.Size != 2 || part0.ETag != "part-2" {
		t.Fatalf("part0=%#v err=%v", part0, err)
	}
	if _, err := workflow.Complete(context.Background(), session.ID, []socialhub.UploadedPart{*part0}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("early complete error=%v", err)
	}
	part1, err := workflow.UploadPart(context.Background(), session.ID, 1, strings.NewReader("ta"))
	if err != nil || part1.Size != 2 {
		t.Fatalf("part1=%#v err=%v", part1, err)
	}
	status, err := workflow.Status(context.Background(), session.VideoID)
	if err != nil || status.Status.State != socialhub.PublishStatePending {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	name := "Renamed"
	privacy := "unlisted"
	updated, err := workflow.Update(context.Background(), session.VideoID, VideoUpdateRequest{Name: &name, PrivacyView: &privacy})
	if err != nil || updated.Status.State != socialhub.PublishStatePublished {
		t.Fatalf("updated=%#v err=%v", updated, err)
	}
	post, err := workflow.Complete(context.Background(), session.ID, []socialhub.UploadedPart{*part0, *part1})
	if err != nil || post.ID != "video-3" || post.Status.State != socialhub.PublishStatePending {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	if _, err := workflow.Complete(context.Background(), session.ID, nil); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("second complete error=%v", err)
	}
	if err := workflow.Delete(context.Background(), session.VideoID); err != nil {
		t.Fatal(err)
	}
}

func TestTUSSecurityOffsetAndValidation(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost && request.URL.Path == "/api/me/videos" {
			var body struct {
				Name string `json:"name"`
			}
			_ = json.NewDecoder(request.Body).Decode(&body)
			link := server.URL + "/tus/bad-offset"
			if body.Name == "evil" {
				link = "http://evil.example/upload"
			}
			if body.Name == "redirect" {
				link = server.URL + "/tus/redirect"
			}
			writeJSON(writer, `{"uri":"/videos/video-3","upload":{"approach":"tus","upload_link":"`+link+`"}}`)
			return
		}
		if request.URL.Path == "/tus/redirect" {
			writer.Header().Set("Location", "https://evil.example/upload")
			writer.WriteHeader(http.StatusTemporaryRedirect)
			return
		}
		if request.URL.Path == "/tus/bad-offset" {
			_, _ = io.Copy(io.Discard, request.Body)
			writer.Header().Set("Upload-Offset", "99")
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		writer.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{"public", "upload", "edit", "delete"})
	workflow := client.VideoUploadWorkflow()
	if _, err := workflow.Initialize(context.Background(), VideoUploadRequest{Name: "evil", Size: 4}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("evil URL error=%v", err)
	}
	redirectSession, err := workflow.Initialize(context.Background(), VideoUploadRequest{Name: "redirect", Size: 4})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workflow.UploadPart(context.Background(), redirectSession.ID, 0, strings.NewReader("data")); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("redirect error=%v", err)
	}
	session, err := workflow.Initialize(context.Background(), VideoUploadRequest{Name: "bad-offset", Size: 4})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workflow.UploadPart(context.Background(), session.ID, 0, strings.NewReader("data")); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("offset error=%v", err)
	}
	if _, err := workflow.UploadPart(context.Background(), "missing", 0, strings.NewReader("data")); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing session error=%v", err)
	}
	if _, err := workflow.Initialize(context.Background(), VideoUploadRequest{Name: "", Size: 0}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("initialize validation=%v", err)
	}
	if _, err := workflow.Initialize(context.Background(), VideoUploadRequest{Name: "video", Size: 4, PrivacyView: "secret"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("privacy validation=%v", err)
	}
	if _, err := workflow.Update(context.Background(), "video-3", VideoUpdateRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("update validation=%v", err)
	}
	empty := ""
	if _, err := workflow.Update(context.Background(), "video-3", VideoUpdateRequest{Name: &empty}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty name validation=%v", err)
	}
	if err := workflow.Delete(context.Background(), "bad/id"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("delete validation=%v", err)
	}
	if validTUSUploadURL("https://evil.example/upload", client.apiBaseURL) || !validTUSUploadURL("https://files.vimeo.com/upload", client.apiBaseURL) || validTUSUploadURL("http://files.vimeo.com/upload", client.apiBaseURL) {
		t.Fatal("TUS URL allowlist mismatch")
	}
}

func strconvInt(value int64) string { return strconv.FormatInt(value, 10) }

func errorCode(err error) socialhub.ErrorCode {
	var hubError *socialhub.Error
	if errors.As(err, &hubError) {
		return hubError.Code
	}
	return ""
}
