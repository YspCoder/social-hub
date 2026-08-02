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

func TestPublishReplyQuoteStatusAndDeletion(t *testing.T) {
	secret := nostrgo.KeyOne
	root := signedEvent(t, secret, 100, nostrgo.KindTextNote, nil, "root")
	parent := signedEvent(t, secret, 101, nostrgo.KindTextNote, nostrgo.Tags{
		{"e", root.ID.Hex(), "wss://origin.example", "root", root.PubKey.Hex()},
		{"p", root.PubKey.Hex()},
	}, "parent")
	fixture := newRelayFixture(t, root, parent)
	_, client := openTestClient(t, []string{fixture.url}, secret.Public().Hex(), nip19.EncodeNsec(secret), 1)

	text := "reply with quote"
	replyID, quoteID := nip19.EncodeNevent(parent.ID, nil, parent.PubKey), nip19.EncodeNevent(root.ID, nil, root.PubKey)
	post, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, ReplyToID: &replyID, QuotePostID: &quoteID})
	if err != nil {
		t.Fatalf("Publish() error: %v", err)
	}
	if post.AuthorID == nil || *post.AuthorID != secret.Public().Hex() || post.Status != nil {
		t.Fatalf("unexpected published post: %#v", post)
	}
	published := publicationByContent(t, fixture.publications(), nostrgo.KindTextNote, text)
	if !validNIP01Event(published) || published.CreatedAt != 1_800_000_000 {
		t.Fatalf("invalid signed event: %s", published.String())
	}
	assertMarkedTag(t, published.Tags, "root", root.ID.Hex())
	assertMarkedTag(t, published.Tags, "reply", parent.ID.Hex())
	if tag := published.Tags.Find("q"); len(tag) < 4 || tag[1] != root.ID.Hex() || tag[3] != root.PubKey.Hex() {
		t.Fatalf("quote tag = %#v", tag)
	}

	status, err := client.PublishStatus(context.Background(), nip19.EncodeNevent(published.ID, nil, published.PubKey))
	if err != nil || status.State != socialhub.PublishStatePublished || status.ID != published.ID.Hex() {
		t.Fatalf("PublishStatus() = %#v, %v", status, err)
	}
	if err := client.DeletePost(context.Background(), published.ID.Hex()); err != nil {
		t.Fatalf("DeletePost() error: %v", err)
	}
	deletion := lastPublicationOfKind(t, fixture.publications(), nostrgo.KindDeletion)
	if deletion.Tags.FindWithValue("e", published.ID.Hex()) == nil || deletion.Tags.FindWithValue("k", "1") == nil {
		t.Fatalf("deletion tags = %#v", deletion.Tags)
	}
	if !validNIP01Event(deletion) {
		t.Fatal("deletion request signature is invalid")
	}
}

func TestPublishAndDeleteValidation(t *testing.T) {
	secret := nostrgo.KeyOne
	other := nostrgo.Generate()
	otherPost := signedEvent(t, other, 100, nostrgo.KindTextNote, nil, "other")
	fixture := newRelayFixture(t, otherPost)
	_, client := openTestClient(t, []string{fixture.url}, secret.Public().Hex(), secret.Hex(), 1)

	if _, err := client.Publish(context.Background(), socialhub.CreatePostRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty Publish() error = %v", err)
	}
	text := "text"
	if _, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, MediaIDs: []string{"media"}}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("media Publish() error = %v", err)
	}
	private := "private"
	if _, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, Visibility: &private}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("private Publish() error = %v", err)
	}
	if _, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text}, socialhub.WithIdempotencyKey("key")); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("idempotent Publish() error = %v", err)
	}
	if err := client.DeletePost(context.Background(), otherPost.ID.Hex()); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("DeletePost(other) error = %v", err)
	}
	if _, err := client.PublishStatus(context.Background(), nostrgo.ID{}.Hex()); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("PublishStatus(missing) error = %v", err)
	}
}

func TestCommentDirectAndDeepReply(t *testing.T) {
	secret := nostrgo.KeyOne
	root := signedEvent(t, secret, 100, nostrgo.KindTextNote, nil, "root")
	fixture := newRelayFixture(t, root)
	_, client := openTestClient(t, []string{fixture.url}, secret.Public().Hex(), secret.Hex(), 1)

	direct, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: root.ID.Hex(), Text: "direct"})
	if err != nil {
		t.Fatalf("Comment(direct) error: %v", err)
	}
	directEvent := publicationByContent(t, fixture.publications(), nostrgo.KindTextNote, "direct")
	assertMarkedTag(t, directEvent.Tags, "root", root.ID.Hex())
	if tagWithMarker(directEvent.Tags, "reply") != nil || direct.ParentID != nil {
		t.Fatalf("direct reply tags/comment = %#v / %#v", directEvent.Tags, direct)
	}

	parentID := direct.ID
	deep, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: root.ID.Hex(), ParentID: &parentID, Text: "deep"})
	if err != nil {
		t.Fatalf("Comment(deep) error: %v", err)
	}
	deepEvent := publicationByContent(t, fixture.publications(), nostrgo.KindTextNote, "deep")
	assertMarkedTag(t, deepEvent.Tags, "root", root.ID.Hex())
	assertMarkedTag(t, deepEvent.Tags, "reply", direct.ID)
	if deep.ParentID == nil || *deep.ParentID != direct.ID {
		t.Fatalf("deep comment parent = %#v", deep.ParentID)
	}
	if err := client.DeleteComment(context.Background(), deep.ID); err != nil {
		t.Fatalf("DeleteComment() error: %v", err)
	}

	unrelated := signedEvent(t, secret, 99, nostrgo.KindTextNote, nil, "unrelated")
	fixture.mu.Lock()
	fixture.events[unrelated.ID.Hex()] = unrelated
	fixture.mu.Unlock()
	unrelatedID := unrelated.ID.Hex()
	if _, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: root.ID.Hex(), ParentID: &unrelatedID, Text: "bad"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("Comment(unrelated parent) error = %v", err)
	}
	if _, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: root.ID.Hex(), Text: "  "}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("Comment(empty) error = %v", err)
	}
}

func TestReactionsRemovalAndInteractionWorkflow(t *testing.T) {
	secret := nostrgo.KeyOne
	target := signedEvent(t, secret, 100, nostrgo.KindTextNote, nil, "target")
	fixture := newRelayFixture(t, target)
	_, client := openTestClient(t, []string{fixture.url}, secret.Public().Hex(), secret.Hex(), 1)
	request := socialhub.ReactionRequest{ActorID: nip19.EncodeNpub(secret.Public()), TargetID: nip19.EncodeNevent(target.ID, nil, target.PubKey), Kind: socialhub.ReactionLike}

	if err := client.React(context.Background(), request); err != nil {
		t.Fatalf("React() error: %v", err)
	}
	like := publicationByContent(t, fixture.publications(), nostrgo.KindReaction, "+")
	if !validNIP01Event(like) || like.Tags.FindWithValue("e", target.ID.Hex()) == nil || like.Tags.FindWithValue("p", target.PubKey.Hex()) == nil || like.Tags.FindWithValue("k", "1") == nil {
		t.Fatalf("like event = %s", like.String())
	}
	if err := client.RemoveReaction(context.Background(), request); err != nil {
		t.Fatalf("RemoveReaction() error: %v", err)
	}
	removal := lastPublicationOfKind(t, fixture.publications(), nostrgo.KindDeletion)
	if removal.Tags.FindWithValue("e", like.ID.Hex()) == nil || removal.Tags.FindWithValue("k", "7") == nil {
		t.Fatalf("reaction deletion tags = %#v", removal.Tags)
	}

	dislike, err := client.ReactWithContent(context.Background(), request, "-")
	if err != nil || dislike.Content != "-" || dislike.ID == "" {
		t.Fatalf("ReactWithContent() = %#v, %v", dislike, err)
	}
	if _, err := client.ReactWithContent(context.Background(), request, "not emoji"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("ReactWithContent(whitespace) error = %v", err)
	}
	post, err := client.Repost(context.Background(), target.ID.Hex())
	if err != nil {
		t.Fatalf("Repost() error: %v", err)
	}
	if len(post.Relations) != 1 || post.Relations[0].Type != socialhub.RelationRepost || post.Relations[0].PostID != target.ID.Hex() {
		t.Fatalf("repost relations = %#v", post.Relations)
	}
	repost := lastPublicationOfKind(t, fixture.publications(), nostrgo.KindRepost)
	var embedded nostrgo.Event
	if err := json.Unmarshal([]byte(repost.Content), &embedded); err != nil || embedded.ID != target.ID {
		t.Fatalf("repost content = %q, %v", repost.Content, err)
	}

	request.ActorID = nostrgo.Generate().Public().Hex()
	if err := client.React(context.Background(), request); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("React(other actor) error = %v", err)
	}
	request.ActorID, request.Kind = "", socialhub.ReactionRepost
	if err := client.React(context.Background(), request); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("React(repost) error = %v", err)
	}
}

func TestCallTimeout(t *testing.T) {
	secret := nostrgo.KeyOne
	fixture := newRelayFixture(t)
	fixture.closeQuery = "error: unavailable"
	_, client := openTestClient(t, []string{fixture.url}, secret.Public().Hex(), "", 1)
	_, err := client.GetPost(context.Background(), nostrgo.ID{}.Hex(), socialhub.WithCallTimeout(time.Second))
	if err == nil {
		t.Fatal("GetPost() unexpectedly succeeded")
	}
	if _, err := client.GetPost(context.Background(), nostrgo.ID{}.Hex(), socialhub.WithCallTimeout(-time.Second)); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("negative timeout error = %v", err)
	}
}

func publicationByContent(t *testing.T, events []nostrgo.Event, kind nostrgo.Kind, content string) nostrgo.Event {
	t.Helper()
	for index := len(events) - 1; index >= 0; index-- {
		if events[index].Kind == kind && events[index].Content == content {
			return events[index]
		}
	}
	t.Fatalf("publication kind=%d content=%q not found in %#v", kind, content, events)
	return nostrgo.Event{}
}

func lastPublicationOfKind(t *testing.T, events []nostrgo.Event, kind nostrgo.Kind) nostrgo.Event {
	t.Helper()
	for index := len(events) - 1; index >= 0; index-- {
		if events[index].Kind == kind {
			return events[index]
		}
	}
	t.Fatalf("publication kind=%d not found", kind)
	return nostrgo.Event{}
}

func tagWithMarker(tags nostrgo.Tags, marker string) nostrgo.Tag {
	for _, tag := range tags {
		if len(tag) >= 4 && tag[0] == "e" && tag[3] == marker {
			return tag
		}
	}
	return nil
}

func assertMarkedTag(t *testing.T, tags nostrgo.Tags, marker, identifier string) {
	t.Helper()
	tag := tagWithMarker(tags, marker)
	if tag == nil || tag[1] != identifier {
		t.Fatalf("%s tag = %#v, tags=%#v", marker, tag, tags)
	}
}
