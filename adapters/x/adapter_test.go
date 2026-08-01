package x

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"

	"social-hub/pkg/socialhub"
)

type mapSecretResolver map[string]string

func (r mapSecretResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, ok := r[reference]
	if !ok {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

func newTestClient(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	config := socialhub.AdapterConfig{
		Adapter: adapterName,
		Product: "api",
		Settings: map[string]any{
			"base_url":  server.URL,
			"token_url": server.URL + "/2/oauth2/token",
			"auth_url":  server.URL + "/i/oauth2/authorize",
		},
		Accounts: []socialhub.AccountConfig{{
			ID:             "primary",
			ClientID:       "client-id",
			SecretRef:      "test://client-secret",
			AccessTokenRef: "test://access-token",
		}},
	}
	adapter := &Adapter{}
	err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapSecretResolver{
			"test://client-secret": "client-secret",
			"test://access-token":  "access-token",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	client, err := adapter.Client(context.Background(), "primary")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, client.(*Client)
}

func TestAdapterRegistrationAndCapabilities(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters = %v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server)
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia, socialhub.CapReact} {
		if !capabilities.Has(capability) {
			t.Fatalf("capability %q is not available", capability)
		}
	}
	if capabilities.Has(socialhub.CapMessage) {
		t.Fatal("message capability should not be advertised")
	}
}

func TestAdapterAndClientSurface(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/2/users/7":
			_, _ = writer.Write([]byte(`{"data":{"id":"7","name":"Ada","username":"ada","profile_image_url":"https://cdn.example/avatar.jpg","url":"https://x.com/ada"}}`))
		case request.Method == http.MethodGet && request.URL.Path == "/2/tweets/42":
			_, _ = writer.Write([]byte(`{"data":{"id":"42","text":"hello","created_at":"2026-08-01T00:00:00Z"}}`))
		case request.Method == http.MethodPost && request.URL.Path == "/2/tweets":
			_, _ = writer.Write([]byte(`{"data":{"id":"43","text":"reply"}}`))
		case request.Method == http.MethodDelete && request.URL.Path == "/2/tweets/43":
			_, _ = writer.Write([]byte(`{"data":{"deleted":true}}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	adapter, client := newTestClient(t, server)

	if adapter.Name() != adapterName || adapter.Metadata().APIVersion != "2" {
		t.Fatalf("adapter metadata = %#v", adapter.Metadata())
	}
	if client.Platform() != "x" || client.Account() != "primary" {
		t.Fatalf("client identity = %q/%q", client.Platform(), client.Account())
	}
	if publisher, ok := client.Publisher(); !ok || publisher == nil {
		t.Fatal("publisher is unavailable")
	}
	if fetcher, ok := client.Fetcher(); !ok || fetcher == nil {
		t.Fatal("fetcher is unavailable")
	}
	if uploader, ok := client.MediaUploader(); !ok || uploader == nil {
		t.Fatal("media uploader is unavailable")
	}
	if reactor, ok := client.Reactor(); !ok || reactor == nil {
		t.Fatal("reactor is unavailable")
	}
	if messenger, ok := client.Messenger(); ok || messenger != nil {
		t.Fatal("messenger should be unavailable")
	}
	if webhook, ok := client.WebhookHandler(); ok || webhook != nil {
		t.Fatal("webhook should be unavailable")
	}

	user, err := client.GetUser(context.Background(), "7")
	if err != nil || user.Username == nil || *user.Username != "ada" {
		t.Fatalf("user = %#v, err = %v", user, err)
	}
	status, err := client.PublishStatus(context.Background(), "42")
	if err != nil || status.State != socialhub.PublishStatePublished {
		t.Fatalf("publish status = %#v, err = %v", status, err)
	}
	comment, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "42", Text: "reply"})
	if err != nil || comment.ID != "43" {
		t.Fatalf("comment = %#v, err = %v", comment, err)
	}
	if err := client.DeleteComment(context.Background(), comment.ID); err != nil {
		t.Fatal(err)
	}
	oauth, err := adapter.OAuth(context.Background(), "primary")
	if err != nil || oauth.ClientSecret != "client-secret" {
		t.Fatalf("OAuth client = %#v, err = %v", oauth, err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "primary"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error = %v", err)
	}
}

func TestPublishGetAndDeletePost(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var publishBody createPostRequest
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/2/tweets":
			mu.Lock()
			_ = json.NewDecoder(request.Body).Decode(&publishBody)
			mu.Unlock()
			_, _ = writer.Write([]byte(`{"data":{"id":"42","text":"hello"}}`))
		case request.Method == http.MethodGet && request.URL.Path == "/2/tweets/42":
			if request.URL.Query().Get("tweet.fields") == "" || request.URL.Query().Get("expansions") == "" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"data":{"id":"42","text":"hello","author_id":"7","created_at":"2026-08-01T00:00:00Z","attachments":{"media_keys":["3_9"]}},"includes":{"media":[{"media_key":"3_9","type":"photo","url":"https://cdn.example/image.jpg","width":100,"height":80}]}}`))
		case request.Method == http.MethodDelete && request.URL.Path == "/2/tweets/42":
			_, _ = writer.Write([]byte(`{"data":{"deleted":true}}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)

	text, replyID := "hello", "11"
	post, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, MediaIDs: []string{"9"}, ReplyToID: &replyID})
	if err != nil {
		t.Fatal(err)
	}
	if post.ID != "42" || post.Text == nil || *post.Text != text {
		t.Fatalf("published post = %#v", post)
	}
	mu.Lock()
	gotBody := publishBody
	mu.Unlock()
	if gotBody.Media == nil || !slices.Equal(gotBody.Media.MediaIDs, []string{"9"}) || gotBody.Reply == nil || gotBody.Reply.InReplyToTweetID != replyID {
		t.Fatalf("publish request = %#v", gotBody)
	}

	post, err = client.GetPost(context.Background(), "42")
	if err != nil {
		t.Fatal(err)
	}
	if post.AuthorID == nil || *post.AuthorID != "7" || len(post.Media) != 1 || post.Media[0].Type != socialhub.MediaTypeImage {
		t.Fatalf("mapped post = %#v", post)
	}
	if err := client.DeletePost(context.Background(), "42"); err != nil {
		t.Fatal(err)
	}
}

func TestListPostsCommentsAndReactions(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		requests = append(requests, request.Method+" "+request.URL.RequestURI())
		mu.Unlock()
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/2/users/7/tweets":
			_, _ = writer.Write([]byte(`{"data":[{"id":"1","text":"one"}],"meta":{"next_token":"next"}}`))
		case request.Method == http.MethodGet && request.URL.Path == "/2/tweets/search/recent":
			_, _ = writer.Write([]byte(`{"data":[{"id":"10","text":"root"},{"id":"12","text":"reply","author_id":"8","referenced_tweets":[{"type":"replied_to","id":"10"}]}]}`))
		case request.Method == http.MethodPost || request.Method == http.MethodDelete:
			_, _ = writer.Write([]byte(`{"data":{"liked":true}}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)

	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "7", Cursor: "cursor", MaxResults: 25})
	if err != nil || len(page.Items) != 1 || !page.HasMore {
		t.Fatalf("post page = %#v, err = %v", page, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "10"})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].ParentID == nil || *comments.Items[0].ParentID != "10" {
		t.Fatalf("comment page = %#v, err = %v", comments, err)
	}
	for _, reaction := range []socialhub.ReactionKind{socialhub.ReactionLike, socialhub.ReactionRepost} {
		input := socialhub.ReactionRequest{ActorID: "7", TargetID: "10", Kind: reaction}
		if err := client.React(context.Background(), input); err != nil {
			t.Fatal(err)
		}
		if err := client.RemoveReaction(context.Background(), input); err != nil {
			t.Fatal(err)
		}
	}

	mu.Lock()
	gotRequests := append([]string(nil), requests...)
	mu.Unlock()
	assertRequestQuery(t, gotRequests, "/2/users/7/tweets", "pagination_token", "cursor")
	assertRequestQuery(t, gotRequests, "/2/tweets/search/recent", "query", "conversation_id:10")
}

func assertRequestQuery(t *testing.T, requests []string, path, key, want string) {
	t.Helper()
	for _, raw := range requests {
		parts := strings.SplitN(raw, " ", 2)
		parsed, err := url.Parse(parts[1])
		if err == nil && parsed.Path == path {
			if got := parsed.Query().Get(key); got != want {
				t.Fatalf("%s query %s = %q, want %q", path, key, got, want)
			}
			return
		}
	}
	t.Fatalf("request for %s not found in %v", path, requests)
}
