package micropub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tomnomnom/linkheader"

	"social-hub/pkg/socialhub"
)

func (client *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := input.Validate(); err != nil {
		return nil, invalidArgument("publish", "post requires text")
	}
	if err := client.requireScope("publish", "create"); err != nil {
		return nil, err
	}
	if len(input.MediaIDs) != 0 {
		return nil, unsupported("publish", "common Publish cannot distinguish Micropub photo, video, and audio URLs; use EntryWorkflow")
	}
	if input.QuotePostID != nil {
		return nil, unsupported("publish", "Micropub quote semantics are not standardized; use typed properties when supported by the endpoint")
	}
	if input.Visibility != nil && *input.Visibility != "" && !strings.EqualFold(*input.Visibility, "public") {
		return nil, unsupported("publish", "Micropub does not standardize non-public visibility")
	}
	if input.Text == nil || !validText(*input.Text, false) {
		return nil, invalidArgument("publish", "text must be non-empty, valid UTF-8, and bounded")
	}
	values := url.Values{"h": {"entry"}, "content": {*input.Text}}
	if input.ReplyToID != nil {
		if !validAbsoluteURL(*input.ReplyToID) {
			return nil, invalidArgument("publish", "reply_to_id must be an absolute HTTP(S) URL")
		}
		values.Set("in-reply-to", *input.ReplyToID)
	}
	request, cancel, err := client.endpointRequest(ctx, http.MethodPost, nil, strings.NewReader(values.Encode()), options...)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	result, err := client.do(request, cancel)
	if err != nil {
		return nil, err
	}
	created, err := entryResult("publish", result, "", true)
	if err != nil {
		return nil, err
	}
	text := *input.Text
	visibility := "public"
	status := socialhub.PublishStatus{ID: created.URL, State: created.State}
	post := &socialhub.Post{
		Platform: platformName, AccountID: client.accountID, ID: created.URL,
		Text: &text, URL: &created.URL, Visibility: &visibility, Status: &status,
	}
	if len(created.Shortlinks) != 0 || len(created.Syndication) != 0 {
		post.Extensions = make(map[string]json.RawMessage, 2)
		if len(created.Shortlinks) != 0 {
			post.Extensions["micropub.shortlinks"] = mustRaw(created.Shortlinks)
		}
		if len(created.Syndication) != 0 {
			post.Extensions["micropub.syndication"] = mustRaw(created.Syndication)
		}
	}
	if input.ReplyToID != nil {
		post.Relations = []socialhub.PostRelation{{Type: socialhub.RelationReply, PostID: *input.ReplyToID}}
	}
	return post, nil
}

func (client *Client) PublishStatus(ctx context.Context, postURL string, options ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	if !client.supportsUpdate {
		return nil, unsupported("publish_status", "endpoint is not configured with source-query support")
	}
	if _, err := client.Source(ctx, postURL, nil, options...); err != nil {
		return nil, err
	}
	now := client.clock.Now()
	return &socialhub.PublishStatus{ID: postURL, State: socialhub.PublishStatePublished, UpdatedAt: &now}, nil
}

func (client *Client) DeletePost(ctx context.Context, postURL string, options ...socialhub.CallOption) error {
	_, err := client.DeleteEntry(ctx, postURL, options...)
	return err
}

func (client *Client) CreateEntry(ctx context.Context, input EntryCreateRequest, options ...socialhub.CallOption) (*EntryResult, error) {
	if err := client.requireScope("create_entry", "create"); err != nil {
		return nil, err
	}
	payload, err := createEntryPayload(input)
	if err != nil {
		return nil, err
	}
	result, err := client.jsonRequest(ctx, http.MethodPost, payload, nil, options...)
	if err != nil {
		return nil, err
	}
	return entryResult("create_entry", result, "", true)
}

func (client *Client) UpdateEntry(ctx context.Context, input EntryUpdateRequest, options ...socialhub.CallOption) (*EntryResult, error) {
	if !client.supportsUpdate {
		return nil, unsupported("update_entry", "endpoint is not configured with update support")
	}
	if err := client.requireScope("update_entry", "update"); err != nil {
		return nil, err
	}
	payload, err := updateEntryPayload(input)
	if err != nil {
		return nil, err
	}
	result, err := client.jsonRequest(ctx, http.MethodPost, payload, nil, options...)
	if err != nil {
		return nil, err
	}
	return entryResult("update_entry", result, input.URL, false)
}

func (client *Client) DeleteEntry(ctx context.Context, postURL string, options ...socialhub.CallOption) (*EntryResult, error) {
	if !client.supportsDelete {
		return nil, unsupported("delete_entry", "endpoint is not configured with delete support")
	}
	if err := client.requireScope("delete_entry", "delete"); err != nil {
		return nil, err
	}
	return client.entryAction(ctx, "delete", postURL, options...)
}

func (client *Client) UndeleteEntry(ctx context.Context, postURL string, options ...socialhub.CallOption) (*EntryResult, error) {
	if !client.supportsUndelete {
		return nil, unsupported("undelete_entry", "endpoint is not configured with undelete support")
	}
	if err := client.requireScope("undelete_entry", "delete"); err != nil {
		return nil, err
	}
	return client.entryAction(ctx, "undelete", postURL, options...)
}

func (client *Client) entryAction(ctx context.Context, action, postURL string, options ...socialhub.CallOption) (*EntryResult, error) {
	operation := action + "_entry"
	if !validAbsoluteURL(postURL) {
		return nil, invalidArgument(operation, "entry URL must be an absolute HTTP(S) URL")
	}
	result, err := client.jsonRequest(ctx, http.MethodPost, actionPayload{Action: action, URL: postURL}, nil, options...)
	if err != nil {
		return nil, err
	}
	return entryResult(operation, result, postURL, false)
}

func createEntryPayload(input EntryCreateRequest) (createPayload, error) {
	if len(input.Types) == 0 {
		input.Types = []string{"h-entry"}
	}
	if len(input.Types) > 32 {
		return createPayload{}, invalidArgument("create_entry", "too many Microformats2 types")
	}
	for _, value := range input.Types {
		if !validMicroformatType(value) {
			return createPayload{}, invalidArgument("create_entry", "types must be bounded h-* Microformats2 names")
		}
	}
	if input.Content.Text != "" && input.Content.HTML != "" {
		return createPayload{}, invalidArgument("create_entry", "content must use either text or HTML, not both")
	}
	for _, value := range []string{input.Name, input.Summary, input.Content.Text, input.Content.HTML, input.Location} {
		if value != "" && !validText(value, true) {
			return createPayload{}, invalidArgument("create_entry", "text properties must be valid UTF-8 and bounded")
		}
	}
	if err := validateURLList("create_entry", input.InReplyTo); err != nil {
		return createPayload{}, err
	}
	if err := validateURLList("create_entry", input.LikeOf); err != nil {
		return createPayload{}, err
	}
	if err := validateURLList("create_entry", input.RepostOf); err != nil {
		return createPayload{}, err
	}
	if err := validateURLList("create_entry", input.Videos); err != nil {
		return createPayload{}, err
	}
	if err := validateURLList("create_entry", input.Audios); err != nil {
		return createPayload{}, err
	}
	for _, photo := range input.Photos {
		if !validAbsoluteURL(photo.Value) || !validText(photo.Alt, true) {
			return createPayload{}, invalidArgument("create_entry", "photos require an absolute URL and bounded alt text")
		}
	}
	for _, value := range append(append([]string(nil), input.Categories...), input.SyndicateTo...) {
		if !validText(value, false) {
			return createPayload{}, invalidArgument("create_entry", "category and syndication values must be non-empty and bounded")
		}
	}
	if err := validateRawProperties("create_entry", input.ExtraProperties, true); err != nil {
		return createPayload{}, err
	}
	properties := cloneProperties(input.ExtraProperties)
	addStringProperty(properties, "name", input.Name)
	addStringProperty(properties, "summary", input.Summary)
	if input.Content.Text != "" {
		properties["content"] = []json.RawMessage{rawString(input.Content.Text)}
	}
	if input.Content.HTML != "" {
		properties["content"] = []json.RawMessage{mustRaw(map[string]string{"html": input.Content.HTML})}
	}
	if input.Published != nil {
		properties["published"] = []json.RawMessage{rawString(input.Published.Format(time.RFC3339))}
	}
	addStrings(properties, "category", input.Categories)
	addStringProperty(properties, "location", input.Location)
	addStrings(properties, "in-reply-to", input.InReplyTo)
	addStrings(properties, "like-of", input.LikeOf)
	addStrings(properties, "repost-of", input.RepostOf)
	if len(input.Photos) != 0 {
		values := make([]json.RawMessage, 0, len(input.Photos))
		for _, photo := range input.Photos {
			if photo.Alt == "" {
				values = append(values, rawString(photo.Value))
			} else {
				values = append(values, mustRaw(photo))
			}
		}
		properties["photo"] = values
	}
	addStrings(properties, "video", input.Videos)
	addStrings(properties, "audio", input.Audios)
	addStrings(properties, "mp-syndicate-to", input.SyndicateTo)
	if len(properties) == 0 {
		return createPayload{}, invalidArgument("create_entry", "at least one entry property is required")
	}
	return createPayload{Type: append([]string(nil), input.Types...), Properties: properties}, nil
}

func updateEntryPayload(input EntryUpdateRequest) (updatePayload, error) {
	if !validAbsoluteURL(input.URL) {
		return updatePayload{}, invalidArgument("update_entry", "entry URL must be an absolute HTTP(S) URL")
	}
	if len(input.DeleteProperties) != 0 && len(input.DeleteValues) != 0 {
		return updatePayload{}, invalidArgument("update_entry", "delete_properties and delete_values cannot be sent together")
	}
	if err := validateRawProperties("update_entry", input.Replace, false); err != nil {
		return updatePayload{}, err
	}
	if err := validateRawProperties("update_entry", input.Add, false); err != nil {
		return updatePayload{}, err
	}
	if err := validateRawProperties("update_entry", input.DeleteValues, false); err != nil {
		return updatePayload{}, err
	}
	if len(input.Replace) == 0 && len(input.Add) == 0 && len(input.DeleteProperties) == 0 && len(input.DeleteValues) == 0 {
		return updatePayload{}, invalidArgument("update_entry", "at least one replace, add, or delete operation is required")
	}
	seen := make(map[string]struct{}, len(input.DeleteProperties))
	for _, name := range input.DeleteProperties {
		if !validPropertyName(name) {
			return updatePayload{}, invalidArgument("update_entry", "delete property name is invalid or reserved")
		}
		if _, exists := seen[name]; exists {
			return updatePayload{}, invalidArgument("update_entry", "delete property names must be unique")
		}
		seen[name] = struct{}{}
	}
	payload := updatePayload{Action: "update", URL: input.URL, Replace: cloneProperties(input.Replace), Add: cloneProperties(input.Add)}
	if len(input.DeleteProperties) != 0 {
		payload.Delete = append([]string(nil), input.DeleteProperties...)
	} else if len(input.DeleteValues) != 0 {
		payload.Delete = cloneProperties(input.DeleteValues)
	}
	return payload, nil
}

func entryResult(operation string, result response, currentURL string, create bool) (*EntryResult, error) {
	allowed := result.Status == http.StatusOK || result.Status == http.StatusCreated || result.Status == http.StatusNoContent
	if create {
		allowed = result.Status == http.StatusCreated || result.Status == http.StatusAccepted
	}
	if !allowed {
		return nil, &socialhub.Error{
			Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent, Platform: platformName, Product: productName,
			Op: operation, HTTPStatus: result.Status, PlatformMessage: "Micropub endpoint returned a non-conforming success status",
		}
	}
	location := strings.TrimSpace(result.Header.Get("Location"))
	if create || result.Status == http.StatusCreated {
		if !validAbsoluteURL(location) {
			return nil, &socialhub.Error{
				Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent, Platform: platformName, Product: productName,
				Op: operation, HTTPStatus: result.Status, PlatformMessage: "Micropub response requires an absolute Location URL",
			}
		}
		currentURL = location
	}
	state := socialhub.PublishStatePublished
	if result.Status == http.StatusAccepted {
		state = socialhub.PublishStatePending
	}
	entry := &EntryResult{URL: currentURL, State: state}
	for _, link := range linkheader.Parse(result.Header.Get("Link")) {
		if !validAbsoluteURL(link.URL) {
			continue
		}
		switch link.Rel {
		case "shortlink":
			entry.Shortlinks = append(entry.Shortlinks, link.URL)
		case "syndication":
			entry.Syndication = append(entry.Syndication, link.URL)
		}
	}
	return entry, nil
}

func validMicroformatType(value string) bool {
	if !strings.HasPrefix(value, "h-") || len(value) < 3 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character < 'a' || character > 'z' {
			if character < '0' || character > '9' {
				if character != '-' {
					return false
				}
			}
		}
	}
	return true
}

func cloneProperties(input map[string][]json.RawMessage) map[string][]json.RawMessage {
	output := make(map[string][]json.RawMessage, len(input)+16)
	for key, values := range input {
		cloned := make([]json.RawMessage, len(values))
		for index, value := range values {
			cloned[index] = append(json.RawMessage(nil), value...)
		}
		output[key] = cloned
	}
	return output
}

func addStringProperty(properties map[string][]json.RawMessage, name, value string) {
	if value != "" {
		properties[name] = []json.RawMessage{rawString(value)}
	}
}

func addStrings(properties map[string][]json.RawMessage, name string, values []string) {
	if len(values) == 0 {
		return
	}
	raw := make([]json.RawMessage, 0, len(values))
	for _, value := range values {
		raw = append(raw, rawString(value))
	}
	properties[name] = raw
}

func rawString(value string) json.RawMessage {
	encoded, _ := json.Marshal(value)
	return encoded
}

func mustRaw(value any) json.RawMessage {
	encoded, _ := json.Marshal(value)
	return encoded
}
