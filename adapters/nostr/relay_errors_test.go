package nostr

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	nostrgo "fiatjaf.com/nostr"

	"social-hub/pkg/socialhub"
)

func TestMultiRelayDedupPartialFailureAndWriteQuorum(t *testing.T) {
	secret := nostrgo.KeyOne
	target := signedEvent(t, secret, 100, nostrgo.KindTextNote, nil, "target")
	first := newRelayFixture(t, target)
	second := newRelayFixture(t, target)
	_, reader := openTestClient(t, []string{first.url, second.url}, secret.Public().Hex(), "", 1)

	post, err := reader.GetPost(context.Background(), target.ID.Hex())
	if err != nil {
		t.Fatalf("GetPost() error: %v", err)
	}
	var sources []string
	if err := json.Unmarshal(post.Extensions["nostr.relays"], &sources); err != nil || len(sources) != 2 {
		t.Fatalf("sources = %q, %v", post.Extensions["nostr.relays"], err)
	}

	second.closeQuery = "error: relay maintenance"
	partial, err := reader.GetPost(context.Background(), target.ID.Hex())
	if err != nil {
		t.Fatalf("partial GetPost() error: %v", err)
	}
	if len(partial.Extensions["nostr.partial_failures"]) == 0 {
		t.Fatalf("partial failures missing: %#v", partial.Extensions)
	}

	second.closeQuery = ""
	second.rejectPublication = "rate-limited: slow down"
	_, writer := openTestClient(t, []string{first.url, second.url}, secret.Public().Hex(), secret.Hex(), 1)
	text := "quorum one"
	published, err := writer.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text})
	if err != nil {
		t.Fatalf("Publish(quorum 1) error: %v", err)
	}
	var report publishReport
	if err := json.Unmarshal(published.Extensions["nostr.publish"], &report); err != nil || len(report.Succeeded) != 1 || len(report.Failed) != 1 {
		t.Fatalf("publish report = %q, %v", published.Extensions["nostr.publish"], err)
	}

	_, strictWriter := openTestClient(t, []string{first.url, second.url}, secret.Public().Hex(), secret.Hex(), 2)
	text = "quorum two"
	_, err = strictWriter.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text})
	if !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("Publish(quorum 2) error = %v", err)
	}
}

func TestRelayErrorPrefixMapping(t *testing.T) {
	for _, test := range []struct {
		prefix string
		want   error
	}{
		{prefix: "rate-limited: later", want: socialhub.ErrRateLimited},
		{prefix: "blocked: policy", want: socialhub.ErrPermissionDenied},
		{prefix: "restricted: auth", want: socialhub.ErrPermissionDenied},
		{prefix: "invalid: bad signature", want: socialhub.ErrInvalidArgument},
		{prefix: "pow: 20 bits required", want: socialhub.ErrInvalidArgument},
		{prefix: "duplicate: already stored", want: socialhub.ErrConflict},
	} {
		t.Run(test.prefix, func(t *testing.T) {
			err := relayError("test", test.prefix, nil)
			if !errors.Is(err, test.want) {
				t.Fatalf("relayError(%q) = %v, want %v", test.prefix, err, test.want)
			}
		})
	}
}

func TestAllRelayQueryFailuresPreserveRateLimit(t *testing.T) {
	secret := nostrgo.KeyOne
	first := newRelayFixture(t)
	second := newRelayFixture(t)
	first.closeQuery = "error: unavailable"
	second.closeQuery = "rate-limited: later"
	_, client := openTestClient(t, []string{first.url, second.url}, secret.Public().Hex(), "", 1)
	_, err := client.GetPost(context.Background(), nostrgo.ID{}.Hex())
	if !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("GetPost(all failed) error = %v", err)
	}
}

func TestClientCloseIsIdempotent(t *testing.T) {
	secret := nostrgo.KeyOne
	event := signedEvent(t, secret, 100, nostrgo.KindTextNote, nil, "event")
	fixture := newRelayFixture(t, event)
	_, client := openTestClient(t, []string{fixture.url}, secret.Public().Hex(), "", 1)
	if _, err := client.GetPost(context.Background(), event.ID.Hex()); err != nil {
		t.Fatalf("GetPost() error: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("first Close() error: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("second Close() error: %v", err)
	}
}
