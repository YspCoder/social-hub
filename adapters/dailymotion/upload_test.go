package dailymotion

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestVideoUploadWorkflow(t *testing.T) {
	var uploadCalls, publishCalls atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v2/files/upload_sessions":
			if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer access-token" {
				http.Error(writer, "initialize", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusCreated, `{"upload_url":`+quoted(server.URL+"/upload")+`,"progress_url":`+quoted(server.URL+"/progress")+`}`)
		case "/upload":
			if request.Method != http.MethodPost || request.Header.Get("Authorization") != "" || request.Header.Get("X-Request-ID") != "upload-request" {
				http.Error(writer, "upload headers", http.StatusBadRequest)
				return
			}
			file, header, err := request.FormFile("file")
			if err != nil {
				http.Error(writer, "multipart", http.StatusBadRequest)
				return
			}
			defer file.Close()
			body, err := io.ReadAll(file)
			if err != nil || header.Filename != "clip.mp4" || string(body) != "data" {
				http.Error(writer, "file", http.StatusBadRequest)
				return
			}
			uploadCalls.Add(1)
			writeJSON(writer, http.StatusOK, `{"url":`+quoted(server.URL+"/source/clip.mp4")+`,"name":"clip.mp4","format":"mp4","duration":"1.5","size":"4","hash":"sha256"}`)
		case "/v2/profiles/profile-1/videos":
			if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer access-token" {
				http.Error(writer, "publish headers", http.StatusBadRequest)
				return
			}
			var body struct {
				Source struct {
					FileURL string `json:"file_url"`
				} `json:"source"`
			}
			if json.NewDecoder(request.Body).Decode(&body) != nil || body.Source.FileURL != server.URL+"/source/clip.mp4" {
				http.Error(writer, "publish body", http.StatusBadRequest)
				return
			}
			publishCalls.Add(1)
			writeJSON(writer, http.StatusCreated, videoResponse)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	_, client := newTestClient(t, server)
	workflow := client.VideoUploadWorkflow()
	session, err := workflow.Initialize(context.Background(), "clip.mp4", 4)
	if err != nil || session.ID == "" || session.Filename != "clip.mp4" || session.Size != 4 || session.ProgressURL != server.URL+"/progress" {
		t.Fatalf("session=%#v err=%v", session, err)
	}
	uploaded, err := workflow.Upload(context.Background(), session.ID, strings.NewReader("data"), socialhub.WithRequestID("upload-request"))
	if err != nil || uploaded.Name != "clip.mp4" || uploaded.Size != "4" || uploaded.Checksum != "sha256" {
		t.Fatalf("uploaded=%#v err=%v", uploaded, err)
	}
	if _, err := workflow.Upload(context.Background(), session.ID, strings.NewReader("data")); errorCode(err) != socialhub.CodeConflict {
		t.Fatalf("duplicate upload=%v", err)
	}
	video, err := workflow.Publish(context.Background(), session.ID, CreateVideoRequest{Title: "Launch", Category: "tech", Visibility: "public"})
	if err != nil || video.VideoID != "video-1" || uploadCalls.Load() != 1 || publishCalls.Load() != 1 {
		t.Fatalf("video=%#v uploads=%d publishes=%d err=%v", video, uploadCalls.Load(), publishCalls.Load(), err)
	}
	if err := workflow.Abort(session.ID); errorCode(err) != socialhub.CodeNotFound {
		t.Fatalf("abort published session=%v", err)
	}
}

func TestVideoUploadSizeAndStateValidation(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v2/files/upload_sessions":
			writeJSON(writer, http.StatusCreated, `{"upload_url":`+quoted(server.URL+"/upload")+`,"progress_url":`+quoted(server.URL+"/progress")+`}`)
		case "/upload":
			_, _ = io.Copy(io.Discard, request.Body)
			writeJSON(writer, http.StatusOK, `{"url":`+quoted(server.URL+"/source")+`,"name":"clip.mp4"}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	workflow := client.VideoUploadWorkflow()

	for _, test := range []struct {
		name   string
		size   int64
		reader string
	}{
		{name: "short", size: 4, reader: "abc"},
		{name: "long", size: 3, reader: "abcd"},
	} {
		t.Run(test.name, func(t *testing.T) {
			session, err := workflow.Initialize(context.Background(), test.name+".mp4", test.size)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := workflow.Upload(context.Background(), session.ID, strings.NewReader(test.reader)); errorCode(err) != socialhub.CodeInvalidArgument {
				t.Fatalf("size mismatch=%v", err)
			}
			if err := workflow.Abort(session.ID); err != nil {
				t.Fatalf("abort retryable session=%v", err)
			}
		})
	}

	session, err := workflow.Initialize(context.Background(), "state.mp4", 4)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workflow.Publish(context.Background(), session.ID, CreateVideoRequest{}); errorCode(err) != socialhub.CodeConflict {
		t.Fatalf("publish before upload=%v", err)
	}
	if err := workflow.Abort(session.ID); err != nil {
		t.Fatal(err)
	}
	if err := workflow.Abort(session.ID); errorCode(err) != socialhub.CodeNotFound {
		t.Fatalf("duplicate abort=%v", err)
	}
	if _, err := workflow.Upload(context.Background(), "missing", strings.NewReader("data")); errorCode(err) != socialhub.CodeNotFound {
		t.Fatalf("missing session=%v", err)
	}
	if _, err := workflow.Upload(context.Background(), "", nil); errorCode(err) != socialhub.CodeInvalidArgument {
		t.Fatalf("invalid upload=%v", err)
	}
	if _, err := workflow.Upload(context.Background(), "missing", strings.NewReader("data"), socialhub.WithCallTimeout(-time.Second)); errorCode(err) != socialhub.CodeInvalidArgument {
		t.Fatalf("invalid call option=%v", err)
	}
	if _, err := workflow.Initialize(context.Background(), "../clip.mp4", 4); errorCode(err) != socialhub.CodeInvalidArgument {
		t.Fatalf("unsafe filename=%v", err)
	}
	if _, err := workflow.Initialize(context.Background(), "clip.mp4", maxVideoSize+1); errorCode(err) != socialhub.CodeInvalidArgument {
		t.Fatalf("oversize initialization=%v", err)
	}
	if _, err := workflow.Publish(context.Background(), "missing", CreateVideoRequest{SourceURL: "https://example.com/video.mp4"}); errorCode(err) != socialhub.CodeInvalidArgument {
		t.Fatalf("caller supplied source=%v", err)
	}
	if err := workflow.Abort(""); errorCode(err) != socialhub.CodeInvalidArgument {
		t.Fatalf("invalid abort=%v", err)
	}
	if !validDailymotionUploadURL("https://upload.dailymotion.com/file", client.apiBaseURL) || validDailymotionUploadURL("http://upload.dailymotion.com/file", client.apiBaseURL) {
		t.Fatal("Dailymotion upload origin validation")
	}
}

func TestVideoUploadOriginRedirectAndEarlyRejection(t *testing.T) {
	t.Run("foreign origin", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writeJSON(writer, http.StatusCreated, `{"upload_url":"https://evil.example/upload","progress_url":"https://evil.example/progress"}`)
		}))
		defer server.Close()
		_, client := newTestClient(t, server)
		if _, err := client.VideoUploadWorkflow().Initialize(context.Background(), "clip.mp4", 4); errorCode(err) != socialhub.CodePlatformError {
			t.Fatalf("foreign origin=%v", err)
		}
	})

	t.Run("redirect is not followed", func(t *testing.T) {
		var redirected atomic.Int32
		var server *httptest.Server
		server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/v2/files/upload_sessions":
				writeJSON(writer, http.StatusCreated, `{"upload_url":`+quoted(server.URL+"/redirect")+`,"progress_url":`+quoted(server.URL+"/progress")+`}`)
			case "/redirect":
				http.Redirect(writer, request, server.URL+"/redirected", http.StatusTemporaryRedirect)
			case "/redirected":
				redirected.Add(1)
				writer.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()
		_, client := newTestClient(t, server)
		session, err := client.VideoUploadWorkflow().Initialize(context.Background(), "clip.mp4", 4)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.VideoUploadWorkflow().Upload(context.Background(), session.ID, strings.NewReader("data")); err == nil || redirected.Load() != 0 {
			t.Fatalf("redirect err=%v followed=%d", err, redirected.Load())
		}
	})

	t.Run("early rejection releases writer", func(t *testing.T) {
		var server *httptest.Server
		server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/v2/files/upload_sessions":
				writeJSON(writer, http.StatusCreated, `{"upload_url":`+quoted(server.URL+"/reject")+`,"progress_url":`+quoted(server.URL+"/progress")+`}`)
			case "/reject":
				writer.WriteHeader(http.StatusRequestEntityTooLarge)
			}
		}))
		defer server.Close()
		_, client := newTestClient(t, server)
		const size = int64(8 << 20)
		session, err := client.VideoUploadWorkflow().Initialize(context.Background(), "large.mp4", size)
		if err != nil {
			t.Fatal(err)
		}
		reader := io.LimitReader(repeatingReader{}, size)
		if _, err := client.VideoUploadWorkflow().Upload(context.Background(), session.ID, reader, socialhub.WithCallTimeout(2*time.Second)); errorCode(err) != socialhub.CodeInvalidArgument {
			t.Fatalf("early rejection=%v", err)
		}
	})
}

type repeatingReader struct{}

func (repeatingReader) Read(target []byte) (int, error) {
	for index := range target {
		target[index] = 'x'
	}
	return len(target), nil
}

func quoted(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
