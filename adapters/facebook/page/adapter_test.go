package page

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"social-hub/pkg/socialhub"
)

type mapResolver map[string]string

func (r mapResolver) Resolve(_ context.Context, reference string) (string, error) {
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
		Product: "page",
		Settings: map[string]any{
			"base_url":  server.URL + "/v26.0",
			"auth_url":  server.URL + "/v26.0/dialog/oauth",
			"token_url": server.URL + "/v26.0/oauth/access_token",
		},
		Accounts: []socialhub.AccountConfig{{
			ID:             "primary",
			ClientID:       "app-id",
			SecretRef:      "test://app-secret",
			AccessTokenRef: "test://page-token",
			Webhook: socialhub.WebhookConfig{
				SecretRef: "test://webhook-secret",
				TokenRef:  "test://verify-token",
			},
			Settings: map[string]any{"page_id": "123"},
		}},
	}
	adapter := &Adapter{}
	err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{
			"test://app-secret":     "app-secret",
			"test://page-token":     "page-token",
			"test://webhook-secret": "webhook-secret",
			"test://verify-token":   "verify-token",
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

func TestAdapterRegistrationAndSurface(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters = %v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestClient(t, server)
	if adapter.Name() != adapterName || adapter.Metadata().APIVersion != graphVersion {
		t.Fatalf("metadata = %#v", adapter.Metadata())
	}
	if client.Platform() != "facebook" || client.Account() != "primary" {
		t.Fatalf("client identity = %s/%s", client.Platform(), client.Account())
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities.Has(socialhub.CapPublish) || !capabilities.Has(socialhub.CapWebhook) || capabilities.Has(socialhub.CapMessage) {
		t.Fatalf("capabilities = %#v, err = %v", capabilities, err)
	}
	if _, ok := client.Publisher(); !ok {
		t.Fatal("publisher unavailable")
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("fetcher unavailable")
	}
	if _, ok := client.MediaUploader(); !ok {
		t.Fatal("uploader unavailable")
	}
	if _, ok := client.Reactor(); !ok {
		t.Fatal("reactor unavailable")
	}
	if _, ok := client.WebhookHandler(); !ok {
		t.Fatal("webhook unavailable")
	}
	if value, ok := client.Messenger(); ok || value != nil {
		t.Fatal("messenger should be unavailable")
	}
	oauth, err := adapter.OAuth(context.Background(), "primary")
	if err != nil || oauth.ClientSecret != "app-secret" {
		t.Fatalf("oauth = %#v, err = %v", oauth, err)
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

func TestPageContentAndEngagement(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var publishedMessage string
	var attachedMedia string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer page-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/v26.0/123/feed":
			_ = request.ParseForm()
			mu.Lock()
			publishedMessage = request.Form.Get("message")
			attachedMedia = request.Form.Get("attached_media[0]")
			mu.Unlock()
			_, _ = writer.Write([]byte(`{"id":"123_456"}`))
		case request.Method == http.MethodGet && request.URL.Path == "/v26.0/123":
			_, _ = writer.Write([]byte(`{"id":"123","name":"Example Page","link":"https://facebook.com/example","picture":{"data":{"url":"https://cdn.example/avatar.jpg"}}}`))
		case request.Method == http.MethodGet && request.URL.Path == "/v26.0/123_456":
			_, _ = writer.Write([]byte(`{"id":"123_456","message":"hello","created_time":"2026-08-01T00:00:00Z","permalink_url":"https://facebook.com/123_456","from":{"id":"123","name":"Example Page"},"attachments":{"data":[{"type":"photo","target":{"id":"photo-1"},"media":{"image":{"src":"https://cdn.example/photo.jpg","width":100,"height":80}}}]}}`))
		case request.Method == http.MethodGet && request.URL.Path == "/v26.0/123/feed":
			_, _ = writer.Write([]byte(`{"data":[{"id":"123_456","message":"hello"}],"paging":{"cursors":{"after":"next"},"next":"https://next.example"}}`))
		case request.Method == http.MethodGet && request.URL.Path == "/v26.0/123_456/comments":
			_, _ = writer.Write([]byte(`{"data":[{"id":"comment-1","message":"reply","from":{"id":"9","name":"Reader"},"parent":{"id":"123_456"}}]}`))
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/comments"):
			_, _ = writer.Write([]byte(`{"id":"comment-2"}`))
		case strings.HasSuffix(request.URL.Path, "/likes"):
			_, _ = writer.Write([]byte(`{"success":true}`))
		case request.Method == http.MethodDelete:
			_, _ = writer.Write([]byte(`{"success":true}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)

	text := "hello"
	post, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, MediaIDs: []string{"photo-1"}})
	if err != nil || post.ID != "123_456" {
		t.Fatalf("post = %#v, err = %v", post, err)
	}
	mu.Lock()
	message, media := publishedMessage, attachedMedia
	mu.Unlock()
	if message != text || !strings.Contains(media, "photo-1") {
		t.Fatalf("publish form message=%q media=%q", message, media)
	}
	user, err := client.GetUser(context.Background(), "")
	if err != nil || user.DisplayName == nil || *user.DisplayName != "Example Page" {
		t.Fatalf("user = %#v, err = %v", user, err)
	}
	post, err = client.GetPost(context.Background(), post.ID)
	if err != nil || len(post.Media) != 1 || post.Media[0].Type != socialhub.MediaTypeImage {
		t.Fatalf("mapped post = %#v, err = %v", post, err)
	}
	status, err := client.PublishStatus(context.Background(), post.ID)
	if err != nil || status.State != socialhub.PublishStatePublished {
		t.Fatalf("status = %#v, err = %v", status, err)
	}
	posts, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "cursor", MaxResults: 10})
	if err != nil || len(posts.Items) != 1 || !posts.HasMore {
		t.Fatalf("posts = %#v, err = %v", posts, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: post.ID})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].ParentID == nil {
		t.Fatalf("comments = %#v, err = %v", comments, err)
	}
	parentID := comments.Items[0].ID
	created, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: post.ID, ParentID: &parentID, Text: "nested"})
	if err != nil || created.ID != "comment-2" {
		t.Fatalf("created comment = %#v, err = %v", created, err)
	}
	reaction := socialhub.ReactionRequest{TargetID: post.ID, Kind: socialhub.ReactionLike}
	if err := client.React(context.Background(), reaction); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveReaction(context.Background(), reaction); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteComment(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	if err := client.DeletePost(context.Background(), post.ID); err != nil {
		t.Fatal(err)
	}
}

func TestRepostIsExplicitlyUnsupported(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server)
	err := client.React(context.Background(), socialhub.ReactionRequest{TargetID: "post", Kind: socialhub.ReactionRepost})
	if !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("error = %v", err)
	}
}
