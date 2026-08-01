package tumblr

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	blog, err := c.selectedBlog(userID)
	if err != nil {
		return nil, err
	}
	var response struct {
		Blog tumblrBlog `json:"blog"`
	}
	if err := c.request(ctx, c.public, http.MethodGet, "/blog/"+url.PathEscape(blog)+"/info", nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.Blog.Name == "" || (response.Blog.UUID == "" && response.Blog.URL == "") {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapBlog(c.accountID, response.Blog), nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if !validPostID(postID) {
		return nil, invalidArgument("get_post", "numeric post ID is required")
	}
	query := url.Values{"id": {postID}, "npf": {"true"}, "notes_info": {"true"}}
	var response tumblrPostList
	if err := c.request(ctx, c.public, http.MethodGet, "/blog/"+url.PathEscape(c.blogIdentifier)+"/posts", query, nil, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Posts) != 1 || response.Posts[0].identifier() != postID {
		return nil, platformError("get_post", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	return mapPost(c.accountID, response.Posts[0], c.clock.Now()), nil
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	limit, err := pageLimit(input.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	if input.Cursor != "" && (input.StartTime != nil || input.EndTime != nil) {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "Tumblr offset cursors cannot be combined with time filters")
	}
	offset, err := parseOffset(input.Cursor)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "cursor must be a non-negative offset")
	}
	blog, err := c.selectedBlog(input.UserID)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query := url.Values{
		"npf": {"true"}, "notes_info": {"true"}, "limit": {strconv.Itoa(limit)},
	}
	if offset > 0 {
		query.Set("offset", strconv.Itoa(offset))
	}
	if input.StartTime != nil {
		query.Set("after", strconv.FormatInt(input.StartTime.Unix(), 10))
		query.Set("sort", "asc")
	}
	if input.EndTime != nil {
		query.Set("before", strconv.FormatInt(input.EndTime.Unix(), 10))
	}
	var response tumblrPostList
	if err := c.request(ctx, c.public, http.MethodGet, "/blog/"+url.PathEscape(blog)+"/posts", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := mapPosts(c.accountID, response.Posts, c.clock.Now())
	page := socialhub.Page[socialhub.Post]{Items: items}
	if int64(offset+len(response.Posts)) < response.TotalPosts && len(response.Posts) > 0 {
		next := strconv.Itoa(offset + len(response.Posts))
		page.NextCursor, page.HasMore = &next, true
	}
	if offset > 0 {
		previous := offset - limit
		if previous < 0 {
			previous = 0
		}
		value := strconv.Itoa(previous)
		page.PrevCursor = &value
	}
	return page, nil
}

func (c *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if !validPostID(input.PostID) || input.MaxResults < 0 {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "numeric post ID and non-negative max results are required")
	}
	page, err := c.Notes(ctx, NotesRequest{
		BlogIdentifier: c.blogIdentifier, PostID: input.PostID, Mode: NotesConversation, Cursor: input.Cursor,
	}, options...)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	limit := len(page.Items)
	if input.MaxResults > 0 && input.MaxResults < limit {
		limit = input.MaxResults
	}
	comments := make([]socialhub.Comment, 0, limit)
	for _, note := range page.Items {
		text := firstNonEmpty(note.ReplyText, note.AddedText)
		if text == "" || (note.Type != "reply" && note.Type != "reblog") {
			continue
		}
		id := firstNonEmpty(note.ReplyID, note.PostID, noteID(input.PostID, note))
		comments = append(comments, socialhub.Comment{
			Platform: "tumblr", AccountID: c.accountID, ID: id, PostID: input.PostID,
			AuthorID: stringPointer(firstNonEmpty(note.BlogUUID, note.BlogName)), Text: text, CreatedAt: note.Timestamp,
			Extensions: map[string]json.RawMessage{"tumblr.note": append(json.RawMessage(nil), note.Raw...)},
		})
		if input.MaxResults > 0 && len(comments) == input.MaxResults {
			break
		}
	}
	return socialhub.Page[socialhub.Comment]{
		Items: comments, NextCursor: page.NextCursor, HasMore: page.NextCursor != nil,
	}, nil
}

func pageLimit(value int) (int, error) {
	if value < 0 || value > 20 {
		return 0, invalidArgument("pagination", "max results must be between 0 and 20")
	}
	if value == 0 {
		return 20, nil
	}
	return value, nil
}

func parseOffset(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(value)
	if err != nil || offset < 0 {
		return 0, invalidArgument("pagination", "offset cursor is invalid")
	}
	return offset, nil
}

func validPostID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func noteID(postID string, note Note) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte(postID))
	_, _ = digest.Write([]byte(note.Type))
	_, _ = digest.Write([]byte(note.BlogUUID))
	if note.Timestamp != nil {
		_, _ = digest.Write([]byte(note.Timestamp.Format("2006-01-02T15:04:05.999999999Z07:00")))
	}
	_, _ = digest.Write([]byte(firstNonEmpty(note.ReplyText, note.AddedText)))
	return hex.EncodeToString(digest.Sum(nil))
}
