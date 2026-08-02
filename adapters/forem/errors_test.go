package forem

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestHTTPErrorMapping(t *testing.T) {
	header := http.Header{"Retry-After": {"2.5"}, "X-Request-Id": {"request-1"}}
	err := decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{"error":"Rate limit reached","status":429}`))
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeRateLimited || !platformErr.Retryable() || platformErr.RetryAfter != 2500*time.Millisecond || platformErr.PlatformCode != "429" || platformErr.PlatformMessage != "Rate limit reached" || platformErr.RequestID != "request-1" {
		t.Fatalf("rate error=%#v", platformErr)
	}
	statuses := map[int]socialhub.ErrorCode{
		http.StatusBadRequest: socialhub.CodeInvalidArgument, http.StatusUnauthorized: socialhub.CodeUnauthenticated,
		http.StatusForbidden: socialhub.CodePermissionDenied, http.StatusNotFound: socialhub.CodeNotFound,
		http.StatusConflict: socialhub.CodeConflict, http.StatusInternalServerError: socialhub.CodeTemporarilyUnavailable,
		http.StatusTeapot: socialhub.CodePlatformError,
	}
	for status, want := range statuses {
		code, _ := classifyError(status)
		if code != want {
			t.Fatalf("status %d code=%s want=%s", status, code, want)
		}
	}
	if parseRetryAfter("bad") != 0 || parseRetryAfter("-1") != 0 || parseRetryAfter("90000") != 0 || parseRetryAfter("1.5") != 1500*time.Millisecond {
		t.Fatal("Retry-After parsing failed")
	}
}

func TestWireDecodersAndMappingFallbacks(t *testing.T) {
	var list stringList
	if err := json.Unmarshal([]byte(`"go, sdk"`), &list); err != nil || len(list) != 2 || list[1] != "sdk" {
		t.Fatalf("string tags=%v err=%v", list, err)
	}
	if err := json.Unmarshal([]byte(`["go","","sdk"]`), &list); err != nil || len(list) != 2 {
		t.Fatalf("array tags=%v err=%v", list, err)
	}
	if err := json.Unmarshal([]byte(`{}`), &list); err == nil {
		t.Fatal("object tags must fail")
	}
	for index, target := range []json.Unmarshaler{&wireUser{}, &wireArticle{}, &wireComment{}} {
		if err := target.UnmarshalJSON([]byte(`{`)); err == nil {
			t.Fatalf("decoder %d accepted invalid JSON", index)
		}
	}

	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server)
	user := client.mapUser(wireUser{UserID: 9, Username: "nested", Name: "Nested", ProfileImage90: "/avatar.png", Raw: json.RawMessage(`{"user_id":9}`)})
	if user.ID != "9" || user.AvatarURL == nil || *user.AvatarURL != server.URL+"/avatar.png" {
		t.Fatalf("mapped user=%#v", user)
	}
	draft := client.mapArticle(wireArticle{ID: 1, Title: "Draft", Description: "Summary", CoverImage: "/cover.png", Tags: stringList{"one"}, Raw: json.RawMessage(`{"id":1}`)})
	if draft.Published || draft.Post.Status.State != socialhub.PublishStatePending || draft.Post.Text == nil || *draft.Post.Text != "Summary" || len(draft.Tags) != 1 || len(draft.Post.Media) != 1 || draft.Post.Media[0].URL != server.URL+"/cover.png" {
		t.Fatalf("draft=%#v", draft)
	}
	invalidMedia := client.mapPost(wireArticle{ID: 2, Title: "Invalid media", CoverImage: "data:image/png;base64,eA=="})
	if len(invalidMedia.Media) != 0 {
		t.Fatalf("invalid media=%#v", invalidMedia.Media)
	}
	if client.absoluteURL("") != nil || client.absoluteURL(":bad") != nil || stringPointer("") != nil {
		t.Fatal("URL and pointer fallbacks failed")
	}
}

func TestBadResponsesAndTransportErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method + " " + request.URL.Path {
		case "GET /api/users/me":
			writeJSON(writer, http.StatusOK, `{"id":0}`)
		case "GET /api/articles/1":
			writeJSON(writer, http.StatusOK, articleFixture(2, true, "Wrong"))
		case "GET /api/articles/2":
			writeJSON(writer, http.StatusOK, `{`)
		case "GET /api/articles/3":
			writer.Header().Set("Retry-After", "1")
			writeJSON(writer, http.StatusTooManyRequests, `{"error":"slow","status":429}`)
		case "GET /api/articles/me":
			writeJSON(writer, http.StatusOK, `[{"id":0}]`)
		case "GET /api/comments":
			writeJSON(writer, http.StatusOK, `[{"id_code":"bad id"}]`)
		case "POST /api/articles":
			writeJSON(writer, http.StatusCreated, `{"id":0}`)
		case "PUT /api/articles/1":
			writeJSON(writer, http.StatusOK, articleFixture(2, true, "Wrong"))
		case "GET /api/articles/me/all":
			writeJSON(writer, http.StatusOK, `[{"id":0}]`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	badCalls := []func() error{
		func() error { _, err := client.GetUser(context.Background(), ""); return err },
		func() error { _, err := client.GetPost(context.Background(), "1"); return err },
		func() error { _, err := client.GetPost(context.Background(), "2"); return err },
		func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{})
			return err
		},
		func() error {
			_, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "1"})
			return err
		},
		func() error {
			_, err := client.CreateArticle(context.Background(), CreateArticleRequest{Title: "x", BodyMarkdown: "x"})
			return err
		},
		func() error {
			title := "x"
			_, err := client.UpdateArticle(context.Background(), "1", UpdateArticleRequest{Title: &title})
			return err
		},
		func() error {
			_, err := client.ListMyArticles(context.Background(), ArticleStateAll, "", 1)
			return err
		},
	}
	for index, call := range badCalls {
		var platformErr *socialhub.Error
		if err := call(); !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodePlatformError {
			t.Fatalf("bad response %d error=%v", index, err)
		}
	}
	if _, err := client.GetPost(context.Background(), "3"); !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("rate error=%v", err)
	}
	if err := client.requestJSON(context.Background(), http.MethodPost, "/api/articles", nil, func() {}, nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("marshal error=%v", err)
	}
}

func TestArticleValidationHelpers(t *testing.T) {
	if !validHTTPURL("https://example.com/image.png") || validHTTPURL("ftp://example.com/file") || validHTTPURL("https://user:pass@example.com") {
		t.Fatal("HTTP URL validation failed")
	}
	if optionalString("") != nil || optionalString("x") == nil {
		t.Fatal("optional string failed")
	}
	if id, err := optionalID("9", "test"); err != nil || id == nil || *id != 9 {
		t.Fatalf("optional ID=%v err=%v", id, err)
	}
	if id, err := optionalID("", "test"); err != nil || id != nil {
		t.Fatalf("empty optional ID=%v err=%v", id, err)
	}
	if tags, err := encodeTags(nil, true); err != nil || tags == nil || *tags != "" {
		t.Fatalf("empty tags=%v err=%v", tags, err)
	}
	if _, err := encodeTags([]string{"bad,tag"}, false); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad tag=%v", err)
	}
	tooLong := strings.Repeat("x", 1025)
	if err := validateOptionalMetadataPointers(&tooLong, nil, nil, nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("long series=%v", err)
	}
}
