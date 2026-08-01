package kuaishou

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"social-hub/extensions/video"
	"social-hub/pkg/socialhub"
)

func TestDirectVideoWorkflowRequiresAndPublishesCover(t *testing.T) {
	t.Parallel()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/openapi/photo/start_upload":
			if request.URL.Query().Get("app_id") != "app-id" || request.URL.Query().Get("access_token") != "user-token" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			parsed, _ := url.Parse(server.URL)
			_, _ = writer.Write([]byte(`{"result":1,"upload_token":"upload-1","endpoint":"` + parsed.Host + `"}`))
		case "/api/upload":
			body, _ := io.ReadAll(request.Body)
			if request.URL.Query().Get("upload_token") != "upload-1" || request.URL.Query().Has("access_token") || request.Header.Get("Content-Type") != "video/mp4" || string(body) != "video" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"result":1}`))
		case "/openapi/photo/publish":
			if request.URL.Query().Get("upload_token") != "upload-1" || request.URL.Query().Get("access_token") != "user-token" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			if err := request.ParseMultipartForm(1 << 20); err != nil || request.FormValue("caption") != "demo caption" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			cover, header, err := request.FormFile("cover")
			if err != nil || header.Filename != "cover.jpg" || header.Header.Get("Content-Type") != "image/jpeg" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			data, _ := io.ReadAll(cover)
			_ = cover.Close()
			if string(data) != "cover" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"result":1,"video_info":{"photo_id":"photo-1","caption":"demo caption","cover":"https://img/cover.jpg","play_url":"https://video/play.mp4","create_time":1785542400000,"like_count":1,"comment_count":2,"view_count":3,"pending":false}}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)

	workflow := client.VideoWorkflow()
	session, err := workflow.Create(context.Background(), video.CreateRequest{Filename: "video.mp4", MIME: "video/mp4", Size: 5})
	if err != nil || session.ID != "upload-1" || session.PartSize != 5 {
		t.Fatalf("session=%#v err=%v", session, err)
	}
	if err := workflow.Upload(context.Background(), session.ID, strings.NewReader("video"), 5); err != nil {
		t.Fatal(err)
	}
	if err := workflow.Complete(context.Background(), session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := workflow.Publish(context.Background(), session.ID, video.PublishRequest{Description: "demo caption"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("publish without cover error=%v", err)
	}
	coverID := uploadCover(t, client, "cover.jpg", "image/jpeg", []byte("cover"))
	job, err := workflow.Publish(context.Background(), session.ID, video.PublishRequest{Description: "demo caption", CoverID: coverID})
	if err != nil || job.ID != "photo-1" || job.State != video.StatePublished {
		t.Fatalf("job=%#v err=%v", job, err)
	}
	status, err := workflow.Status(context.Background(), job.ID)
	if err != nil || status.State != video.StatePublished {
		t.Fatalf("status=%#v err=%v", status, err)
	}
}

func TestFragmentUploadUsesZeroBasedIDsAndCount(t *testing.T) {
	t.Parallel()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		parsed, _ := url.Parse(server.URL)
		switch request.URL.Path {
		case "/openapi/photo/start_upload":
			_, _ = writer.Write([]byte(`{"result":1,"upload_token":"chunk-1","endpoint":"` + parsed.Host + `"}`))
		case "/api/upload/fragment":
			fragmentID, err := strconv.Atoi(request.URL.Query().Get("fragment_id"))
			body, _ := io.ReadAll(request.Body)
			if err != nil || fragmentID < 0 || fragmentID > 1 || len(body) != 3 {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"result":1,"checksum":"sum-` + strconv.Itoa(fragmentID) + `","size":3}`))
		case "/api/upload/complete":
			if request.URL.Query().Get("upload_token") != "chunk-1" || request.URL.Query().Get("fragment_count") != "2" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"result":1}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)

	// A small fixture is marked chunked after start_upload so the contract test
	// exercises wire behavior without allocating a 10 MiB payload.
	session, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "chunk.mp4", MIME: "video/mp4", Type: socialhub.MediaTypeVideo, Size: 6})
	if err != nil {
		t.Fatal(err)
	}
	client.uploadMu.Lock()
	client.uploads[session.ID].chunked = true
	client.uploadMu.Unlock()
	parts := make([]socialhub.UploadedPart, 0, 2)
	for number := 0; number < 2; number++ {
		part, err := client.UploadPart(context.Background(), session.ID, number, bytes.NewReader([]byte("abc")))
		if err != nil {
			t.Fatal(err)
		}
		parts = append(parts, *part)
	}
	media, err := client.CompleteUpload(context.Background(), session.ID, parts)
	if err != nil || media.ID != "chunk-1" || media.State != socialhub.MediaStateReady {
		t.Fatalf("media=%#v err=%v", media, err)
	}
}

func TestDynamicUploadEndpointRejectsUntrustedHost(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"result":1,"upload_token":"upload-1","endpoint":"169.254.169.254"}`))
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	_, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "video.mp4", MIME: "video/mp4", Type: socialhub.MediaTypeVideo, Size: 5})
	if !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("error=%v", err)
	}
}

func TestCoverSizeIsBounded(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server)
	_, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "cover.jpg", MIME: "image/jpeg", Type: socialhub.MediaTypeImage, Size: 10 << 20})
	if !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("error=%v", err)
	}
}

func uploadCover(t *testing.T, client *Client, filename, mimeType string, data []byte) string {
	t.Helper()
	session, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: filename, MIME: mimeType, Type: socialhub.MediaTypeImage, Size: int64(len(data))})
	if err != nil {
		t.Fatal(err)
	}
	part, err := client.UploadPart(context.Background(), session.ID, 0, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	media, err := client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part})
	if err != nil {
		t.Fatal(err)
	}
	return media.ID
}
