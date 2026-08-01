package reddit

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

// SubmissionKind identifies a Reddit self or link submission.
type SubmissionKind string

const (
	SubmissionSelf SubmissionKind = "self"
	SubmissionLink SubmissionKind = "link"
)

// SubmissionRequest contains fields required by Reddit's /api/submit endpoint.
type SubmissionRequest struct {
	Kind        SubmissionKind
	Subreddit   string
	Title       string
	Text        string
	URL         string
	NSFW        bool
	Spoiler     bool
	SendReplies bool
	Resubmit    bool
	FlairID     string
	FlairText   string
}

// SubmissionWorkflow exposes Reddit's subreddit-aware submit and delete flow.
type SubmissionWorkflow interface {
	Submit(context.Context, SubmissionRequest, ...socialhub.CallOption) (*socialhub.Post, error)
	Delete(context.Context, string, ...socialhub.CallOption) error
}

// SubmissionService implements SubmissionWorkflow.
type SubmissionService struct{ client *Client }

func (s *SubmissionService) Submit(ctx context.Context, input SubmissionRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := validateSubmission(input); err != nil {
		return nil, err
	}
	if err := s.client.requireScopes("submit", "submit"); err != nil {
		return nil, err
	}
	values := url.Values{
		"api_type": {"json"}, "kind": {string(input.Kind)}, "sr": {input.Subreddit}, "title": {input.Title},
		"nsfw": {strconv.FormatBool(input.NSFW)}, "spoiler": {strconv.FormatBool(input.Spoiler)},
		"sendreplies": {strconv.FormatBool(input.SendReplies)}, "resubmit": {strconv.FormatBool(input.Resubmit)}, "raw_json": {"1"},
	}
	if input.Kind == SubmissionSelf {
		values.Set("text", input.Text)
	} else {
		values.Set("url", input.URL)
	}
	if input.FlairID != "" {
		values.Set("flair_id", input.FlairID)
	}
	if input.FlairText != "" {
		values.Set("flair_text", input.FlairText)
	}
	var response redditAPIResponse
	if err := s.client.form(ctx, "/api/submit", values, &response, options...); err != nil {
		return nil, err
	}
	if err := checkAPIResponse("submit", response); err != nil {
		return nil, err
	}
	if len(response.JSON.Data.Things) > 0 && response.JSON.Data.Things[0].Kind == "t3" {
		return mapPost(s.client.accountID, response.JSON.Data.Things[0].Data, s.client.clock.Now()), nil
	}
	postID := fullname(firstNonEmpty(response.JSON.Data.Name, response.JSON.Data.ID), "t3_")
	if !validFullname(postID, "t3_") {
		return nil, platformError("submit", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	text := input.Text
	if text == "" {
		text = input.Title
	}
	return &socialhub.Post{
		Platform: "reddit", AccountID: s.client.accountID, ID: postID, AuthorID: stringPointer(s.client.userID), Text: stringPointer(text),
		URL: stringPointer(response.JSON.Data.URL), Visibility: stringPointer("public"),
		Status: &socialhub.PublishStatus{ID: postID, State: socialhub.PublishStatePublished},
	}, nil
}

func validateSubmission(input SubmissionRequest) error {
	if !validCommunity(input.Subreddit) || strings.TrimSpace(input.Title) == "" || utf8.RuneCountInString(input.Title) > 300 {
		return invalidArgument("submit", "subreddit and a title up to 300 characters are required")
	}
	if utf8.RuneCountInString(input.FlairID) > 36 || utf8.RuneCountInString(input.FlairText) > 64 {
		return invalidArgument("submit", "flair ID or text exceeds Reddit limits")
	}
	switch input.Kind {
	case SubmissionSelf:
		if input.URL != "" {
			return invalidArgument("submit", "self submissions cannot include a link URL")
		}
	case SubmissionLink:
		if input.Text != "" || !validHTTPURL(input.URL) {
			return invalidArgument("submit", "link submissions require one absolute HTTP(S) URL and no self text")
		}
	default:
		return invalidArgument("submit", "kind must be self or link")
	}
	return nil
}

func (s *SubmissionService) Delete(ctx context.Context, postID string, options ...socialhub.CallOption) error {
	if !validFullname(postID, "t3_") {
		return invalidArgument("delete_post", "submission t3_ fullname is required")
	}
	if err := s.client.requireScopes("delete_post", "edit"); err != nil {
		return err
	}
	return s.client.form(ctx, "/api/del", url.Values{"id": {postID}}, nil, options...)
}

func validFullname(value, prefix string) bool {
	if !strings.HasPrefix(value, prefix) || len(value) <= len(prefix) || len(value) > 32 {
		return false
	}
	for _, r := range value[len(prefix):] {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func validThingFullname(value string) bool {
	return validFullname(value, "t1_") || validFullname(value, "t3_")
}

func validCommunity(value string) bool {
	if len(value) < 2 || len(value) > 21 {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

func validHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil
}

var _ SubmissionWorkflow = (*SubmissionService)(nil)
