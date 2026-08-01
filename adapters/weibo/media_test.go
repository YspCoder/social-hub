package weibo

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestPictureUploadWorkflow(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/2/statuses/upload_pic.json" || request.URL.Query().Get("access_token") != "access-token" {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		file, _, err := request.FormFile("pic")
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		data, _ := io.ReadAll(file)
		_ = file.Close()
		if string(data) != "image" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = writer.Write([]byte(`{"pic_id":"pic-1","original_pic":"https://img.example/original.jpg"}`))
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	session, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "image.jpg", Type: socialhub.MediaTypeImage, MIME: "image/jpeg", Size: 5})
	if err != nil {
		t.Fatal(err)
	}
	part, err := client.UploadPart(context.Background(), session.ID, 0, bytes.NewBufferString("image"))
	if err != nil {
		t.Fatal(err)
	}
	media, err := client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part})
	if err != nil || media.ID != "pic-1" || media.State != socialhub.MediaStateReady {
		t.Fatalf("media=%#v err=%v", media, err)
	}
	status, err := client.MediaStatus(context.Background(), media.ID)
	if err != nil || status.URL != "https://img.example/original.jpg" {
		t.Fatalf("status=%#v err=%v", status, err)
	}
}
