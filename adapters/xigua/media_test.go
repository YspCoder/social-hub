package xigua

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"social-hub/extensions/video"
	"social-hub/pkg/socialhub"
)

func TestDirectUploadAndTypedVideoWorkflow(t *testing.T) {
	t.Parallel()
	apiServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/xigua/video/upload/":
			if request.Method != http.MethodPost || request.URL.Query().Get("open_id") != "open-id-1" || request.Header.Get("access-token") != "user-token" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			if err := request.ParseMultipartForm(1 << 20); err != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			file, header, err := request.FormFile("video")
			if err != nil || header.Filename == "" || header.Header.Get("Content-Type") != "video/mp4" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			data, _ := io.ReadAll(file)
			_ = file.Close()
			videoID := "video-direct"
			if string(data) == "clip" {
				videoID = "video-workflow"
			} else if string(data) == "123456" {
				videoID = "video-overflow"
			}
			_, _ = writer.Write([]byte(`{"data":{"video":{"video_id":"` + videoID + `","width":720,"height":1280},"error_code":0}}`))
		case "/xigua/video/create/":
			var body map[string]string
			_ = json.NewDecoder(request.Body).Decode(&body)
			if body["video_id"] != "video-workflow" || body["text"] != "workflow demo" || body["abstract"] != "workflow summary" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"data":{"item_id":"item-workflow","error_code":0}}`))
		case "/xigua/video/data/":
			_, _ = writer.Write([]byte(`{"data":{"list":[{"item_id":"item-workflow","video_id":"video-workflow","title":"workflow demo","create_time":1785629045,"statistics":{}}],"error_code":0}}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer apiServer.Close()
	oauthServer := httptest.NewServer(http.NotFoundHandler())
	defer oauthServer.Close()
	_, client := newTestAdapter(t, apiServer, oauthServer)

	session, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "video.mp4", MIME: "video/mp4", Type: socialhub.MediaTypeVideo, Size: 5})
	if err != nil || session.ID == "" || session.PartSize != 5 {
		t.Fatalf("session=%#v err=%v", session, err)
	}
	status, err := client.MediaStatus(context.Background(), session.ID)
	if err != nil || status.State != socialhub.MediaStateUploading {
		t.Fatalf("uploading status=%#v err=%v", status, err)
	}
	part, err := client.UploadPart(context.Background(), session.ID, 0, bytes.NewBufferString("video"))
	if err != nil || part.Size != 5 || part.ETag != "video-direct" {
		t.Fatalf("part=%#v err=%v", part, err)
	}
	media, err := client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part})
	if err != nil || media.ID != "video-direct" || media.Width == nil || *media.Width != 720 || media.Height == nil || *media.Height != 1280 {
		t.Fatalf("media=%#v err=%v", media, err)
	}
	status, err = client.MediaStatus(context.Background(), media.ID)
	if err != nil || status.State != socialhub.MediaStateReady {
		t.Fatalf("ready status=%#v err=%v", status, err)
	}
	if _, err := client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part}); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("second completion error=%v", err)
	}

	overflow, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "overflow.mp4", MIME: "video/mp4", Type: socialhub.MediaTypeVideo, Size: 5})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.UploadPart(context.Background(), overflow.ID, 0, bytes.NewBufferString("123456")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("overflow error=%v", err)
	}

	workflow := client.VideoWorkflow()
	workflowSession, err := workflow.Create(context.Background(), video.CreateRequest{Filename: "clip.mp4", MIME: "video/mp4", Size: 4})
	if err != nil {
		t.Fatal(err)
	}
	if err := workflow.Upload(context.Background(), workflowSession.ID, bytes.NewBufferString("clip"), 4); err != nil {
		t.Fatal(err)
	}
	if err := workflow.Complete(context.Background(), workflowSession.ID); err != nil {
		t.Fatal(err)
	}
	job, err := workflow.Publish(context.Background(), workflowSession.ID, video.PublishRequest{Title: "workflow demo", Description: "workflow summary"})
	if err != nil || job.ID != "item-workflow" || job.State != video.StatePublishPending {
		t.Fatalf("job=%#v err=%v", job, err)
	}
	job, err = workflow.Status(context.Background(), job.ID)
	if err != nil || job.State != video.StatePublished || job.UpdatedAt == nil {
		t.Fatalf("job status=%#v err=%v", job, err)
	}
	if err := workflow.Abort(context.Background(), workflowSession.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := client.MediaStatus(context.Background(), "video-workflow"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("status after abort error=%v", err)
	}
}

func TestMultipartUploadRequestShapeAndLayout(t *testing.T) {
	t.Parallel()
	var initCalls atomic.Int32
	apiServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/xigua/video/part/init/":
			initCalls.Add(1)
			_, _ = writer.Write([]byte(`{"data":{"upload_id":"large-upload","error_code":0}}`))
		case "/xigua/video/part/upload/":
			partNumber, _ := strconv.Atoi(request.URL.Query().Get("part_number"))
			if request.URL.Query().Get("upload_id") != "chunk-fixture" || partNumber < 1 || partNumber > 2 {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			if err := request.ParseMultipartForm(6 << 20); err != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			file, _, err := request.FormFile("video")
			if err != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			size, _ := io.Copy(io.Discard, file)
			_ = file.Close()
			if size != minimumPartSize {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"data":{"error_code":0}}`))
		case "/xigua/video/part/complete/":
			if request.URL.Query().Get("upload_id") != "chunk-fixture" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"data":{"video":{"video_id":"video-large","width":1920,"height":1080},"error_code":0}}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer apiServer.Close()
	_, client := newTestAdapter(t, apiServer, apiServer)
	boundary, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "boundary.mp4", MIME: "video/mp4", Type: socialhub.MediaTypeVideo, Size: directUploadThreshold})
	if err != nil || !strings.HasPrefix(boundary.ID, "direct:") || initCalls.Load() != 0 {
		t.Fatalf("boundary=%#v calls=%d err=%v", boundary, initCalls.Load(), err)
	}
	large, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "large.mp4", MIME: "video/mp4", Type: socialhub.MediaTypeVideo, Size: directUploadThreshold + 1})
	if err != nil || large.ID != "large-upload" || large.PartSize != defaultPartSize || initCalls.Load() != 1 {
		t.Fatalf("large=%#v calls=%d err=%v", large, initCalls.Load(), err)
	}

	const fixtureSize = 10 << 20
	client.uploadMu.Lock()
	client.uploads["chunk-fixture"] = &uploadState{
		request: socialhub.BeginUploadRequest{Filename: "chunk.mp4", MIME: "video/mp4", Type: socialhub.MediaTypeVideo, Size: fixtureSize},
		chunked: true, parts: make(map[int]socialhub.UploadedPart), uploading: make(map[int]bool),
	}
	client.uploadMu.Unlock()
	parts := make([]socialhub.UploadedPart, 0, 2)
	for number := 0; number < 2; number++ {
		part, err := client.UploadPart(context.Background(), "chunk-fixture", number, bytes.NewReader(make([]byte, minimumPartSize)))
		if err != nil {
			t.Fatal(err)
		}
		parts = append(parts, *part)
	}
	media, err := client.CompleteUpload(context.Background(), "chunk-fixture", parts)
	if err != nil || media.ID != "video-large" || media.Size == nil || *media.Size != fixtureSize {
		t.Fatalf("media=%#v err=%v", media, err)
	}
}

func TestUploadStateConflictsAndValidation(t *testing.T) {
	t.Parallel()
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/xigua/video/upload/" {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		close(started)
		<-release
		_, _ = io.Copy(io.Discard, request.Body)
		_, _ = writer.Write([]byte(`{"data":{"video":{"video_id":"video-1"},"error_code":0}}`))
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, server)
	session, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "one.mp4", MIME: "video/mp4", Type: socialhub.MediaTypeVideo, Size: 1})
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		_, uploadErr := client.UploadPart(context.Background(), session.ID, 0, bytes.NewBufferString("x"))
		result <- uploadErr
	}()
	<-started
	if _, err := client.UploadPart(context.Background(), session.ID, 0, bytes.NewBufferString("x")); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("parallel upload error=%v", err)
	}
	if _, err := client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{{Number: 0, Size: 1, ETag: "video-1"}}); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("completion during upload error=%v", err)
	}
	close(release)
	if err := <-result; err != nil {
		t.Fatal(err)
	}

	workflow := client.VideoWorkflow()
	for _, test := range []struct {
		name string
		call func() error
		want error
	}{
		{"empty begin", func() error {
			_, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{})
			return err
		}, socialhub.ErrInvalidArgument},
		{"image unsupported", func() error {
			_, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "x.jpg", MIME: "image/jpeg", Type: socialhub.MediaTypeImage, Size: 1})
			return err
		}, socialhub.ErrUnsupported},
		{"too large", func() error {
			_, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "x.mp4", MIME: "video/mp4", Type: socialhub.MediaTypeVideo, Size: maximumVideoBytes + 1})
			return err
		}, socialhub.ErrInvalidArgument},
		{"missing upload", func() error {
			_, err := client.UploadPart(context.Background(), "missing", 0, bytes.NewReader(nil))
			return err
		}, socialhub.ErrNotFound},
		{"bad part", func() error {
			_, err := client.UploadPart(context.Background(), session.ID, -1, bytes.NewReader(nil))
			return err
		}, socialhub.ErrInvalidArgument},
		{"missing complete", func() error {
			_, err := client.CompleteUpload(context.Background(), "missing", []socialhub.UploadedPart{{Number: 0}})
			return err
		}, socialhub.ErrNotFound},
		{"bad status", func() error { _, err := client.MediaStatus(context.Background(), ""); return err }, socialhub.ErrInvalidArgument},
		{"empty publish session", func() error { _, err := workflow.Publish(context.Background(), "", video.PublishRequest{}); return err }, socialhub.ErrInvalidArgument},
		{"cover unsupported", func() error {
			_, err := workflow.Publish(context.Background(), "valid", video.PublishRequest{CoverID: "cover"})
			return err
		}, socialhub.ErrUnsupported},
		{"missing abort", func() error { return workflow.Abort(context.Background(), "missing") }, socialhub.ErrNotFound},
		{"bad workflow upload", func() error { return workflow.Upload(context.Background(), "", bytes.NewReader(nil), 0) }, socialhub.ErrInvalidArgument},
		{"bad workflow complete", func() error { return workflow.Complete(context.Background(), "missing") }, socialhub.ErrNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); !errors.Is(err, test.want) {
				t.Fatalf("error=%v want=%v", err, test.want)
			}
		})
	}
}

func TestUploadPartValidationHelpers(t *testing.T) {
	t.Parallel()
	recorded := map[int]socialhub.UploadedPart{0: {Number: 0, ETag: "a", Size: minimumPartSize}, 1: {Number: 1, ETag: "b", Size: 1}}
	parts := []socialhub.UploadedPart{recorded[1], recorded[0]}
	if total, ok := validateUploadedParts(parts, recorded); !ok || total != minimumPartSize+1 {
		t.Fatalf("total=%d ok=%v", total, ok)
	}
	if !validChunkLayout(parts) {
		t.Fatalf("valid layout rejected: %#v", parts)
	}
	bad := []socialhub.UploadedPart{{Number: 0, ETag: "a", Size: 1}, recorded[1]}
	if validChunkLayout(bad) {
		t.Fatal("undersized non-final part accepted")
	}
	if _, ok := validateUploadedParts([]socialhub.UploadedPart{recorded[0], recorded[0]}, recorded); ok {
		t.Fatal("duplicate parts accepted")
	}
	if safeFilename("a\\b\"c") != "a\\\\b\\\"c" {
		t.Fatalf("safe filename=%q", safeFilename("a\\b\"c"))
	}
}
