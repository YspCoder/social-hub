package flickr

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestRESTErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		photoID := request.URL.Query().Get("photo_id")
		switch photoID {
		case "missing":
			writeJSON(writer, http.StatusOK, `{"stat":"fail","code":1,"message":"Photo not found"}`)
		case "limited":
			writer.Header().Set("Retry-After", "7")
			writer.Header().Set("X-Request-ID", "flickr-request")
			writeJSON(writer, http.StatusTooManyRequests, `{"stat":"fail","code":0,"message":"slow down"}`)
		case "bad-json":
			writeJSON(writer, http.StatusOK, `{`)
		case "mismatch":
			writeJSON(writer, http.StatusOK, `{"stat":"ok","photo":{"id":"other"}}`)
		case "redirect":
			http.Redirect(writer, request, "/target", http.StatusFound)
		default:
			writeJSON(writer, http.StatusInternalServerError, `{"stat":"fail","code":105,"message":"service unavailable"}`)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)

	tests := []struct {
		photoID string
		code    socialhub.ErrorCode
	}{
		{"missing", socialhub.CodeNotFound},
		{"bad-json", socialhub.CodePlatformError},
		{"mismatch", socialhub.CodePlatformError},
		{"redirect", socialhub.CodePlatformError},
		{"server", socialhub.CodeTemporarilyUnavailable},
	}
	for _, test := range tests {
		t.Run(test.photoID, func(t *testing.T) {
			_, err := client.GetPhoto(context.Background(), test.photoID)
			if errorCode(err) != test.code {
				t.Fatalf("error=%v", err)
			}
		})
	}
	_, err := client.GetPhoto(context.Background(), "limited")
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeRateLimited || platformErr.RetryAfter != 7*time.Second || platformErr.RequestID != "flickr-request" {
		t.Fatalf("limited error=%#v", err)
	}
}

func TestWorkflowValidation(t *testing.T) {
	client := &Client{userID: "owner@N01", permission: PermissionDelete, signed: &http.Client{}, clock: fixedClock{testNow}}
	parent := "parent"
	start, end := time.Unix(20, 0), time.Unix(10, 0)
	tests := []struct {
		name string
		run  func() error
	}{
		{"get user", func() error { _, err := client.GetUser(context.Background(), "bad/id"); return err }},
		{"get photo", func() error { _, err := client.GetPhoto(context.Background(), ""); return err }},
		{"photo cursor", func() error {
			_, err := client.ListPhotos(context.Background(), PhotoListRequest{Cursor: "zero"})
			return err
		}},
		{"photo search", func() error {
			_, err := client.ListPhotos(context.Background(), PhotoListRequest{SafeSearch: 4})
			return err
		}},
		{"photo time", func() error {
			_, err := client.ListPhotos(context.Background(), PhotoListRequest{StartTime: &start, EndTime: &end})
			return err
		}},
		{"update empty", func() error { return client.UpdatePhoto(context.Background(), "photo", UpdatePhotoRequest{}) }},
		{"update id", func() error {
			value := "x"
			return client.UpdatePhoto(context.Background(), "bad/id", UpdatePhotoRequest{Title: &value})
		}},
		{"delete", func() error { return client.DeletePhoto(context.Background(), "bad/id") }},
		{"react kind", func() error {
			return client.React(context.Background(), socialhub.ReactionRequest{TargetID: "photo", Kind: socialhub.ReactionRepost})
		}},
		{"react actor", func() error {
			return client.React(context.Background(), socialhub.ReactionRequest{TargetID: "photo", ActorID: "other", Kind: socialhub.ReactionLike})
		}},
		{"remove reaction", func() error {
			return client.RemoveReaction(context.Background(), socialhub.ReactionRequest{TargetID: "", Kind: socialhub.ReactionLike})
		}},
		{"list comments", func() error {
			_, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "photo", Cursor: "2"})
			return err
		}},
		{"comment parent", func() error {
			_, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "photo", ParentID: &parent, Text: "text"})
			return err
		}},
		{"delete comment", func() error { return client.DeleteComment(context.Background(), "bad/id") }},
		{"get album", func() error { _, err := client.GetAlbum(context.Background(), "bad/id", "owner"); return err }},
		{"list albums", func() error {
			_, err := client.ListAlbums(context.Background(), AlbumListRequest{Cursor: "0"})
			return err
		}},
		{"album photos", func() error {
			_, err := client.ListAlbumPhotos(context.Background(), AlbumPhotosRequest{AlbumID: "album", Media: "audio"})
			return err
		}},
		{"create album", func() error {
			_, err := client.CreateAlbum(context.Background(), CreateAlbumRequest{PrimaryPhotoID: "photo"})
			return err
		}},
		{"add album photo", func() error { return client.AddAlbumPhoto(context.Background(), "album", "bad/id") }},
		{"remove album photo", func() error { return client.RemoveAlbumPhoto(context.Background(), "", "photo") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if code := errorCode(test.run()); code != socialhub.CodeInvalidArgument {
				t.Fatalf("code=%q", code)
			}
		})
	}

	public := &Client{}
	if err := public.DeletePhoto(context.Background(), "photo"); errorCode(err) != socialhub.CodeApprovalRequired {
		t.Fatalf("permission error=%v", err)
	}
}

func TestScalarPaginationAndErrorHelpers(t *testing.T) {
	tests := []struct {
		input string
		want  scalar
	}{
		{`"42"`, "42"},
		{`42`, "42"},
		{`true`, "true"},
		{`false`, "false"},
		{`null`, ""},
	}
	for _, test := range tests {
		var value scalar
		if err := json.Unmarshal([]byte(test.input), &value); err != nil || value != test.want {
			t.Fatalf("input=%s value=%q err=%v", test.input, value, err)
		}
	}
	var invalid scalar
	if err := json.Unmarshal([]byte(`{}`), &invalid); err == nil {
		t.Fatal("object scalar must fail")
	}
	if value, ok := scalar("42").Int64(); !ok || value != 42 || !scalar("true").Bool() || scalar("no").Bool() {
		t.Fatal("scalar conversions failed")
	}

	values, err := pageValues("page", "2", 999)
	if err != nil || values.Get("page") != "2" || values.Get("per_page") != "500" {
		t.Fatalf("values=%v err=%v", values, err)
	}
	for _, cursor := range []string{"0", "1000001", "not-a-page"} {
		if _, err := pageValues("page", cursor, 1); errorCode(err) != socialhub.CodeInvalidArgument {
			t.Fatalf("cursor=%q err=%v", cursor, err)
		}
	}
	if _, err := pageValues("page", "", -1); errorCode(err) != socialhub.CodeInvalidArgument {
		t.Fatalf("negative maximum err=%v", err)
	}
	next, previous, more, err := pageCursors("2", "3")
	if err != nil || next == nil || *next != "3" || previous == nil || *previous != "1" || !more {
		t.Fatalf("next=%v previous=%v more=%v err=%v", next, previous, more, err)
	}
	if _, _, _, err := pageCursors("3", "2"); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("pagination error=%v", err)
	}

	classifications := []struct {
		status int
		code   int
		want   socialhub.ErrorCode
	}{
		{http.StatusOK, 99, socialhub.CodeApprovalRequired},
		{http.StatusOK, 106, socialhub.CodeTemporarilyUnavailable},
		{http.StatusUnauthorized, 0, socialhub.CodeUnauthenticated},
		{http.StatusForbidden, 0, socialhub.CodePermissionDenied},
		{http.StatusGone, 0, socialhub.CodeNotFound},
		{http.StatusConflict, 0, socialhub.CodeConflict},
		{http.StatusUnprocessableEntity, 0, socialhub.CodeInvalidArgument},
	}
	for _, test := range classifications {
		if got, _ := classifyError(test.status, test.code); got != test.want {
			t.Fatalf("status=%d platform=%d got=%q want=%q", test.status, test.code, got, test.want)
		}
	}
	if parseRetryAfter("86401") != 0 || parseRetryAfter("bad") != 0 || parseRetryAfter("5") != 5*time.Second {
		t.Fatal("Retry-After parsing failed")
	}
	if got := boundedMessage(strings.Repeat("界", 4), 2); got != "界界" {
		t.Fatalf("bounded message=%q", got)
	}
}

func TestCallTransportAndResponseLimits(t *testing.T) {
	client := &Client{apiKey: "key", baseURL: "https://api.invalid/rest", public: &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network down")
	})}}
	err := client.call(context.Background(), http.MethodGet, "test.method", nil, false, nil)
	if errorCode(err) != socialhub.CodeTemporarilyUnavailable {
		t.Fatalf("transport error=%v", err)
	}

	client.public = &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(io.LimitReader(strings.NewReader(strings.Repeat("x", 1024)), 1024)), Request: request}, nil
	})}
	err = client.call(context.Background(), http.MethodGet, "test.method", nil, true, nil)
	if errorCode(err) != socialhub.CodeUnauthenticated {
		t.Fatalf("authentication error=%v", err)
	}

	client.baseURL = "://bad"
	err = client.call(context.Background(), http.MethodGet, "test.method", nil, false, nil)
	if errorCode(err) != socialhub.CodeInvalidArgument {
		t.Fatalf("URL error=%v", err)
	}
}
