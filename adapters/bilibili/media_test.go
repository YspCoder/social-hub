package bilibili

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/extensions/video"
	"social-hub/pkg/socialhub"
)

func TestSingleVideoCoverAndSubmissionWorkflow(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/arcopen/fn/archive/video/init":
			body, ok := verifySignedRequest(request, nil)
			var input map[string]string
			if !ok || json.Unmarshal(body, &input) != nil || input["name"] != "video.mp4" || input["utype"] != "1" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"code":0,"message":"0","data":{"upload_token":"upload-1"}}`))
		case "/video/v2/upload":
			body, _ := io.ReadAll(request.Body)
			if request.URL.Query().Get("upload_token") != "upload-1" || request.Header.Get("Access-Token") != "" || request.Header.Get("Authorization") != "" || request.Header.Get("Content-Type") != "application/octet-stream" || request.ContentLength != 5 || string(body) != "video" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"code":0,"message":"0"}`))
		case "/arcopen/fn/archive/cover/upload":
			empty := md5.Sum(nil)
			digest := hex.EncodeToString(empty[:])
			_, ok := verifySignedRequest(request, &digest)
			if !ok {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			if err := request.ParseMultipartForm(1 << 20); err != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			file, header, err := request.FormFile("file")
			if err != nil || header.Filename != "cover.jpg" || header.Header.Get("Content-Type") != "image/jpeg" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			data, _ := io.ReadAll(file)
			_ = file.Close()
			if string(data) != "cover" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"code":0,"message":"0","data":{"url":"https://img/cover.jpg"}}`))
		case "/arcopen/fn/archive/add-by-utoken":
			body, ok := verifySignedRequest(request, nil)
			var input struct {
				Title     string `json:"title"`
				Cover     string `json:"cover"`
				TID       int    `json:"tid"`
				Tag       string `json:"tag"`
				Copyright int    `json:"copyright"`
				NoReprint int    `json:"no_reprint"`
			}
			if !ok || json.Unmarshal(body, &input) != nil || request.URL.Query().Get("upload_token") != "upload-1" || input.Title != "demo title" || input.Cover != "https://img/cover.jpg" || input.TID != 229 || input.Tag != "social-hub" || input.Copyright != 1 || input.NoReprint != 1 {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"code":0,"message":"0","data":{"resource_id":"BV1NEW"}}`))
		case "/arcopen/fn/archive/view":
			if _, ok := verifySignedRequest(request, nil); !ok || request.URL.Query().Get("resource_id") != "BV1NEW" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"code":0,"message":"0","data":` + strings.ReplaceAll(archiveFixture, "BV1TEST", "BV1NEW") + `}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)

	workflow := client.VideoWorkflow()
	session, err := workflow.Create(context.Background(), video.CreateRequest{Filename: "video.mp4", MIME: "video/mp4", Size: 5})
	if err != nil || session.ID != "upload-1" {
		t.Fatalf("session=%#v err=%v", session, err)
	}
	if err := workflow.Upload(context.Background(), session.ID, strings.NewReader("video"), 5); err != nil {
		t.Fatal(err)
	}
	if err := workflow.Complete(context.Background(), session.ID); err != nil {
		t.Fatal(err)
	}
	coverID := uploadCover(t, client, []byte("cover"))
	job, err := workflow.Publish(context.Background(), session.ID, video.PublishRequest{Title: "demo title", Description: "description", CoverID: coverID})
	if err != nil || job.ID != "BV1NEW" || job.State != video.StatePublishPending {
		t.Fatalf("job=%#v err=%v", job, err)
	}
	status, err := workflow.Status(context.Background(), job.ID)
	if err != nil || status.State != video.StatePublished {
		t.Fatalf("status=%#v err=%v", status, err)
	}
}

func TestTypedSubmissionValidatesRepostSource(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server)
	_, err := client.SubmissionWorkflow().Publish(context.Background(), SubmissionRequest{UploadToken: "video", Title: "title", TID: 1, Tags: []string{"tag"}, Copyright: 2})
	if !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("error=%v", err)
	}
}

func TestCommonPublisherUsesAccountDefaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/arcopen/fn/archive/add-by-utoken" || request.URL.Query().Get("upload_token") != "video-1" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		body, ok := verifySignedRequest(request, nil)
		var input map[string]any
		if !ok || json.Unmarshal(body, &input) != nil || input["title"] != "common title" || input["tid"] != float64(229) || input["tag"] != "social-hub" || input["no_reprint"] != float64(1) {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = writer.Write([]byte(`{"code":0,"message":"0","data":{"resource_id":"BV1COMMON"}}`))
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	client.uploads["video-1"] = &uploadState{
		request: socialhub.BeginUploadRequest{Filename: "video.mp4", MIME: "video/mp4", Type: socialhub.MediaTypeVideo, Size: 5},
		media:   &socialhub.Media{ID: "video-1", MIME: "video/mp4", Type: socialhub.MediaTypeVideo, State: socialhub.MediaStateReady}, completed: true,
	}
	title := "common title"
	post, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &title, MediaIDs: []string{"video-1"}})
	if err != nil || post.ID != "BV1COMMON" {
		t.Fatalf("post=%#v err=%v", post, err)
	}
}

func TestLargeVideoAndInvalidCoverAreExplicit(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server)
	_, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "large.mp4", MIME: "video/mp4", Type: socialhub.MediaTypeVideo, Size: maximumSingleVideoBytes + 1})
	if !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("large video error=%v", err)
	}
	_, err = client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "cover.gif", MIME: "image/gif", Type: socialhub.MediaTypeImage, Size: 5})
	if !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("cover error=%v", err)
	}
}

func uploadCover(t *testing.T, client *Client, data []byte) string {
	t.Helper()
	session, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "cover.jpg", MIME: "image/jpeg", Type: socialhub.MediaTypeImage, Size: int64(len(data))})
	if err != nil {
		t.Fatal(err)
	}
	part, err := client.UploadPart(context.Background(), session.ID, 0, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	media, err := client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part})
	if err != nil || media.URL != "https://img/cover.jpg" {
		t.Fatalf("media=%#v err=%v", media, err)
	}
	return media.ID
}
