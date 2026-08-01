package linkedin

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestLinkedInImageUploadContract(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/rest/images":
			if request.URL.Query().Get("action") != "initializeUpload" || request.Header.Get("Linkedin-Version") != "202607" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"value":{"uploadUrl":"`+server.URL+`/upload","uploadUrlExpiresAt":1785546000000,"image":"urn:li:image:image-1"}}`)
		case request.Method == http.MethodPut && request.URL.Path == "/upload":
			body, _ := io.ReadAll(request.Body)
			if request.Header.Get("Authorization") != "Bearer access-token" || request.Header.Get("Content-Type") != "image/png" || request.Header.Get("X-Request-ID") != "upload-request" || string(body) != "data" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writer.WriteHeader(http.StatusCreated)
		case request.Method == http.MethodGet && request.URL.Path == "/rest/images/urn:li:image:image-1":
			writeJSON(writer, `{"id":"urn:li:image:image-1","status":"AVAILABLE","downloadUrl":"https://cdn.example/image.png","downloadUrlExpiresAt":1785546000000}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{"w_organization_social"})
	session, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "image.png", Type: socialhub.MediaTypeImage, MIME: "image/png", Size: 4})
	if err != nil || session.ID != "urn:li:image:image-1" || session.PartSize != 4 {
		t.Fatalf("session=%#v err=%v", session, err)
	}
	part, err := client.UploadPart(context.Background(), session.ID, 0, strings.NewReader("data"), socialhub.WithRequestID("upload-request"))
	if err != nil || part.Size != 4 {
		t.Fatalf("part=%#v err=%v", part, err)
	}
	media, err := client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part})
	if err != nil || media.State != socialhub.MediaStateProcessing {
		t.Fatalf("media=%#v err=%v", media, err)
	}
	status, err := client.MediaStatus(context.Background(), session.ID)
	if err != nil || status.State != socialhub.MediaStateReady || status.URL != "https://cdn.example/image.png" {
		t.Fatalf("status=%#v err=%v", status, err)
	}
}
