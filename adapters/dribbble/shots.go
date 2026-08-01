package dribbble

import (
	"context"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const maxShotBytes int64 = 8 << 20

func (client *Client) CreateShot(ctx context.Context, input CreateShotRequest, reader io.Reader, options ...socialhub.CallOption) (*PendingResource, error) {
	tags, err := validateCreateShot(input, reader)
	if err != nil {
		return nil, err
	}
	if err := client.requireScopes("create_shot", "upload"); err != nil {
		return nil, err
	}
	fields := map[string][]string{"title": {input.Title}}
	if input.Description != "" {
		fields["description"] = []string{input.Description}
	}
	if input.LowProfile {
		fields["low_profile"] = []string{"true"}
	}
	if input.ReboundSourceID != "" {
		fields["rebound_source_id"] = []string{input.ReboundSourceID}
	}
	if input.ScheduledFor != nil {
		fields["scheduled_for"] = []string{input.ScheduledFor.UTC().Format(time.RFC3339)}
	}
	if len(tags) > 0 {
		fields["tags[]"] = tags
	}
	if input.TeamID != "" {
		fields["team_id"] = []string{input.TeamID}
	}
	metadata, err := client.multipartUpload(ctx, "create_shot", "/shots", "image", input.Filename, input.MIME, input.Size, fields, reader, options...)
	if err != nil {
		return nil, err
	}
	if metadata.StatusCode != http.StatusAccepted {
		return nil, platformError("create_shot", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	location := metadata.Header.Get("Location")
	id, ok := client.shotIDFromLocation(location)
	if !ok {
		return nil, platformError("create_shot", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &PendingResource{ID: id, Location: location, State: socialhub.PublishStatePending}, nil
}

func (client *Client) UpdateShot(ctx context.Context, shotID string, input UpdateShotRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if !validID(shotID) {
		return nil, invalidArgument("update_shot", "Shot ID must be a positive integer")
	}
	payload, err := updateShotPayload(input)
	if err != nil {
		return nil, err
	}
	if err := client.requireScopes("update_shot", "upload"); err != nil {
		return nil, err
	}
	var response Shot
	if _, err := client.requestJSON(ctx, http.MethodPut, "/shots/"+shotID, nil, payload, &response, options...); err != nil {
		return nil, err
	}
	if response.ID <= 0 || response.ID != mustID(shotID) {
		return nil, platformError("update_shot", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapShot(client.accountID, response, client.clock.Now()), nil
}

func (client *Client) DeleteShot(ctx context.Context, shotID string, options ...socialhub.CallOption) error {
	if !validID(shotID) {
		return invalidArgument("delete_shot", "Shot ID must be a positive integer")
	}
	if err := client.requireScopes("delete_shot", "upload"); err != nil {
		return err
	}
	_, err := client.requestJSON(ctx, http.MethodDelete, "/shots/"+shotID, nil, nil, nil, options...)
	return err
}

type shotUpdatePayload struct {
	Title        *string   `json:"title,omitempty"`
	Description  *string   `json:"description,omitempty"`
	LowProfile   *bool     `json:"low_profile,omitempty"`
	ScheduledFor *string   `json:"scheduled_for,omitempty"`
	Tags         *[]string `json:"tags,omitempty"`
	TeamID       *string   `json:"team_id,omitempty"`
}

func updateShotPayload(input UpdateShotRequest) (shotUpdatePayload, error) {
	if input.Title == nil && input.Description == nil && input.LowProfile == nil && input.ScheduledFor == nil && input.Tags == nil && input.TeamID == nil {
		return shotUpdatePayload{}, invalidArgument("update_shot", "at least one mutable field is required")
	}
	if input.Title != nil && !validText(*input.Title, true, 1000) || input.Description != nil && !validText(*input.Description, false, 20000) {
		return shotUpdatePayload{}, invalidArgument("update_shot", "title or description is invalid")
	}
	tags, err := normalizeTags(input.Tags)
	if err != nil {
		return shotUpdatePayload{}, err
	}
	if input.TeamID != nil && *input.TeamID != "" && !validID(*input.TeamID) {
		return shotUpdatePayload{}, invalidArgument("update_shot", "team ID must be empty or a positive integer")
	}
	if input.ScheduledFor != nil && input.ScheduledFor.IsZero() {
		return shotUpdatePayload{}, invalidArgument("update_shot", "scheduled_for must be a valid timestamp")
	}
	payload := shotUpdatePayload{Title: input.Title, Description: input.Description, LowProfile: input.LowProfile, TeamID: input.TeamID}
	if input.ScheduledFor != nil {
		formatted := input.ScheduledFor.UTC().Format(time.RFC3339)
		payload.ScheduledFor = &formatted
	}
	if input.Tags != nil {
		payload.Tags = &tags
	}
	return payload, nil
}

func validateCreateShot(input CreateShotRequest, reader io.Reader) ([]string, error) {
	if reader == nil || !validFilename(input.Filename) || !validShotMIME(input.MIME) || input.Size <= 0 || input.Size > maxShotBytes ||
		!validText(input.Title, true, 1000) || !validText(input.Description, false, 20000) {
		return nil, invalidArgument("create_shot", "safe filename, GIF/JPEG/PNG MIME, exact size up to 8 MiB, title, and reader are required")
	}
	if input.ReboundSourceID != "" && !validID(input.ReboundSourceID) || input.TeamID != "" && !validID(input.TeamID) {
		return nil, invalidArgument("create_shot", "rebound source and team IDs must be positive integers")
	}
	if input.ScheduledFor != nil && input.ScheduledFor.IsZero() {
		return nil, invalidArgument("create_shot", "scheduled_for must be a valid timestamp")
	}
	return normalizeTags(input.Tags)
}

func normalizeTags(input []string) ([]string, error) {
	if len(input) > 12 {
		return nil, invalidArgument("tags", "Dribbble supports at most 12 tags")
	}
	output := make([]string, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, value := range input {
		tag := strings.TrimSpace(value)
		if tag == "" || utf8.RuneCountInString(tag) > 255 || strings.ContainsAny(tag, "\r\n") {
			return nil, invalidArgument("tags", "tags must be non-empty single-line values up to 255 characters")
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		output = append(output, tag)
	}
	return output, nil
}

func (client *Client) shotIDFromLocation(value string) (string, bool) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != client.baseURL.Scheme || parsed.Host != client.baseURL.Host || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false
	}
	prefix := strings.TrimRight(client.baseURL.Path, "/") + "/shots/"
	if !strings.HasPrefix(parsed.Path, prefix) {
		return "", false
	}
	id := strings.TrimPrefix(parsed.Path, prefix)
	return id, validID(id)
}

func validFilename(value string) bool {
	return value != "" && len(value) <= 255 && value == strings.TrimSpace(value) && !strings.ContainsAny(value, "/\\\r\n") && value != "." && value != ".."
}

func validShotMIME(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (mediaType == "image/gif" || mediaType == "image/jpeg" || mediaType == "image/png")
}

func validText(value string, required bool, maximum int) bool {
	if required && strings.TrimSpace(value) == "" || utf8.RuneCountInString(value) > maximum {
		return false
	}
	for _, character := range value {
		if character == 0 || character == '\r' {
			return false
		}
	}
	return true
}
