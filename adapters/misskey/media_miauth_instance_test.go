package misskey

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestDriveUploadStatusAndInstanceContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer access-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/api/drive/files/create":
			if err := request.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("parse multipart: %v", err)
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			file, header, err := request.FormFile("file")
			if err != nil {
				t.Errorf("form file: %v", err)
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			defer file.Close()
			body, _ := io.ReadAll(file)
			if header.Filename != "clip.mp4" || header.Header.Get("Content-Type") != "video/mp4" || string(body) != "video-data" ||
				request.FormValue("folderId") != "folder-1" || request.FormValue("comment") != "alt text" ||
				request.FormValue("isSensitive") != "true" || request.FormValue("force") != "true" {
				t.Errorf("multipart filename=%q type=%q body=%q form=%v", header.Filename, header.Header.Get("Content-Type"), body, request.Form)
			}
			writeTestJSON(t, writer, testDriveFile("file-1", "video/mp4"))
		case "/api/drive/files/show":
			var body map[string]string
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body["fileId"] != "file-1" {
				t.Errorf("show body=%v err=%v", body, err)
			}
			writeTestJSON(t, writer, testDriveFile("file-1", "video/mp4"))
		case "/api/meta":
			var body map[string]bool
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil || !body["detail"] {
				t.Errorf("meta body=%v err=%v", body, err)
			}
			writeTestJSON(t, writer, map[string]any{
				"name": "Example Misskey", "shortName": "Example", "version": "2025.12.2",
				"description": "federated", "uri": "social.example.test", "disableLocalTimeline": true,
				"disableGlobalTimeline": false, "mediaProxy": "https://social.example.test/proxy",
			})
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, allTestPermissions())
	request := DriveUploadRequest{
		Upload:   socialhub.BeginUploadRequest{Filename: "clip.mp4", Type: socialhub.MediaTypeVideo, MIME: "video/mp4", Size: int64(len("video-data"))},
		FolderID: "folder-1", Comment: "alt text", Sensitive: true, Force: true,
	}
	session, err := client.BeginDriveUpload(context.Background(), request)
	if err != nil || session.ID == "" || session.PartSize != request.Upload.Size {
		t.Fatalf("session=%#v err=%v", session, err)
	}
	part, err := client.UploadPart(context.Background(), session.ID, 1, strings.NewReader("video-data"))
	if err != nil || part.Number != 1 || part.Size != request.Upload.Size {
		t.Fatalf("part=%#v err=%v", part, err)
	}
	if _, err := client.UploadPart(context.Background(), session.ID, 1, strings.NewReader("video-data")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("repeat part=%v", err)
	}
	media, err := client.CompleteUpload(context.Background(), session.ID, []socialhub.UploadedPart{*part})
	if err != nil || media.ID != "file-1" || media.Type != socialhub.MediaTypeVideo || media.State != socialhub.MediaStateReady || media.Width == nil || *media.Width != 640 {
		t.Fatalf("media=%#v err=%v", media, err)
	}
	status, err := client.MediaStatus(context.Background(), media.ID)
	if err != nil || status.ID != media.ID || status.URL == "" {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	instance, err := client.Instance(context.Background())
	if err != nil || instance.Name != "Example Misskey" || !instance.DisableLocalTimeline || instance.MediaProxy == "" {
		t.Fatalf("instance=%#v err=%v", instance, err)
	}
	commonSession, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{
		Filename: "image.png", Type: socialhub.MediaTypeImage, MIME: "image/png", Size: 1,
	})
	if err != nil || commonSession.ID == "" {
		t.Fatalf("common session=%#v err=%v", commonSession, err)
	}
}

func TestDriveValidationAndMediaMapping(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, allTestPermissions())
	invalid := []DriveUploadRequest{
		{},
		{Upload: socialhub.BeginUploadRequest{Filename: "x", Type: socialhub.MediaTypeImage, MIME: "bad", Size: 1}},
		{Upload: socialhub.BeginUploadRequest{Filename: "x", Type: "other", MIME: "image/png", Size: 1}},
		{Upload: socialhub.BeginUploadRequest{Filename: "x", Type: socialhub.MediaTypeImage, MIME: "image/png", Size: 1}, FolderID: " bad"},
		{Upload: socialhub.BeginUploadRequest{Filename: "x", Type: socialhub.MediaTypeImage, MIME: "image/png", Size: 1}, Comment: "bad\x00"},
	}
	for index, input := range invalid {
		if _, err := client.BeginDriveUpload(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid upload %d=%v", index, err)
		}
	}
	if _, err := client.UploadPart(context.Background(), "missing", 2, strings.NewReader("x")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid part=%v", err)
	}
	if _, err := client.UploadPart(context.Background(), "missing", 1, nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("nil part=%v", err)
	}
	if _, err := client.CompleteUpload(context.Background(), "missing", nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid complete=%v", err)
	}
	if _, err := client.MediaStatus(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid status=%v", err)
	}

	for mimeType, want := range map[string]socialhub.MediaType{
		"image/gif": socialhub.MediaTypeAnimation, "image/jpeg": socialhub.MediaTypeImage,
		"audio/mpeg": socialhub.MediaTypeAudio, "application/pdf": socialhub.MediaTypeDocument,
	} {
		encoded, _ := json.Marshal(testDriveFile("file", mimeType))
		var file misskeyDriveFile
		_ = json.Unmarshal(encoded, &file)
		media, err := mapDriveFile(file)
		if err != nil || media.Type != want {
			t.Fatalf("MIME %s media=%#v err=%v", mimeType, media, err)
		}
	}
	if _, err := mapDriveFile(misskeyDriveFile{}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("invalid media=%v", err)
	}
}

func TestMiAuthAuthorizationAndCheck(t *testing.T) {
	const session = "c1f6d42b-468b-4fd2-8274-e58abdedef6f"
	checks := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/api/miauth/"+session+"/check" || request.Header.Get("Authorization") != "" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		checks++
		if checks == 1 {
			writeTestJSON(t, writer, map[string]any{"ok": false})
			return
		}
		writeTestJSON(t, writer, map[string]any{"ok": true, "token": "new-access-token", "user": testUser("user-1")})
	}))
	defer server.Close()
	adapter, _ := newTestClient(t, server, allTestPermissions())
	miauth, err := adapter.MiAuth("main")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := miauth.AuthorizationURL(MiAuthRequest{
		Session: session, Name: "social-hub", IconURL: "https://app.example.test/icon.png",
		CallbackURL: "https://app.example.test/callback", Permissions: []string{"write:notes", "read:drive"},
	})
	parsed, parseErr := url.Parse(authorizationURL)
	if err != nil || parseErr != nil || parsed.Path != "/miauth/"+session || parsed.Query().Get("name") != "social-hub" ||
		parsed.Query().Get("icon") == "" || parsed.Query().Get("callback") == "" || parsed.Query().Get("permission") != "write:notes,read:drive" {
		t.Fatalf("authorization URL=%q err=%v parse=%v", authorizationURL, err, parseErr)
	}
	denied, err := miauth.Check(context.Background(), session)
	if err != nil || denied.OK || denied.AccessToken != "" || denied.User != nil {
		t.Fatalf("denied=%#v err=%v", denied, err)
	}
	approved, err := miauth.Check(context.Background(), session)
	if err != nil || !approved.OK || approved.AccessToken != "new-access-token" || approved.User == nil || approved.User.ID != "user-1" {
		t.Fatalf("approved=%#v err=%v", approved, err)
	}
}

func TestMiAuthAndInstanceValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/miauth/c1f6d42b-468b-4fd2-8274-e58abdedef6f/check":
			writeTestJSON(t, writer, map[string]any{"ok": true})
		case "/api/meta":
			writeTestJSON(t, writer, map[string]any{"name": "", "version": ""})
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	adapter, client := newTestClient(t, server, allTestPermissions())
	miauth, err := adapter.MiAuth("main")
	if err != nil {
		t.Fatal(err)
	}
	validSession := "c1f6d42b-468b-4fd2-8274-e58abdedef6f"
	invalid := []MiAuthRequest{
		{},
		{Session: validSession, Name: "bad\n"},
		{Session: validSession, IconURL: "file:///icon"},
		{Session: validSession, CallbackURL: "javascript:alert(1)"},
		{Session: validSession, Permissions: []string{"read:drive", "read:drive"}},
		{Session: validSession, Permissions: []string{"read:drive,write:drive"}},
	}
	for index, input := range invalid {
		if _, err := miauth.AuthorizationURL(input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid MiAuth URL %d=%v", index, err)
		}
	}
	if _, err := miauth.Check(context.Background(), "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid check=%v", err)
	}
	if _, err := miauth.Check(context.Background(), validSession); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("malformed check=%v", err)
	}
	if _, err := client.Instance(context.Background()); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("malformed instance=%v", err)
	}
}
