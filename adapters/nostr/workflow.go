package nostr

import (
	"context"
	"encoding/json"
	"time"

	nostrgo "fiatjaf.com/nostr"

	"social-hub/pkg/socialhub"
)

const (
	CapabilityNIP25   socialhub.Capability = "nostr_nip25_reactions"
	CapabilityReposts socialhub.Capability = "nostr_nip18_reposts"
)

// Reaction is the typed result of publishing a NIP-25 reaction event.
type Reaction struct {
	ID         string                     `json:"id"`
	TargetID   string                     `json:"target_id"`
	Content    string                     `json:"content"`
	CreatedAt  *time.Time                 `json:"created_at,omitempty"`
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`
}

// InteractionWorkflow exposes Nostr semantics that do not fit the common
// like-only Reactor contract.
type InteractionWorkflow interface {
	ReactWithContent(context.Context, socialhub.ReactionRequest, string, ...socialhub.CallOption) (*Reaction, error)
	Repost(context.Context, string, ...socialhub.CallOption) (*socialhub.Post, error)
}

func (client *Client) InteractionWorkflow() (InteractionWorkflow, bool) {
	if client.secretKey == nil {
		return nil, false
	}
	return client, true
}

func (client *Client) ReactWithContent(ctx context.Context, input socialhub.ReactionRequest, content string, options ...socialhub.CallOption) (*Reaction, error) {
	event, publish, err := client.reactWithContent(ctx, input, content, "react_with_content", options...)
	if err != nil {
		return nil, err
	}
	targetID := input.TargetID
	if tag := event.Tags.Find("e"); len(tag) >= 2 {
		targetID = tag[1]
	}
	result := &Reaction{
		ID: event.ID.Hex(), TargetID: targetID, Content: event.Content, CreatedAt: eventTime(event.CreatedAt),
		Extensions: eventExtensions(event, reportForPublished(event, publish)),
	}
	putExtension(result.Extensions, "nostr.publish", publish)
	return result, nil
}

func (client *Client) Repost(ctx context.Context, identifier string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	callCtx, cancel, err := client.callContext(ctx, "repost", options...)
	if err != nil {
		return nil, err
	}
	defer cancel()
	target, query, err := client.getTextNote(callCtx, "repost", identifier)
	if err != nil {
		return nil, err
	}
	hint := relayHint(query.Sources[target.ID.Hex()])
	event := nostrgo.Event{
		Kind:    nostrgo.KindRepost,
		Tags:    nostrgo.Tags{{"e", target.ID.Hex(), hint}, {"p", target.PubKey.Hex()}},
		Content: rawEventContent(target),
	}
	publish, err := client.publishEvent(callCtx, "repost", &event)
	if err != nil {
		return nil, err
	}
	post := client.mapPost(event, reportForPublished(event, publish))
	putExtension(post.Extensions, "nostr.publish", publish)
	return &post, nil
}

var _ InteractionWorkflow = (*Client)(nil)
