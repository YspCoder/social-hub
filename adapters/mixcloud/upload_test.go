package mixcloud

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestStreamingUploadAndEdit(t *testing.T) {
	audioBytes := []byte("abcdef")
	pictureBytes := []byte("JPEG")
	publishDate := time.Date(2026, 8, 3, 12, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("access_token") != "access-token" || request.Method != http.MethodPost {
			t.Errorf("request=%s %s", request.Method, request.URL)
		}
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse multipart: %v", err)
			return
		}
		defer request.MultipartForm.RemoveAll()
		switch request.URL.Path {
		case "/upload/":
			wantFields := map[string]string{
				"name": "Night Radio", "description": "A complete session", "tags-0-tag": "house", "tags-1-tag": "radio",
				"unlisted": "false", "publish_date": "2026-08-03T04:30:00Z", "disable_comments": "true", "hide_stats": "false",
				"hosts-0-username": "guest-dj", "sections-0-chapter": "Intro", "sections-0-start_time": "0",
				"sections-1-artist": "Artist", "sections-1-song": "Track", "sections-1-start_time": "30",
			}
			assertFormValues(t, request, wantFields)
			assertFormFile(t, request, "mp3", "mix.mp3", "audio/mpeg", audioBytes)
			assertFormFile(t, request, "picture", "cover.jpg", "image/jpeg", pictureBytes)
			writeJSON(writer, http.StatusOK, `{"result":{"success":true,"key":"/sample-dj/night-radio/","message":"uploaded"}}`)
		case "/upload/sample-dj/night-radio/edit/":
			wantFields := map[string]string{
				"name": "Night Radio Updated", "description": "Updated", "tags-0-tag": "deep", "publish": "true",
				"disable_comments": "false", "hide_stats": "true", "hosts-0-username": "",
				"sections-0-artist": "Other", "sections-0-song": "Song", "sections-0-start_time": "5",
			}
			assertFormValues(t, request, wantFields)
			assertFormFile(t, request, "picture", "new.png", "image/png", pictureBytes)
			writeJSON(writer, http.StatusOK, `{"result":{"success":true,"key":"/sample-dj/night-radio/","message":"edited"}}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, "pro")
	falseValue, trueValue := false, true
	audio := &oneByteReader{data: audioBytes}
	response, err := client.Upload(context.Background(), UploadRequest{
		Name: "Night Radio", AudioFilename: "mix.mp3", AudioSize: int64(len(audioBytes)), Description: "A complete session",
		Tags: []string{"house", "radio"}, Unlisted: &falseValue, PublishDate: &publishDate,
		DisableComments: &trueValue, HideStats: &falseValue, Hosts: []string{"guest-dj"},
		Sections:        []Section{{Chapter: "Intro", StartTime: 0}, {Artist: "Artist", Song: "Track", StartTime: 30}},
		PictureFilename: "cover.jpg", PictureMIME: "image/jpeg", PictureSize: int64(len(pictureBytes)),
	}, audio, bytes.NewReader(pictureBytes))
	if err != nil || response.Result == nil || response.Result.Key != "/sample-dj/night-radio/" || audio.reads != len(audioBytes) {
		t.Fatalf("upload=%#v reads=%d err=%v", response, audio.reads, err)
	}
	name, description := "Night Radio Updated", "Updated"
	tags := []string{"deep"}
	hosts := []string{}
	sections := []Section{{Artist: "Other", Song: "Song", StartTime: 5}}
	response, err = client.Edit(context.Background(), "/sample-dj/night-radio/", EditRequest{
		Name: &name, Description: &description, Tags: &tags, Publish: true, DisableComments: &falseValue, HideStats: &trueValue,
		Hosts: &hosts, Sections: &sections, PictureFilename: "new.png", PictureMIME: "image/png", PictureSize: int64(len(pictureBytes)),
	}, bytes.NewReader(pictureBytes))
	if err != nil || response.Result == nil || response.Result.Message != "edited" {
		t.Fatalf("edit=%#v err=%v", response, err)
	}
}

func TestUploadSizeMismatchAndReaderFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		writeJSON(writer, http.StatusOK, `{"result":{"success":true,"key":"/sample-dj/test/"}}`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, "")
	_, err := client.Upload(context.Background(), UploadRequest{Name: "Test", AudioFilename: "test.mp3", AudioSize: 4}, strings.NewReader("abc"), nil)
	if !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("size mismatch=%v", err)
	}
	_, err = client.Upload(context.Background(), UploadRequest{Name: "Test", AudioFilename: "test.mp3", AudioSize: 4}, failingReader{}, nil)
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || !platformErr.Retryable() {
		t.Fatalf("reader failure=%v", err)
	}
}

func TestUploadAndEditValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, basic := newTestClient(t, server, "basic")
	valid := UploadRequest{Name: "Test", AudioFilename: "test.mp3", AudioSize: 3}
	tests := []struct {
		name    string
		input   UploadRequest
		audio   io.Reader
		picture io.Reader
		want    error
	}{
		{"missing audio", valid, nil, nil, socialhub.ErrInvalidArgument},
		{"wrong extension", func() UploadRequest { value := valid; value.AudioFilename = "test.wav"; return value }(), strings.NewReader("abc"), nil, socialhub.ErrInvalidArgument},
		{"too many tags", func() UploadRequest {
			value := valid
			value.Tags = []string{"1", "2", "3", "4", "5", "6"}
			return value
		}(), strings.NewReader("abc"), nil, socialhub.ErrInvalidArgument},
		{"long description", func() UploadRequest { value := valid; value.Description = strings.Repeat("x", 1001); return value }(), strings.NewReader("abc"), nil, socialhub.ErrInvalidArgument},
		{"picture mismatch", func() UploadRequest {
			value := valid
			value.PictureFilename = "cover.jpg"
			value.PictureMIME = "image/jpeg"
			value.PictureSize = 3
			return value
		}(), strings.NewReader("abc"), nil, socialhub.ErrInvalidArgument},
		{"invalid section", func() UploadRequest {
			value := valid
			value.Sections = []Section{{Chapter: "Intro", Artist: "Artist", Song: "Song"}}
			return value
		}(), strings.NewReader("abc"), nil, socialhub.ErrInvalidArgument},
		{"unordered sections", func() UploadRequest {
			value := valid
			value.Sections = []Section{{Chapter: "A", StartTime: 10}, {Chapter: "B", StartTime: 5}}
			return value
		}(), strings.NewReader("abc"), nil, socialhub.ErrInvalidArgument},
		{"Pro option", func() UploadRequest { value := valid; date := testNow; value.PublishDate = &date; return value }(), strings.NewReader("abc"), nil, socialhub.ErrApprovalRequired},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := basic.Upload(context.Background(), test.input, test.audio, test.picture); !errors.Is(err, test.want) {
				t.Fatalf("error=%v want=%v", err, test.want)
			}
		})
	}
	trueValue := true
	if _, err := basic.Edit(context.Background(), "/other/show/", EditRequest{}, nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("foreign edit=%v", err)
	}
	if _, err := basic.Edit(context.Background(), "/sample-dj/show/", EditRequest{}, nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty edit=%v", err)
	}
	if _, err := basic.Edit(context.Background(), "/sample-dj/show/", EditRequest{Publish: true, Unpublish: true}, nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("publish conflict=%v", err)
	}
	emptySections := []Section{}
	if _, err := basic.Edit(context.Background(), "/sample-dj/show/", EditRequest{Sections: &emptySections}, nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty sections=%v", err)
	}
	if _, err := basic.Edit(context.Background(), "/sample-dj/show/", EditRequest{DisableComments: &trueValue}, nil); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("Pro edit=%v", err)
	}
}

func assertFormValues(t *testing.T, request *http.Request, want map[string]string) {
	t.Helper()
	for name, value := range want {
		if got := request.FormValue(name); got != value {
			t.Errorf("form %s=%q want=%q", name, got, value)
		}
	}
}

func assertFormFile(t *testing.T, request *http.Request, field, filename, contentType string, want []byte) {
	t.Helper()
	file, header, err := request.FormFile(field)
	if err != nil {
		t.Errorf("form file %s: %v", field, err)
		return
	}
	defer file.Close()
	got, err := io.ReadAll(file)
	if err != nil || header.Filename != filename || header.Header.Get("Content-Type") != contentType || !bytes.Equal(got, want) {
		t.Errorf("file %s filename=%q MIME=%q body=%q err=%v", field, header.Filename, header.Header.Get("Content-Type"), got, err)
	}
}

type oneByteReader struct {
	data  []byte
	reads int
}

func (reader *oneByteReader) Read(buffer []byte) (int, error) {
	if reader.reads == len(reader.data) {
		return 0, io.EOF
	}
	buffer[0] = reader.data[reader.reads]
	reader.reads++
	return 1, nil
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("source failed") }
