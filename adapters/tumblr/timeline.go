package tumblr

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) Dashboard(ctx context.Context, input PageRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	limit, err := pageLimit(input.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	offset, err := parseOffset(input.Cursor)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("dashboard", "cursor must be a non-negative offset")
	}
	user, err := c.requireUser("dashboard")
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	if err := c.requireScopes("dashboard", "basic"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query := url.Values{
		"limit": {strconv.Itoa(limit)}, "npf": {"true"}, "notes_info": {"true"},
	}
	if offset > 0 {
		query.Set("offset", strconv.Itoa(offset))
	}
	var response tumblrPostList
	if err := c.request(ctx, user, http.MethodGet, "/user/dashboard", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	return offsetPage(mapPosts(c.accountID, response.Posts, c.clock.Now()), offset, limit, len(response.Posts), len(response.Posts) == limit), nil
}

func (c *Client) Tagged(ctx context.Context, input TaggedRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	tag := strings.TrimSpace(input.Tag)
	if tag == "" {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("tagged", "tag is required")
	}
	limit, err := pageLimit(input.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	if input.Cursor != "" {
		before, err := strconv.ParseInt(input.Cursor, 10, 64)
		if err != nil || before <= 0 {
			return socialhub.Page[socialhub.Post]{}, invalidArgument("tagged", "cursor must be a positive Unix timestamp")
		}
	}
	query := url.Values{"tag": {tag}, "limit": {strconv.Itoa(limit)}}
	if input.Cursor != "" {
		query.Set("before", input.Cursor)
	}
	var response []tumblrPost
	if err := c.request(ctx, c.public, http.MethodGet, "/tagged", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	page := socialhub.Page[socialhub.Post]{Items: mapPosts(c.accountID, response, c.clock.Now())}
	if len(response) == limit {
		last := response[len(response)-1]
		before := last.Timestamp
		if last.FeaturedTimestamp > 0 {
			before = last.FeaturedTimestamp
		}
		if before > 0 {
			next := strconv.FormatInt(before, 10)
			page.NextCursor, page.HasMore = &next, true
		}
	}
	return page, nil
}

func (c *Client) Notes(ctx context.Context, input NotesRequest, options ...socialhub.CallOption) (NotesPage, error) {
	if !validPostID(input.PostID) {
		return NotesPage{}, invalidArgument("notes", "numeric post ID is required")
	}
	blog, err := c.selectedBlog(input.BlogIdentifier)
	if err != nil {
		return NotesPage{}, err
	}
	mode := input.Mode
	if mode == "" {
		mode = NotesAll
	}
	if !validNotesMode(mode) {
		return NotesPage{}, invalidArgument("notes", "notes mode is invalid")
	}
	if input.Cursor != "" {
		cursor, err := strconv.ParseFloat(input.Cursor, 64)
		if err != nil || cursor <= 0 || math.IsInf(cursor, 0) || math.IsNaN(cursor) {
			return NotesPage{}, invalidArgument("notes", "cursor must be a positive Unix timestamp")
		}
	}
	query := url.Values{"id": {input.PostID}, "mode": {string(mode)}}
	if input.Cursor != "" {
		query.Set("before_timestamp", input.Cursor)
	}
	var response tumblrNotesResponse
	if err := c.request(ctx, c.public, http.MethodGet, "/blog/"+url.PathEscape(blog)+"/notes", query, nil, &response, options...); err != nil {
		return NotesPage{}, err
	}
	return NotesPage{
		Items: mapNotes(response.Notes), Rollup: mapNotes(response.Rollup), NextCursor: linkCursor(response.Links, "before_timestamp"),
		TotalNotes: response.TotalNotes, TotalLikes: response.TotalLikes, TotalReblogs: response.TotalReblogs,
	}, nil
}

func offsetPage(items []socialhub.Post, offset, limit, consumed int, hasMore bool) socialhub.Page[socialhub.Post] {
	page := socialhub.Page[socialhub.Post]{Items: items, HasMore: hasMore}
	if hasMore {
		next := strconv.Itoa(offset + consumed)
		page.NextCursor = &next
	}
	if offset > 0 {
		previous := offset - limit
		if previous < 0 {
			previous = 0
		}
		value := strconv.Itoa(previous)
		page.PrevCursor = &value
	}
	return page
}

func validNotesMode(mode NotesMode) bool {
	switch mode {
	case NotesAll, NotesLikes, NotesConversation, NotesRollup, NotesReblogsWithTags:
		return true
	default:
		return false
	}
}

func linkCursor(links tumblrLinks, key string) *string {
	if links.Next == nil {
		return nil
	}
	raw := links.Next.QueryParams[key]
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		value = strings.TrimSpace(string(raw))
	}
	if value == "" {
		return nil
	}
	return &value
}
