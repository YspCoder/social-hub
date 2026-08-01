package douyin

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"social-hub/extensions/video"
	"social-hub/pkg/socialhub"
)

func TestDirectVideoWorkflow(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/video/upload/":
			if err := request.ParseMultipartForm(1 << 20); err != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			file, _, _ := request.FormFile("video")
			data, _ := io.ReadAll(file)
			_ = file.Close()
			if string(data) != "video" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"data":{"video":{"video_id":"video-1","width":720,"height":1280},"error_code":0}}`))
		case "/video/create/":
			_, _ = writer.Write([]byte(`{"data":{"item_id":"item-1","error_code":0}}`))
		case "/video/data/":
			_, _ = writer.Write([]byte(`{"data":{"list":[` + videoFixture + `],"error_code":0}}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	workflow := client.VideoWorkflow()
	session, err := workflow.Create(context.Background(), video.CreateRequest{Filename: "video.mp4", MIME: "video/mp4", Size: 5})
	if err != nil {
		t.Fatal(err)
	}
	if err := workflow.Upload(context.Background(), session.ID, bytes.NewBufferString("video"), 5); err != nil {
		t.Fatal(err)
	}
	if err := workflow.Complete(context.Background(), session.ID); err != nil {
		t.Fatal(err)
	}
	job, err := workflow.Publish(context.Background(), session.ID, video.PublishRequest{Description: "demo"})
	if err != nil || job.State != video.StatePublishPending {
		t.Fatalf("job=%#v err=%v", job, err)
	}
	job, err = workflow.Status(context.Background(), job.ID)
	if err != nil || job.State != video.StatePublished {
		t.Fatalf("status=%#v err=%v", job, err)
	}
}

func TestMultipartVideoRequestShape(t *testing.T) {
	t.Parallel()
	var initCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/video/part/init/":
			initCalls.Add(1)
			_, _ = writer.Write([]byte(`{"data":{"upload_id":"large-upload","error_code":0}}`))
		case "/video/part/upload/":
			partNumber, _ := strconv.Atoi(request.URL.Query().Get("part_number"))
			if request.URL.Query().Get("upload_id") != "chunk-fixture" || partNumber < 1 || partNumber > 2 {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			if err := request.ParseMultipartForm(6 << 20); err != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			file, _, _ := request.FormFile("video")
			_, _ = io.Copy(io.Discard, file)
			_ = file.Close()
			_, _ = writer.Write([]byte(`{"data":{"error_code":0}}`))
		case "/video/part/complete/":
			if request.URL.Query().Get("upload_id") != "chunk-fixture" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"data":{"video":{"video_id":"video-large"},"error_code":0}}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	large, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "large.mp4", Type: socialhub.MediaTypeVideo, MIME: "video/mp4", Size: directUploadThreshold + 1})
	if err != nil || large.ID != "large-upload" || large.PartSize != defaultPartSize || initCalls.Load() != 1 {
		t.Fatalf("large session=%#v calls=%d err=%v", large, initCalls.Load(), err)
	}

	const fixtureSize = 10 << 20
	client.uploadMu.Lock()
	client.uploads["chunk-fixture"] = &uploadState{request: socialhub.BeginUploadRequest{Filename: "chunk.mp4", Type: socialhub.MediaTypeVideo, MIME: "video/mp4", Size: fixtureSize}, chunked: true, parts: make(map[int]socialhub.UploadedPart), uploading: make(map[int]bool)}
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
	if err != nil || media.ID != "video-large" {
		t.Fatalf("media=%#v err=%v", media, err)
	}
}
