package kitsu

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetLibraryEntry(ctx context.Context, id string, options ...socialhub.CallOption) (*LibraryEntry, error) {
	if !validID(id) {
		return nil, invalidArgument("get_library_entry", "library entry ID is invalid")
	}
	var document resourceDocument
	query := url.Values{"include": {"anime,manga"}}
	if err := c.request(ctx, "get_library_entry", http.MethodGet, "library-entries/"+url.PathEscape(id), query, nil, &document, options...); err != nil {
		return nil, err
	}
	result, err := decodeLibraryEntry(document.Data, includedIndex(document.Included))
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ListLibraryEntries(ctx context.Context, input LibraryEntriesRequest, options ...socialhub.CallOption) (socialhub.Page[LibraryEntry], error) {
	userID := input.UserID
	if userID == "" {
		userID = c.userID
	}
	if userID != "" && !validID(userID) || input.MediaID != "" && !validID(input.MediaID) ||
		!validMediaKind(input.MediaKind) || !validLibraryStatus(input.Status) {
		return socialhub.Page[LibraryEntry]{}, invalidArgument("list_library_entries", "filter is invalid")
	}
	if userID == "" && input.MediaID == "" && input.Status == "" && input.Since == nil {
		return socialhub.Page[LibraryEntry]{}, invalidArgument("list_library_entries", "at least one Kitsu library filter is required")
	}
	offset, query, err := pagination(input.Cursor, input.Limit)
	if err != nil {
		return socialhub.Page[LibraryEntry]{}, err
	}
	if userID != "" {
		query.Set("filter[userId]", userID)
	}
	if input.MediaID != "" {
		query.Set("filter[mediaId]", input.MediaID)
	}
	if input.MediaKind != "" {
		query.Set("filter[mediaType]", string(input.MediaKind))
	}
	if input.Status != "" {
		query.Set("filter[status]", string(input.Status))
	}
	if input.Since != nil {
		query.Set("filter[since]", input.Since.UTC().Format(timeFormat))
	}
	query.Set("include", "anime,manga")
	var document collectionDocument
	if err := c.request(ctx, "list_library_entries", http.MethodGet, "library-entries", query, nil, &document, options...); err != nil {
		return socialhub.Page[LibraryEntry]{}, err
	}
	index := includedIndex(document.Included)
	items := make([]LibraryEntry, 0, len(document.Data))
	for _, item := range document.Data {
		decoded, err := decodeLibraryEntry(item, index)
		if err != nil {
			return socialhub.Page[LibraryEntry]{}, err
		}
		items = append(items, decoded)
	}
	limit := input.Limit
	if limit == 0 {
		limit = maxPageSize
	}
	return toPage(items, document.Links, offset, limit)
}

const timeFormat = "2006-01-02T15:04:05Z07:00"

func (c *Client) CreateLibraryEntry(ctx context.Context, input CreateLibraryEntryRequest, options ...socialhub.CallOption) (*LibraryEntry, error) {
	if err := c.requireUser("create_library_entry"); err != nil {
		return nil, err
	}
	if !validID(input.MediaID) || !validMediaKind(input.MediaKind) || input.MediaKind == "" ||
		!validLibraryStatus(input.Status) || input.Status == "" || !validCount(input.Progress) ||
		!validCount(input.VolumesOwned) || !validCount(input.ReconsumeCount) || !validRating(input.RatingTwenty) ||
		!validOptionalText(input.Notes) {
		return nil, invalidArgument("create_library_entry", "media, status, progress, rating, or notes is invalid")
	}
	attributes := libraryAttributes(input.Status, input.Progress, input.VolumesOwned, input.Reconsuming,
		input.ReconsumeCount, input.Notes, input.Private, input.RatingTwenty, input.StartedAt, input.FinishedAt)
	relations := map[string]relationship{
		"user":                                   identifierRelationship("users", c.userID),
		strings.ToLower(string(input.MediaKind)): identifierRelationship(strings.ToLower(string(input.MediaKind)), input.MediaID),
	}
	request := mutationDocument{Data: mutationResource{Type: "libraryEntries", Attributes: attributes, Relationships: relations}}
	var document resourceDocument
	if err := c.request(ctx, "create_library_entry", http.MethodPost, "library-entries", nil, request, &document, options...); err != nil {
		return nil, err
	}
	result, err := decodeLibraryEntry(document.Data, includedIndex(document.Included))
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) UpdateLibraryEntry(ctx context.Context, input UpdateLibraryEntryRequest, options ...socialhub.CallOption) (*LibraryEntry, error) {
	if err := c.requireUser("update_library_entry"); err != nil {
		return nil, err
	}
	if !validID(input.ID) || !validConcreteLibraryStatus(input.Status) || !validCount(input.Progress) ||
		!validCount(input.VolumesOwned) || !validCount(input.ReconsumeCount) || !validRating(input.RatingTwenty) ||
		!validOptionalText(input.Notes) {
		return nil, invalidArgument("update_library_entry", "ID, status, progress, rating, or notes is invalid")
	}
	attributes := libraryAttributesPointer(input.Status, input.Progress, input.VolumesOwned, input.Reconsuming,
		input.ReconsumeCount, input.Notes, input.Private, input.RatingTwenty, input.StartedAt, input.FinishedAt)
	if len(attributes) == 0 {
		return nil, invalidArgument("update_library_entry", "at least one field is required")
	}
	request := mutationDocument{Data: mutationResource{Type: "libraryEntries", ID: input.ID, Attributes: attributes}}
	var document resourceDocument
	if err := c.request(ctx, "update_library_entry", http.MethodPatch, "library-entries/"+url.PathEscape(input.ID), nil, request, &document, options...); err != nil {
		return nil, err
	}
	result, err := decodeLibraryEntry(document.Data, includedIndex(document.Included))
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteLibraryEntry(ctx context.Context, id string, options ...socialhub.CallOption) error {
	if err := c.requireUser("delete_library_entry"); err != nil {
		return err
	}
	if !validID(id) {
		return invalidArgument("delete_library_entry", "library entry ID is invalid")
	}
	return c.request(ctx, "delete_library_entry", http.MethodDelete, "library-entries/"+url.PathEscape(id), nil, nil, nil, options...)
}

func libraryAttributes(status LibraryStatus, progress, volumesOwned *int, reconsuming *bool, reconsumeCount *int, notes *string, private *bool, rating *int, startedAt, finishedAt *time.Time) map[string]any {
	result := libraryAttributesPointer(&status, progress, volumesOwned, reconsuming, reconsumeCount, notes, private, rating, startedAt, finishedAt)
	return result
}

func libraryAttributesPointer(status *LibraryStatus, progress, volumesOwned *int, reconsuming *bool, reconsumeCount *int, notes *string, private *bool, rating *int, startedAt, finishedAt *time.Time) map[string]any {
	result := make(map[string]any)
	putPointer(result, "status", status)
	putPointer(result, "progress", progress)
	putPointer(result, "volumesOwned", volumesOwned)
	putPointer(result, "reconsuming", reconsuming)
	putPointer(result, "reconsumeCount", reconsumeCount)
	putPointer(result, "notes", notes)
	putPointer(result, "private", private)
	putPointer(result, "ratingTwenty", rating)
	putPointer(result, "startedAt", startedAt)
	putPointer(result, "finishedAt", finishedAt)
	return result
}

func putPointer[T any](target map[string]any, key string, value *T) {
	if value != nil {
		target[key] = *value
	}
}

func decodeLibraryEntry(source resource, included map[string]resource) (LibraryEntry, error) {
	if source.Type != "libraryEntries" || !validID(source.ID) {
		return LibraryEntry{}, platformError("decode_library_entry", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	var result LibraryEntry
	if err := unmarshalAttributes(source, &result); err != nil {
		return LibraryEntry{}, err
	}
	result.ID, result.UserID = source.ID, relationshipID(source, "user")
	media := relationshipIdentifier(source, "media")
	if media.ID == "" {
		media = relationshipIdentifier(source, "anime")
	}
	if media.ID == "" {
		media = relationshipIdentifier(source, "manga")
	}
	result.MediaID = media.ID
	switch media.Type {
	case "anime":
		result.MediaKind = MediaAnime
	case "manga":
		result.MediaKind = MediaManga
	}
	if item, ok := included[media.Type+":"+media.ID]; ok {
		decoded, err := decodeMedia(item, result.MediaKind)
		if err != nil {
			return LibraryEntry{}, err
		}
		result.Media = &decoded
	}
	return result, nil
}
