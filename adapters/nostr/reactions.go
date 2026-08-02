package nostr

import (
	"context"
	"strings"

	nostrgo "fiatjaf.com/nostr"

	"social-hub/pkg/socialhub"
)

func (client *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if input.Kind != socialhub.ReactionLike {
		return unsupported("react", "the common Nostr reactor supports NIP-25 likes; use InteractionWorkflow for dislikes, emoji, and reposts")
	}
	_, _, err := client.reactWithContent(ctx, input, "+", "react", options...)
	return err
}

func (client *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if input.Kind != socialhub.ReactionLike {
		return unsupported("remove_reaction", "the common Nostr reactor supports NIP-25 like removal only")
	}
	if err := client.validateActor(input.ActorID, "remove_reaction"); err != nil {
		return err
	}
	targetID, err := parseEventID(input.TargetID)
	if err != nil {
		return invalidArgument("remove_reaction", "target ID must be hex, note, or nevent")
	}
	callCtx, cancel, err := client.callContext(ctx, "remove_reaction", options...)
	if err != nil {
		return err
	}
	defer cancel()
	filter := nostrgo.Filter{
		Kinds: []nostrgo.Kind{nostrgo.KindReaction}, Authors: []nostrgo.PubKey{client.publicKey},
		Tags: nostrgo.TagMap{"e": {targetID.Hex()}}, Limit: maximumPageSize,
	}
	events, _, err := client.network.Query(callCtx, filter)
	if err != nil {
		return err
	}
	reactions := make([]nostrgo.Event, 0, len(events))
	for _, event := range matchingEvents(events, filter) {
		if event.Content == "" || event.Content == "+" {
			reactions = append(reactions, event)
		}
	}
	if len(reactions) == 0 {
		return nil
	}
	tags := make(nostrgo.Tags, 0, len(reactions)+1)
	for _, reaction := range reactions {
		tags = append(tags, nostrgo.Tag{"e", reaction.ID.Hex()})
	}
	tags = append(tags, nostrgo.Tag{"k", "7"})
	deletion := nostrgo.Event{Kind: nostrgo.KindDeletion, Tags: tags, Content: ""}
	_, err = client.publishEvent(callCtx, "remove_reaction", &deletion)
	return err
}

func (client *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	if strings.TrimSpace(input.Text) == "" {
		return nil, invalidArgument("comment", "comment text is required")
	}
	callCtx, cancel, err := client.callContext(ctx, "comment", options...)
	if err != nil {
		return nil, err
	}
	defer cancel()
	root, rootReport, err := client.getTextNote(callCtx, "comment", input.PostID)
	if err != nil {
		return nil, err
	}
	tags := client.replyTags(root, rootReport.Sources[root.ID.Hex()])
	if input.ParentID != nil {
		parent, parentReport, getErr := client.getTextNote(callCtx, "comment", *input.ParentID)
		if getErr != nil {
			return nil, getErr
		}
		parentRoot, hasRoot := threadRootReference(parent.Tags)
		if parent.ID != root.ID && (!hasRoot || parentRoot.ID != root.ID) {
			return nil, invalidArgument("comment", "parent comment does not belong to post_id")
		}
		if parent.ID != root.ID {
			rootReference := eventReference{
				ID: root.ID, Relay: relayHint(rootReport.Sources[root.ID.Hex()]), Author: root.PubKey,
			}
			parentReference := eventReference{
				ID: parent.ID, Relay: relayHint(parentReport.Sources[parent.ID.Hex()]), Author: parent.PubKey,
			}
			tags = nostrgo.Tags{referenceTag("e", rootReference, "root"), referenceTag("e", parentReference, "reply")}
			tags = appendPTag(tags, root.PubKey, rootReference.Relay)
			tags = appendPTag(tags, parent.PubKey, parentReference.Relay)
		}
	}
	event := nostrgo.Event{Kind: nostrgo.KindTextNote, Tags: tags, Content: input.Text}
	publish, err := client.publishEvent(callCtx, "comment", &event)
	if err != nil {
		return nil, err
	}
	report := reportForPublished(event, publish)
	comment := client.mapComment(event, root.ID.Hex(), report)
	putExtension(comment.Extensions, "nostr.publish", publish)
	return &comment, nil
}

func (client *Client) DeleteComment(ctx context.Context, identifier string, options ...socialhub.CallOption) error {
	return client.deleteTextNote(ctx, "delete_comment", identifier, options...)
}

func (client *Client) validateActor(identifier, operation string) error {
	if strings.TrimSpace(identifier) == "" {
		return nil
	}
	publicKey, err := parsePublicKey(identifier)
	if err != nil || publicKey != client.publicKey {
		return invalidArgument(operation, "actor must identify the configured Nostr public key")
	}
	return nil
}

func (client *Client) reactWithContent(ctx context.Context, input socialhub.ReactionRequest, content, operation string, options ...socialhub.CallOption) (nostrgo.Event, publishReport, error) {
	if err := client.validateActor(input.ActorID, operation); err != nil {
		return nostrgo.Event{}, publishReport{}, err
	}
	if content == "" {
		content = "+"
	}
	if len(content) > 64 || strings.ContainsAny(content, "\r\n\t ") {
		return nostrgo.Event{}, publishReport{}, invalidArgument(operation, "reaction content must be +, -, or an emoji token of at most 64 bytes without whitespace")
	}
	callCtx, cancel, err := client.callContext(ctx, operation, options...)
	if err != nil {
		return nostrgo.Event{}, publishReport{}, err
	}
	defer cancel()
	target, report, err := client.getTextNote(callCtx, operation, input.TargetID)
	if err != nil {
		return nostrgo.Event{}, publishReport{}, err
	}
	hint := relayHint(report.Sources[target.ID.Hex()])
	event := nostrgo.Event{
		Kind: nostrgo.KindReaction,
		Tags: nostrgo.Tags{
			relayTag("e", target.ID.Hex(), hint, "", ""),
			{"p", target.PubKey.Hex()},
			{"k", "1"},
		},
		Content: content,
	}
	publish, err := client.publishEvent(callCtx, operation, &event)
	return event, publish, err
}
