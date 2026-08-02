package nostr

import (
	"encoding/json"
	"testing"

	nostrgo "fiatjaf.com/nostr"
)

func TestNIP01KnownSignatureVector(t *testing.T) {
	const raw = `{"kind":1,"id":"dc90c95f09947507c1044e8f48bcf6350aa6bff1507dd4acfc755b9239b5c962","pubkey":"3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d","created_at":1644271588,"tags":[],"content":"now that https://blueskyweb.org/blog/2-7-2022-overview was announced we can stop working on nostr?","sig":"230e9d8f0ddaf7eb70b5f7741ccfa37e87a455c9a469282e3464e2052d3192cd63a167e196e381ef9d7e69e9ea43af2443b839974dc85d8aaab9efe1d9296524"}`
	var event nostrgo.Event
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatalf("decode known event: %v", err)
	}
	if !validNIP01Event(event) {
		t.Fatal("known NIP-01 event did not verify")
	}
	event.Content += "!"
	if validNIP01Event(event) {
		t.Fatal("modified NIP-01 event unexpectedly verified")
	}
}

func TestCanonicalEventIDDoesNotHTMLEscape(t *testing.T) {
	event := nostrgo.Event{
		PubKey: nostrgo.KeyOne.Public(), CreatedAt: 1, Kind: nostrgo.KindTextNote,
		Tags: nostrgo.Tags{}, Content: "<>&\u2028",
	}
	first := canonicalEventID(event)
	event.Content = "\\u003c\\u003e\\u0026\u2028"
	second := canonicalEventID(event)
	if first == second {
		t.Fatal("canonical encoder HTML-escaped event content")
	}
}
