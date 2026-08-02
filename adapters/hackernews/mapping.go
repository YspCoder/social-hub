package hackernews

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (c *Client) mapUser(user *User) *socialhub.User {
	profileURL := "https://news.ycombinator.com/user?id=" + url.QueryEscape(user.ID)
	accountType := "hackernews_user"
	return &socialhub.User{
		Platform: "hackernews", AccountID: c.accountID, ID: user.ID,
		Username: stringPointer(user.ID), DisplayName: stringPointer(user.ID),
		ProfileURL: &profileURL, AccountType: &accountType,
		Extensions: extension("hackernews.user", user),
	}
}

func (c *Client) mapPost(item *Item) *socialhub.Post {
	id := strconv.FormatInt(item.ID, 10)
	visibility := "public"
	if item.Dead || item.Deleted {
		visibility = "removed"
	}
	postURL := item.URL
	if postURL == "" {
		postURL = "https://news.ycombinator.com/item?id=" + id
	}
	text := strings.TrimSpace(strings.Join(nonEmpty(item.Title, item.Text), "\n\n"))
	now := c.clock.Now()
	metrics := []socialhub.Metric{
		{Name: "score", Value: float64(item.Score), AsOf: now, Definition: "Hacker News points at retrieval time"},
		{Name: "comments", Value: float64(item.Descendants), AsOf: now, Definition: "Hacker News descendant count at retrieval time"},
	}
	return &socialhub.Post{
		Platform: "hackernews", AccountID: c.accountID, ID: id,
		AuthorID: stringPointer(item.By), Text: stringPointer(text), CreatedAt: unixTimePointer(item.Time),
		URL: &postURL, Visibility: &visibility, Metrics: metrics,
		Extensions: extension("hackernews.item", item),
	}
}

func (c *Client) mapComment(item *Item, postID string) *socialhub.Comment {
	return &socialhub.Comment{
		Platform: "hackernews", AccountID: c.accountID, ID: strconv.FormatInt(item.ID, 10), PostID: postID,
		AuthorID: stringPointer(item.By), Text: item.Text, CreatedAt: unixTimePointer(item.Time),
		Extensions: extension("hackernews.item", item),
	}
}

func extension(key string, value any) map[string]json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return map[string]json.RawMessage{key: encoded}
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}

func unixTimePointer(seconds int64) *time.Time {
	if seconds <= 0 {
		return nil
	}
	value := time.Unix(seconds, 0).UTC()
	return &value
}

func nonEmpty(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, value)
		}
	}
	return result
}
