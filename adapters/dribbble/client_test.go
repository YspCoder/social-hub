package dribbble

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestFetchPaginationMappingAndRateLimit(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" || request.Header.Get("Accept") != textMediaType {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		writer.Header().Set("X-RateLimit-Limit", "60")
		writer.Header().Set("X-RateLimit-Remaining", "59")
		writer.Header().Set("X-RateLimit-Reset", "1785628860")
		switch request.URL.Path {
		case "/v2/user":
			writeJSON(writer, http.StatusOK, `{"id":1,"name":"Alice &amp; Design","login":"alice","html_url":"https://dribbble.com/alice","avatar_url":"https://cdn.example/avatar.png","can_upload_shot":true,"pro":true}`)
		case "/v2/shots/10":
			writeJSON(writer, http.StatusOK, shotJSON(10, "A typed shot"))
		case "/v2/user/shots":
			if request.URL.Query().Get("page") != "2" || request.URL.Query().Get("per_page") != "100" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writer.Header().Set("Link", `<`+server.URL+`/v2/user/shots?page=1&per_page=100>; rel="prev", <`+server.URL+`/v2/user/shots?page=3&per_page=100>; rel="next", <https://evil.example/v2/user/shots?page=9>; rel="next"`)
			writeJSON(writer, http.StatusOK, `[`+shotJSON(11, "Listed shot")+`]`)
		case "/v2/shots/99":
			writer.Header().Set("X-RateLimit-Remaining", "0")
			writeJSON(writer, http.StatusTooManyRequests, `{"message":"API rate limit exceeded."}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, []string{"public", "upload"})
	user, err := client.GetUser(context.Background(), "1")
	if err != nil || user.ID != "1" || user.DisplayName == nil || *user.DisplayName != "Alice & Design" || user.AccountType == nil || *user.AccountType != "player" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	post, err := client.GetPost(context.Background(), "10")
	if err != nil || post.ID != "10" || post.Text == nil || *post.Text != "A typed shot" || len(post.Media) != 1 || post.Media[0].Type != socialhub.MediaTypeImage {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "2", MaxResults: 150})
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil || *page.NextCursor != "3" || page.PrevCursor == nil || *page.PrevCursor != "1" || !page.HasMore {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "10"}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("comments error=%v", err)
	}
	rate := client.RateLimit()
	if rate.Limit != 60 || rate.Remaining != 59 || !rate.ResetAt.Equal(time.Unix(1785628860, 0)) || !rate.ObservedAt.Equal(time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("rate=%#v", rate)
	}
	if _, err := client.GetPost(context.Background(), "99"); !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("rate error=%v", err)
	} else {
		var typed *socialhub.Error
		if !errors.As(err, &typed) || typed.RetryAfter != time.Minute || typed.PlatformMessage != "API rate limit exceeded." {
			t.Fatalf("typed error=%#v", typed)
		}
	}
}

func TestFetchValidationAndVideoMapping(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, []string{"upload"})
	invalid := []func() error{
		func() error { _, err := client.GetUser(context.Background(), "2"); return err },
		func() error { _, err := client.GetPost(context.Background(), "bad"); return err },
		func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "2"})
			return err
		},
		func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "0"})
			return err
		},
	}
	for index, call := range invalid {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) && !errors.Is(err, socialhub.ErrApprovalRequired) {
			t.Fatalf("call %d error=%v", index, err)
		}
	}
	shot := Shot{ID: 8, DescriptionText: "video", User: &User{ID: 1}, Video: &Video{ID: 9, Duration: 3, Filename: "demo.webm", Size: 20, Width: 800, Height: 600, URL: "https://cdn.example/demo.webm"}}
	post := mapShot("designer", shot, time.Now())
	if len(post.Media) != 1 || post.Media[0].Type != socialhub.MediaTypeVideo || post.Media[0].MIME != "video/webm" || post.AuthorID == nil || *post.AuthorID != "1" {
		t.Fatalf("video post=%#v", post)
	}
}

func shotJSON(id int, description string) string {
	return `{"id":` + strconv.Itoa(id) + `,"title":"Demo","description_text":"` + description + `","width":800,"height":600,"images":{"normal":"https://cdn.example/shot.png"},"published_at":"2026-08-02T00:00:00Z","updated_at":"2026-08-02T00:01:00Z","html_url":"https://dribbble.com/shots/` + strconv.Itoa(id) + `","user":{"id":1},"views_count":10,"likes_count":2}`
}
