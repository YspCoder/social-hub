package nostr

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	nostrgo "fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip19"

	"social-hub/pkg/socialhub"
)

func TestFetchProfilePostsPaginationAndMedia(t *testing.T) {
	secret := nostrgo.KeyOne
	publicKey := secret.Public()
	profile := signedEvent(t, secret, 100, nostrgo.KindProfileMetadata, nil,
		`{"name":"alice","display_name":"Alice","picture":"https://cdn.example/avatar.png","website":"https://alice.example","nip05":"alice@example.com"}`)
	mediaURL := "https://cdn.example/photo.jpg"
	mediaTags := nostrgo.Tags{{
		"imeta", "url " + mediaURL, "m image/jpeg", "dim 1200x800", "size 42", "alt launch photo",
		"x aabbcc", "fallback https://backup.example/one.jpg", "fallback https://backup.example/two.jpg",
	}}
	notes := []nostrgo.Event{
		signedEvent(t, secret, 200, nostrgo.KindTextNote, mediaTags, "photo "+mediaURL),
		signedEvent(t, secret, 200, nostrgo.KindTextNote, nil, "second"),
		signedEvent(t, secret, 200, nostrgo.KindTextNote, nil, "third"),
		signedEvent(t, secret, 200, nostrgo.KindTextNote, nil, "fourth"),
	}
	fixture := newRelayFixture(t, append([]nostrgo.Event{profile}, notes...)...)
	_, client := openTestClient(t, []string{fixture.url}, nip19.EncodeNpub(publicKey), "", 1)

	user, err := client.GetUser(context.Background(), nip19.EncodeNprofile(publicKey, []string{fixture.url}))
	if err != nil {
		t.Fatalf("GetUser() error: %v", err)
	}
	if user.ID != publicKey.Hex() || user.Username == nil || *user.Username != "alice" || user.DisplayName == nil || *user.DisplayName != "Alice" {
		t.Fatalf("unexpected user: %#v", user)
	}
	if user.ProfileURL == nil || *user.ProfileURL != "nostr:"+nip19.EncodeNpub(publicKey) {
		t.Fatalf("unexpected profile URL: %#v", user.ProfileURL)
	}

	post, err := client.GetPost(context.Background(), nip19.EncodeNevent(notes[0].ID, []string{fixture.url}, publicKey))
	if err != nil {
		t.Fatalf("GetPost() error: %v", err)
	}
	if post.ID != notes[0].ID.Hex() || len(post.Media) != 1 {
		t.Fatalf("unexpected post: %#v", post)
	}
	media := post.Media[0]
	if media.URL != mediaURL || media.Type != socialhub.MediaTypeImage || media.Width == nil || *media.Width != 1200 || media.Size == nil || *media.Size != 42 {
		t.Fatalf("unexpected media: %#v", media)
	}
	var fallbacks []string
	if err := json.Unmarshal(media.Extensions["nostr.fallback"], &fallbacks); err != nil || len(fallbacks) != 2 {
		t.Fatalf("fallback extension = %q, %v", media.Extensions["nostr.fallback"], err)
	}

	page1, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: publicKey.Hex(), MaxResults: 2})
	if err != nil {
		t.Fatalf("ListPosts(page 1) error: %v", err)
	}
	if len(page1.Items) != 2 || !page1.HasMore || page1.NextCursor == nil {
		t.Fatalf("unexpected page 1: %#v", page1)
	}
	page2, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: nip19.EncodeNpub(publicKey), MaxResults: 2, Cursor: *page1.NextCursor})
	if err != nil {
		t.Fatalf("ListPosts(page 2) error: %v", err)
	}
	if len(page2.Items) != 2 || page2.HasMore {
		t.Fatalf("unexpected page 2: %#v", page2)
	}
	seen := map[string]bool{}
	for _, post := range append(page1.Items, page2.Items...) {
		if seen[post.ID] {
			t.Fatalf("duplicate post across pages: %s", post.ID)
		}
		seen[post.ID] = true
	}
	if len(seen) != len(notes) {
		t.Fatalf("paginated IDs = %v", seen)
	}
	if !fixture.sawCommand("REQ") {
		t.Fatal("relay did not receive NIP-01 REQ")
	}
}

func TestFetchCommentsPreservesNIP10ThreadShape(t *testing.T) {
	rootSecret := nostrgo.KeyOne
	commenter := nostrgo.Generate()
	root := signedEvent(t, rootSecret, 100, nostrgo.KindTextNote, nil, "root")
	direct := signedEvent(t, commenter, 101, nostrgo.KindTextNote, nostrgo.Tags{
		{"e", root.ID.Hex(), "wss://relay.example", "root", root.PubKey.Hex()},
		{"p", root.PubKey.Hex()},
	}, "direct")
	deep := signedEvent(t, rootSecret, 102, nostrgo.KindTextNote, nostrgo.Tags{
		{"e", root.ID.Hex(), "wss://relay.example", "root", root.PubKey.Hex()},
		{"e", direct.ID.Hex(), "wss://relay.example", "reply", direct.PubKey.Hex()},
		{"p", root.PubKey.Hex()}, {"p", direct.PubKey.Hex()},
	}, "deep")
	quoteOnly := signedEvent(t, commenter, 103, nostrgo.KindTextNote, nostrgo.Tags{{"q", root.ID.Hex()}}, "quote")
	fixture := newRelayFixture(t, root, direct, deep, quoteOnly)
	_, client := openTestClient(t, []string{fixture.url}, root.PubKey.Hex(), "", 1)

	page, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: nip19.EncodeNevent(root.ID, nil, root.PubKey), MaxResults: 10})
	if err != nil {
		t.Fatalf("ListComments() error: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("comments = %#v", page.Items)
	}
	byID := map[string]socialhub.Comment{}
	for _, comment := range page.Items {
		byID[comment.ID] = comment
	}
	if byID[direct.ID.Hex()].ParentID != nil {
		t.Fatalf("direct reply parent = %#v", byID[direct.ID.Hex()].ParentID)
	}
	if byID[deep.ID.Hex()].ParentID == nil || *byID[deep.ID.Hex()].ParentID != direct.ID.Hex() {
		t.Fatalf("deep reply parent = %#v", byID[deep.ID.Hex()].ParentID)
	}
	post, err := client.GetPost(context.Background(), deep.ID.Hex())
	if err != nil {
		t.Fatalf("GetPost(deep) error: %v", err)
	}
	if len(post.Relations) != 1 || post.Relations[0].Type != socialhub.RelationReply || post.Relations[0].PostID != direct.ID.Hex() {
		t.Fatalf("relations = %#v", post.Relations)
	}
}

func TestFetchValidationNotFoundAndMalformedProfile(t *testing.T) {
	secret := nostrgo.KeyOne
	malformed := signedEvent(t, secret, 100, nostrgo.KindProfileMetadata, nil, "not-json")
	fixture := newRelayFixture(t, malformed)
	_, client := openTestClient(t, []string{fixture.url}, secret.Public().Hex(), "", 1)

	if _, err := client.GetUser(context.Background(), ""); err == nil {
		t.Fatal("GetUser() accepted malformed profile metadata")
	}
	if _, err := client.GetPost(context.Background(), nostrgo.Generate().Public().Hex()); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("GetPost(not found) error = %v", err)
	}
	if _, err := client.GetPost(context.Background(), "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("GetPost(bad ID) error = %v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{MaxResults: 101}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("ListPosts(max) error = %v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "bad"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("ListPosts(cursor) error = %v", err)
	}
	start, end := time.Unix(20, 0), time.Unix(10, 0)
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{StartTime: &start, EndTime: &end}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("ListPosts(time range) error = %v", err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "bad"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("ListComments(bad ID) error = %v", err)
	}
}

func TestMediaIgnoresUnreferencedOrInvalidIMeta(t *testing.T) {
	secret := nostrgo.KeyOne
	event := signedEvent(t, secret, 100, nostrgo.KindTextNote, nostrgo.Tags{
		{"imeta", "url https://cdn.example/unreferenced.jpg", "m image/jpeg"},
		{"imeta", "url javascript:alert(1)", "m image/jpeg"},
	}, "no media URL here")
	fixture := newRelayFixture(t, event)
	_, client := openTestClient(t, []string{fixture.url}, secret.Public().Hex(), "", 1)
	post, err := client.GetPost(context.Background(), nip19.EncodeNevent(event.ID, nil, event.PubKey))
	if err != nil {
		t.Fatalf("GetPost() error: %v", err)
	}
	if len(post.Media) != 0 {
		t.Fatalf("unexpected media: %#v", post.Media)
	}
}
