package mixcloud

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestLibraryAndCommonReactions(t *testing.T) {
	seen := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("access_token") != "access-token" {
			t.Errorf("token query=%v", request.URL.Query())
		}
		seen[request.Method+" "+request.URL.Path]++
		writeJSON(writer, http.StatusOK, `{"result":{"success":true,"message":"done"}}`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, "")
	ctx := context.Background()
	key := "/other-dj/episode/"
	actions := []struct {
		name string
		call func() (*ActionResponse, error)
	}{
		{"favourite", func() (*ActionResponse, error) { return client.Favourite(ctx, key) }},
		{"unfavourite", func() (*ActionResponse, error) { return client.Unfavourite(ctx, key) }},
		{"repost", func() (*ActionResponse, error) { return client.Repost(ctx, key) }},
		{"unrepost", func() (*ActionResponse, error) { return client.Unrepost(ctx, key) }},
		{"listen later", func() (*ActionResponse, error) { return client.ListenLater(ctx, key) }},
		{"remove listen later", func() (*ActionResponse, error) { return client.RemoveListenLater(ctx, key) }},
		{"follow", func() (*ActionResponse, error) { return client.Follow(ctx, "/other-dj/") }},
		{"unfollow", func() (*ActionResponse, error) { return client.Unfollow(ctx, "other-dj") }},
	}
	for _, action := range actions {
		t.Run(action.name, func(t *testing.T) {
			response, err := action.call()
			if err != nil || response.Result == nil || !response.Result.Success {
				t.Fatalf("response=%#v err=%v", response, err)
			}
		})
	}
	for _, input := range []socialhub.ReactionRequest{
		{ActorID: "/sample-dj/", TargetID: key, Kind: socialhub.ReactionLike},
		{ActorID: "sample-dj", TargetID: key, Kind: socialhub.ReactionRepost},
	} {
		if err := client.React(ctx, input); err != nil {
			t.Fatal(err)
		}
		if err := client.RemoveReaction(ctx, input); err != nil {
			t.Fatal(err)
		}
	}
	want := map[string]int{
		"POST /other-dj/episode/favorite/":       2,
		"DELETE /other-dj/episode/favorite/":     2,
		"POST /other-dj/episode/repost/":         2,
		"DELETE /other-dj/episode/repost/":       2,
		"POST /other-dj/episode/listen-later/":   1,
		"DELETE /other-dj/episode/listen-later/": 1,
		"POST /other-dj/follow/":                 1,
		"DELETE /other-dj/follow/":               1,
	}
	for request, count := range want {
		if seen[request] != count {
			t.Errorf("%s count=%d want=%d", request, seen[request], count)
		}
	}
}

func TestReactionValidationAndFailedResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusOK, `{"result":{"success":false,"message":"denied"}}`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, "")
	ctx := context.Background()
	if err := client.React(ctx, socialhub.ReactionRequest{ActorID: "other", TargetID: "/dj/show/", Kind: socialhub.ReactionLike}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("actor=%v", err)
	}
	if err := client.React(ctx, socialhub.ReactionRequest{TargetID: "/dj/show/", Kind: "clap"}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("kind=%v", err)
	}
	if err := client.RemoveReaction(ctx, socialhub.ReactionRequest{TargetID: "/dj/show/", Kind: "clap"}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("remove kind=%v", err)
	}
	if _, err := client.Favourite(ctx, "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("target=%v", err)
	}
	if _, err := client.Follow(ctx, "bad/name"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("follow=%v", err)
	}
	if _, err := client.Unfollow(ctx, "bad/name"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("unfollow=%v", err)
	}
	if _, err := client.Favourite(ctx, "/dj/show/"); err == nil {
		t.Fatal("failed result succeeded")
	}
	if _, err := client.Comment(ctx, socialhub.CreateCommentRequest{}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("comment=%v", err)
	}
	if err := client.DeleteComment(ctx, "id"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("delete comment=%v", err)
	}
}
