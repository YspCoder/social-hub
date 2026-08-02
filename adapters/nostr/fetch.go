package nostr

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	nostrgo "fiatjaf.com/nostr"

	"social-hub/pkg/socialhub"
)

const (
	defaultPageSize   = 20
	maximumPageSize   = 100
	maximumCursorSkip = 10000
)

type pageCursor struct {
	timestamp nostrgo.Timestamp
	skip      int
}

func (client *Client) GetUser(ctx context.Context, identifier string, options ...socialhub.CallOption) (*socialhub.User, error) {
	publicKey := client.publicKey
	var err error
	if strings.TrimSpace(identifier) != "" {
		publicKey, err = parsePublicKey(identifier)
		if err != nil {
			return nil, invalidArgument("get_user", "user ID must be hex, npub, or nprofile")
		}
	}
	callCtx, cancel, err := client.callContext(ctx, "get_user", options...)
	if err != nil {
		return nil, err
	}
	defer cancel()
	events, report, err := client.network.Query(callCtx, nostrgo.Filter{
		Kinds: []nostrgo.Kind{nostrgo.KindProfileMetadata}, Authors: []nostrgo.PubKey{publicKey}, Limit: 1,
	})
	if err != nil {
		return nil, err
	}
	events = matchingEvents(events, nostrgo.Filter{Kinds: []nostrgo.Kind{nostrgo.KindProfileMetadata}, Authors: []nostrgo.PubKey{publicKey}})
	if len(events) == 0 {
		return nil, platformError("get_user", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	sortEvents(events)
	return client.mapUser(events[0], report)
}

func (client *Client) GetPost(ctx context.Context, identifier string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	event, report, err := client.getTextNote(ctx, "get_post", identifier, options...)
	if err != nil {
		return nil, err
	}
	post := client.mapPost(event, report)
	return &post, nil
}

func (client *Client) getTextNote(ctx context.Context, operation, identifier string, options ...socialhub.CallOption) (nostrgo.Event, queryReport, error) {
	eventID, err := parseEventID(identifier)
	if err != nil {
		return nostrgo.Event{}, queryReport{}, invalidArgument(operation, "event ID must be hex, note, or nevent")
	}
	callCtx, cancel, err := client.callContext(ctx, operation, options...)
	if err != nil {
		return nostrgo.Event{}, queryReport{}, err
	}
	defer cancel()
	filter := nostrgo.Filter{IDs: []nostrgo.ID{eventID}, Kinds: []nostrgo.Kind{nostrgo.KindTextNote}, Limit: 1}
	events, report, err := client.network.Query(callCtx, filter)
	if err != nil {
		return nostrgo.Event{}, report, err
	}
	events = matchingEvents(events, filter)
	if len(events) == 0 {
		return nostrgo.Event{}, report, platformError(operation, socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	sortEvents(events)
	return events[0], report, nil
}

func (client *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	maximum, cursor, err := pageParameters(input.MaxResults, input.Cursor, "list_posts")
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	if input.StartTime != nil && input.EndTime != nil && input.StartTime.After(*input.EndTime) {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "start_time must not be after end_time")
	}
	publicKey := client.publicKey
	if strings.TrimSpace(input.UserID) != "" {
		publicKey, err = parsePublicKey(input.UserID)
		if err != nil {
			return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "user ID must be hex, npub, or nprofile")
		}
	}
	filter := nostrgo.Filter{
		Kinds: []nostrgo.Kind{nostrgo.KindTextNote}, Authors: []nostrgo.PubKey{publicKey},
		Limit: maximum + cursor.skip + 1,
	}
	if input.StartTime != nil {
		filter.Since = nostrgo.Timestamp(input.StartTime.Unix())
	}
	if input.EndTime != nil {
		filter.Until = nostrgo.Timestamp(input.EndTime.Unix())
	}
	if cursor.timestamp != 0 && (filter.Until == 0 || cursor.timestamp < filter.Until) {
		filter.Until = cursor.timestamp
	}
	return client.listPostPage(ctx, filter, maximum, cursor, options...)
}

func (client *Client) listPostPage(ctx context.Context, filter nostrgo.Filter, maximum int, cursor pageCursor, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	callCtx, cancel, err := client.callContext(ctx, "list_posts", options...)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	defer cancel()
	events, report, err := client.network.Query(callCtx, filter)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	sortEvents(events)
	events = applyCursor(matchingEvents(events, filter), cursor)
	result := socialhub.Page[socialhub.Post]{Items: make([]socialhub.Post, 0, min(len(events), maximum))}
	for _, event := range events[:min(len(events), maximum)] {
		result.Items = append(result.Items, client.mapPost(event, report))
	}
	setNextCursor(&result.NextCursor, &result.HasMore, events, maximum, cursor)
	return result, nil
}

func (client *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	postID, err := parseEventID(input.PostID)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "post ID must be hex, note, or nevent")
	}
	maximum, cursor, err := pageParameters(input.MaxResults, input.Cursor, "list_comments")
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	filter := nostrgo.Filter{
		Kinds: []nostrgo.Kind{nostrgo.KindTextNote}, Tags: nostrgo.TagMap{"e": {postID.Hex()}},
		Limit: maximum + cursor.skip + 1,
	}
	if cursor.timestamp != 0 {
		filter.Until = cursor.timestamp
	}
	callCtx, cancel, err := client.callContext(ctx, "list_comments", options...)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	defer cancel()
	events, report, err := client.network.Query(callCtx, filter)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	events = matchingEvents(events, filter)
	comments := events[:0]
	for _, event := range events {
		root := eventPointerRoot(event)
		if root != nil && root.ID == postID {
			comments = append(comments, event)
		}
	}
	events = comments
	sortEvents(events)
	events = applyCursor(events, cursor)
	result := socialhub.Page[socialhub.Comment]{Items: make([]socialhub.Comment, 0, min(len(events), maximum))}
	for _, event := range events[:min(len(events), maximum)] {
		result.Items = append(result.Items, client.mapComment(event, postID.Hex(), report))
	}
	setNextCursor(&result.NextCursor, &result.HasMore, events, maximum, cursor)
	return result, nil
}

func matchingEvents(events []nostrgo.Event, filter nostrgo.Filter) []nostrgo.Event {
	matched := events[:0]
	for _, event := range events {
		if filter.Matches(event) {
			matched = append(matched, event)
		}
	}
	return matched
}

func sortEvents(events []nostrgo.Event) {
	sort.Slice(events, func(left, right int) bool {
		if events[left].CreatedAt != events[right].CreatedAt {
			return events[left].CreatedAt > events[right].CreatedAt
		}
		return events[left].ID.Hex() < events[right].ID.Hex()
	})
}

func pageParameters(maximum int, encodedCursor, operation string) (int, pageCursor, error) {
	if maximum < 0 || maximum > maximumPageSize {
		return 0, pageCursor{}, invalidArgument(operation, "max_results must be between 0 and 100")
	}
	if maximum == 0 {
		maximum = defaultPageSize
	}
	if encodedCursor == "" {
		return maximum, pageCursor{}, nil
	}
	timestampValue, skipValue, found := strings.Cut(encodedCursor, ":")
	if !found || strings.Contains(skipValue, ":") {
		return 0, pageCursor{}, invalidArgument(operation, "cursor is invalid")
	}
	timestamp, timestampErr := strconv.ParseInt(timestampValue, 10, 64)
	skip, skipErr := strconv.Atoi(skipValue)
	if timestampErr != nil || skipErr != nil || timestamp <= 0 || skip <= 0 || skip > maximumCursorSkip {
		return 0, pageCursor{}, invalidArgument(operation, "cursor is invalid")
	}
	return maximum, pageCursor{timestamp: nostrgo.Timestamp(timestamp), skip: skip}, nil
}

func applyCursor(events []nostrgo.Event, cursor pageCursor) []nostrgo.Event {
	if cursor.timestamp == 0 || cursor.skip == 0 {
		return events
	}
	skipped := 0
	result := events[:0]
	for _, event := range events {
		if event.CreatedAt == cursor.timestamp && skipped < cursor.skip {
			skipped++
			continue
		}
		result = append(result, event)
	}
	return result
}

func setNextCursor(target **string, hasMore *bool, events []nostrgo.Event, maximum int, previous pageCursor) {
	if len(events) <= maximum || maximum == 0 {
		return
	}
	last := events[maximum-1]
	skip := 0
	for index := maximum - 1; index >= 0 && events[index].CreatedAt == last.CreatedAt; index-- {
		skip++
	}
	if previous.timestamp == last.CreatedAt {
		skip += previous.skip
	}
	encoded := fmt.Sprintf("%d:%d", last.CreatedAt, skip)
	*target, *hasMore = &encoded, true
}

func eventPointerRoot(event nostrgo.Event) *nostrgo.EventPointer {
	if root, found := threadRootReference(event.Tags); found {
		return &nostrgo.EventPointer{ID: root.ID, Relays: []string{root.Relay}, Author: root.Author}
	}
	if parent, found := immediateParentReference(event.Tags); found {
		return &nostrgo.EventPointer{ID: parent.ID, Relays: []string{parent.Relay}, Author: parent.Author}
	}
	return nil
}
