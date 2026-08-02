package nostr

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	nostrgo "fiatjaf.com/nostr"

	"social-hub/pkg/socialhub"
)

func (client *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := input.Validate(); err != nil {
		return nil, platformError("publish", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if len(input.MediaIDs) != 0 {
		return nil, unsupported("publish", "Nostr core has no uniform media upload identifier contract; put remote URLs in text with NIP-92 imeta tags via a platform extension")
	}
	if input.Text == nil || strings.TrimSpace(*input.Text) == "" {
		return nil, invalidArgument("publish", "a non-empty text note is required")
	}
	if input.Visibility != nil && strings.TrimSpace(*input.Visibility) != "" && !strings.EqualFold(strings.TrimSpace(*input.Visibility), "public") {
		return nil, unsupported("publish", "NIP-01 kind 1 notes are public")
	}
	callCtx, cancel, err := client.callContext(ctx, "publish", options...)
	if err != nil {
		return nil, err
	}
	defer cancel()

	tags := make(nostrgo.Tags, 0, 6)
	if input.ReplyToID != nil {
		parent, report, getErr := client.getTextNote(callCtx, "publish", *input.ReplyToID)
		if getErr != nil {
			return nil, getErr
		}
		tags = append(tags, client.replyTags(parent, report.Sources[parent.ID.Hex()])...)
	}
	if input.QuotePostID != nil {
		quoted, report, getErr := client.getTextNote(callCtx, "publish", *input.QuotePostID)
		if getErr != nil {
			return nil, getErr
		}
		hint := relayHint(report.Sources[quoted.ID.Hex()])
		tags = append(tags, quoteTag(quoted.ID, hint, quoted.PubKey))
		tags = appendPTag(tags, quoted.PubKey, hint)
	}
	event := nostrgo.Event{Kind: nostrgo.KindTextNote, Tags: tags, Content: *input.Text}
	report, err := client.publishEvent(callCtx, "publish", &event)
	if err != nil {
		return nil, err
	}
	post := client.mapPost(event, reportForPublished(event, report))
	putExtension(post.Extensions, "nostr.publish", report)
	return &post, nil
}

func (client *Client) PublishStatus(ctx context.Context, identifier string, options ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	event, _, err := client.getTextNote(ctx, "publish_status", identifier, options...)
	if err != nil {
		return nil, err
	}
	updatedAt := eventTime(event.CreatedAt)
	return &socialhub.PublishStatus{ID: event.ID.Hex(), State: socialhub.PublishStatePublished, UpdatedAt: updatedAt}, nil
}

func (client *Client) DeletePost(ctx context.Context, identifier string, options ...socialhub.CallOption) error {
	return client.deleteTextNote(ctx, "delete_post", identifier, options...)
}

func (client *Client) deleteTextNote(ctx context.Context, operation, identifier string, options ...socialhub.CallOption) error {
	callCtx, cancel, err := client.callContext(ctx, operation, options...)
	if err != nil {
		return err
	}
	defer cancel()
	target, report, err := client.getTextNote(callCtx, operation, identifier)
	if err != nil {
		return err
	}
	if target.PubKey != client.publicKey {
		return &socialhub.Error{
			Code: socialhub.CodePermissionDenied, Class: socialhub.ClassUserAction,
			Op: operation, Platform: "nostr", Product: productName,
			PlatformMessage: "NIP-09 permits an author to request deletion only for their own event",
		}
	}
	hint := relayHint(report.Sources[target.ID.Hex()])
	deletion := nostrgo.Event{
		Kind: nostrgo.KindDeletion,
		Tags: nostrgo.Tags{
			relayTag("e", target.ID.Hex(), hint, "", ""),
			{"k", strconv.Itoa(int(target.Kind))},
		},
		Content: "",
	}
	_, err = client.publishEvent(callCtx, operation, &deletion)
	return err
}

func (client *Client) publishEvent(ctx context.Context, operation string, event *nostrgo.Event) (publishReport, error) {
	if err := client.signEvent(event); err != nil {
		return publishReport{}, err
	}
	report, err := client.network.Publish(ctx, *event, client.writeQuorum)
	if err != nil {
		var platform *socialhub.Error
		if errors.As(err, &platform) {
			platform.Op = operation
		}
		return report, err
	}
	return report, nil
}

func (client *Client) replyTags(parent nostrgo.Event, sourceRelays []string) nostrgo.Tags {
	hint := relayHint(sourceRelays)
	parentReference := eventReference{ID: parent.ID, Relay: hint, Author: parent.PubKey}
	root, hasRoot := threadRootReference(parent.Tags)
	if !hasRoot || root.ID == parent.ID {
		return appendPTag(nostrgo.Tags{referenceTag("e", parentReference, "root")}, parent.PubKey, hint)
	}
	if root.Relay == "" {
		root.Relay = hint
	}
	tags := nostrgo.Tags{referenceTag("e", root, "root"), referenceTag("e", parentReference, "reply")}
	tags = appendPTag(tags, root.Author, root.Relay)
	return appendPTag(tags, parent.PubKey, hint)
}

func quoteTag(identifier nostrgo.ID, hint string, author nostrgo.PubKey) nostrgo.Tag {
	tag := nostrgo.Tag{"q", identifier.Hex()}
	if hint != "" || author != nostrgo.ZeroPK {
		tag = append(tag, hint)
	}
	if author != nostrgo.ZeroPK {
		tag = append(tag, author.Hex())
	}
	return tag
}

func appendPTag(tags nostrgo.Tags, publicKey nostrgo.PubKey, hint string) nostrgo.Tags {
	if publicKey == nostrgo.ZeroPK {
		return tags
	}
	for _, tag := range tags {
		if len(tag) >= 2 && tag[0] == "p" && tag[1] == publicKey.Hex() {
			return tags
		}
	}
	tag := nostrgo.Tag{"p", publicKey.Hex()}
	if hint != "" {
		tag = append(tag, hint)
	}
	return append(tags, tag)
}

func reportForPublished(event nostrgo.Event, report publishReport) queryReport {
	return queryReport{
		Succeeded: append([]string(nil), report.Succeeded...), Failed: report.Failed,
		Sources: map[string][]string{event.ID.Hex(): append([]string(nil), report.Succeeded...)},
	}
}

func rawEventContent(event nostrgo.Event) string {
	encoded, _ := json.Marshal(event)
	return string(encoded)
}
