package letterboxd

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (c *Client) ListLogEntries(ctx context.Context, input LogEntriesRequest, options ...socialhub.CallOption) (socialhub.Page[LogEntry], error) {
	if err := c.requireToken("list_log_entries"); err != nil {
		return socialhub.Page[LogEntry]{}, err
	}
	query, err := logEntriesQuery(input)
	if err != nil {
		return socialhub.Page[LogEntry]{}, err
	}
	var response pageEnvelope[LogEntry]
	if err := c.requestJSON(ctx, http.MethodGet, "/log-entries", query, nil, &response, options...); err != nil {
		return socialhub.Page[LogEntry]{}, err
	}
	return toPage(response.Items, response.Next), nil
}

func (c *Client) GetLogEntry(ctx context.Context, id string, options ...socialhub.CallOption) (*LogEntry, error) {
	if err := c.requireToken("get_log_entry"); err != nil {
		return nil, err
	}
	if !validIdentifier(id) {
		return nil, invalidArgument("get_log_entry", "log-entry ID is invalid")
	}
	var response LogEntry
	if err := c.requestJSON(ctx, http.MethodGet, "/log-entry/"+escaped(id), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) ListReviewComments(ctx context.Context, id string, input PageRequest, options ...socialhub.CallOption) (socialhub.Page[ReviewComment], error) {
	if err := c.requireToken("list_review_comments"); err != nil {
		return socialhub.Page[ReviewComment]{}, err
	}
	if !validIdentifier(id) || !validPage(input.Cursor, input.PerPage) {
		return socialhub.Page[ReviewComment]{}, invalidArgument("list_review_comments", "log-entry ID or pagination is invalid")
	}
	var response pageEnvelope[ReviewComment]
	if err := c.requestJSON(ctx, http.MethodGet, "/log-entry/"+escaped(id)+"/comments", pageQuery(input.Cursor, input.PerPage), nil, &response, options...); err != nil {
		return socialhub.Page[ReviewComment]{}, err
	}
	return toPage(response.Items, response.Next), nil
}

func (c *Client) CreateLogEntry(ctx context.Context, input LogEntryCreationRequest, options ...socialhub.CallOption) (*LogEntry, error) {
	if err := c.requireContentModify("create_log_entry"); err != nil {
		return nil, err
	}
	if !validLogEntryCreation(input) {
		return nil, invalidArgument("create_log_entry", "log-entry creation request is invalid")
	}
	var response LogEntry
	if err := c.requestJSON(ctx, http.MethodPost, "/log-entries", nil, input, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) UpdateLogEntry(ctx context.Context, id string, input LogEntryUpdateRequest, options ...socialhub.CallOption) (*LogEntryUpdateResponse, error) {
	if err := c.requireContentModify("update_log_entry"); err != nil {
		return nil, err
	}
	if !validIdentifier(id) || !validLogEntryUpdate(input) {
		return nil, invalidArgument("update_log_entry", "log-entry ID or update request is invalid")
	}
	var response LogEntryUpdateResponse
	if err := c.requestJSON(ctx, http.MethodPatch, "/log-entry/"+escaped(id), nil, input, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) DeleteLogEntry(ctx context.Context, id string, options ...socialhub.CallOption) error {
	if err := c.requireContentModify("delete_log_entry"); err != nil {
		return err
	}
	if !validIdentifier(id) {
		return invalidArgument("delete_log_entry", "log-entry ID is invalid")
	}
	return c.requestJSON(ctx, http.MethodDelete, "/log-entry/"+escaped(id), nil, nil, nil, options...)
}

func (c *Client) CreateReviewComment(ctx context.Context, id, comment string, options ...socialhub.CallOption) (*ReviewComment, error) {
	if err := c.requireContentModify("create_review_comment"); err != nil {
		return nil, err
	}
	if !validIdentifier(id) || !validText(comment) {
		return nil, invalidArgument("create_review_comment", "log-entry ID or comment is invalid")
	}
	var response ReviewComment
	if err := c.requestJSON(ctx, http.MethodPost, "/log-entry/"+escaped(id)+"/comments", nil, struct {
		Comment string `json:"comment"`
	}{Comment: comment}, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func logEntriesQuery(input LogEntriesRequest) (url.Values, error) {
	if !validPage(input.Cursor, input.PerPage) || !validOptionalIdentifier(input.FilmID) ||
		!validOptionalIdentifier(input.MemberID) || !validYear(input.Year) || input.Month < 0 || input.Month > 12 ||
		(input.Month != 0 && input.Year == 0) || !validOptionalRating(input.MinRating) ||
		!validOptionalRating(input.MaxRating) || input.MinRating > 0 && input.MaxRating > 0 && input.MinRating > input.MaxRating ||
		!validUniqueValues(input.Where, allowedLogWhere) || !validQueryValue(input.Sort, 128) {
		return nil, invalidArgument("list_log_entries", "log-entry filters or pagination are invalid")
	}
	query := pageQuery(input.Cursor, input.PerPage)
	setOptional(query, "film", input.FilmID)
	setOptional(query, "member", input.MemberID)
	setOptional(query, "sort", input.Sort)
	if input.Year != 0 {
		query.Set("year", strconv.Itoa(input.Year))
	}
	if input.Month != 0 {
		query.Set("month", strconv.Itoa(input.Month))
	}
	if input.MinRating != 0 {
		query.Set("minRating", strconv.FormatFloat(input.MinRating, 'f', 1, 64))
	}
	if input.MaxRating != 0 {
		query.Set("maxRating", strconv.FormatFloat(input.MaxRating, 'f', 1, 64))
	}
	for _, clause := range input.Where {
		query.Add("where", clause)
	}
	return query, nil
}

func validOptionalRating(value float64) bool { return value == 0 || validRating(value) }

func validLogEntryCreation(input LogEntryCreationRequest) bool {
	if !validIdentifier(input.FilmID) || input.DiaryDetails == nil && input.Review == nil ||
		input.Rating != nil && !validRating(*input.Rating) || !validTags(input.Tags) ||
		!validCommentPolicy(input.CommentPolicy) || !validPrivacyPolicy(input.PrivacyPolicy) {
		return false
	}
	if input.DiaryDetails != nil && !validDate(input.DiaryDetails.DiaryDate) {
		return false
	}
	return input.Review == nil || validText(input.Review.Text)
}

func validLogEntryUpdate(input LogEntryUpdateRequest) bool {
	if input.DiaryDetails == nil && input.Review == nil && input.Tags == nil && input.Rating == nil &&
		input.Like == nil && input.CommentPolicy == "" && input.PrivacyPolicy == "" {
		return false
	}
	if input.Rating != nil && !validRating(*input.Rating) || input.Tags != nil && !validTags(input.Tags) ||
		!validCommentPolicy(input.CommentPolicy) || !validPrivacyPolicy(input.PrivacyPolicy) {
		return false
	}
	if input.DiaryDetails != nil && !validDate(input.DiaryDetails.DiaryDate) {
		return false
	}
	return input.Review == nil || validText(input.Review.Text)
}
