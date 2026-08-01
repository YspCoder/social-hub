package dailymotion

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const videoFields = "video_id,title,description,category,visibility,is_for_kids,is_explicit,created_at,profile,video_url,updated_at,uploaded_at,published_at,language,country,engagement_message,hashtags,tags,is_published,processing,is_ai_altered,enable_ai_chapter_generation,source,thumbnail"

func (c *Client) GetVideo(ctx context.Context, videoID string, options ...socialhub.CallOption) (*Video, error) {
	if err := c.requireScopes("get_video", ScopeVideoRead); err != nil {
		return nil, err
	}
	if !validResourceID(videoID) {
		return nil, invalidArgument("get_video", "a valid video ID is required")
	}
	var response Video
	if err := c.requestJSON(ctx, http.MethodGet, "/videos/"+escapedID(videoID), url.Values{"fields": {videoFields}}, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.VideoID != videoID {
		return nil, platformError("get_video", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (c *Client) ListVideos(ctx context.Context, input VideoListRequest, options ...socialhub.CallOption) (socialhub.Page[Video], error) {
	if err := c.requireScopes("list_videos", ScopeVideoRead); err != nil {
		return socialhub.Page[Video]{}, err
	}
	profileID, err := c.defaultProfile("list_videos", input.ProfileID)
	if err != nil {
		return socialhub.Page[Video]{}, err
	}
	query, err := pageQuery("list_videos", input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[Video]{}, err
	}
	if !validSort(input.Sort, map[string]struct{}{"created_at": {}, "title": {}}) || input.Visibility != "" && !validVideoVisibility(input.Visibility) || input.CreatedAfter != nil && input.CreatedBefore != nil && input.CreatedAfter.After(*input.CreatedBefore) || !validTags(input.Tags) {
		return socialhub.Page[Video]{}, invalidArgument("list_videos", "sort, filters, time range, or tags are invalid")
	}
	query.Set("fields", videoFields)
	if input.Sort != "" {
		query.Set("sort", input.Sort)
	}
	if input.Visibility != "" {
		query.Set("visibility", input.Visibility)
	}
	if input.IsExplicit != nil {
		query.Set("is_explicit", boolString(*input.IsExplicit))
	}
	if input.IsForKids != nil {
		query.Set("is_for_kids", boolString(*input.IsForKids))
	}
	if input.CreatedAfter != nil {
		query.Set("created_after", input.CreatedAfter.UTC().Format(timeFormat))
	}
	if input.CreatedBefore != nil {
		query.Set("created_before", input.CreatedBefore.UTC().Format(timeFormat))
	}
	if len(input.Tags) > 0 {
		query.Set("tags", strings.Join(input.Tags, ","))
	}
	var response apiPage[Video]
	if err := c.requestJSON(ctx, http.MethodGet, "/profiles/"+escapedID(profileID)+"/videos", query, nil, &response, options...); err != nil {
		return socialhub.Page[Video]{}, err
	}
	return mapPage(c, response)
}

func (c *Client) CreateVideo(ctx context.Context, input CreateVideoRequest, options ...socialhub.CallOption) (*Video, error) {
	if err := c.requireScopes("create_video", ScopeVideoManage); err != nil {
		return nil, err
	}
	profileID, err := c.defaultProfile("create_video", input.ProfileID)
	if err != nil {
		return nil, err
	}
	if err := validateCreateVideo(input); err != nil {
		return nil, err
	}
	type nestedEmbedding struct {
		EnableEmbed bool `json:"enable_embed"`
	}
	body := struct {
		Title                     string           `json:"title"`
		Description               string           `json:"description,omitempty"`
		Category                  string           `json:"category"`
		Visibility                string           `json:"visibility"`
		IsForKids                 bool             `json:"is_for_kids"`
		IsExplicit                bool             `json:"is_explicit,omitempty"`
		Password                  string           `json:"password,omitempty"`
		PublishedAt               *time.Time       `json:"published_at,omitempty"`
		Language                  string           `json:"language,omitempty"`
		Country                   string           `json:"country,omitempty"`
		EngagementMessage         string           `json:"engagement_message,omitempty"`
		Hashtags                  []string         `json:"hashtags,omitempty"`
		Tags                      []string         `json:"tags,omitempty"`
		IsAIAltered               bool             `json:"is_ai_altered,omitempty"`
		EnableAIChapterGeneration bool             `json:"enable_ai_chapter_generation,omitempty"`
		Embedding                 *nestedEmbedding `json:"embedding,omitempty"`
		Source                    struct {
			FileURL string `json:"file_url"`
		} `json:"source"`
	}{
		Title: input.Title, Description: input.Description, Category: input.Category, Visibility: input.Visibility,
		IsForKids: input.IsForKids, IsExplicit: input.IsExplicit, Password: input.Password, PublishedAt: input.PublishedAt,
		Language: input.Language, Country: input.Country, EngagementMessage: input.EngagementMessage,
		Hashtags: input.Hashtags, Tags: input.Tags, IsAIAltered: input.IsAIAltered, EnableAIChapterGeneration: input.EnableAIChapterGeneration,
	}
	if input.EnableEmbed != nil {
		body.Embedding = &nestedEmbedding{EnableEmbed: *input.EnableEmbed}
	}
	body.Source.FileURL = input.SourceURL
	var response Video
	if err := c.requestJSON(ctx, http.MethodPost, "/profiles/"+escapedID(profileID)+"/videos", nil, body, &response, options...); err != nil {
		return nil, err
	}
	if !validResourceID(response.VideoID) {
		return nil, platformError("create_video", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (c *Client) UpdateVideo(ctx context.Context, videoID string, input UpdateVideoRequest, options ...socialhub.CallOption) error {
	if err := c.requireScopes("update_video", ScopeVideoManage); err != nil {
		return err
	}
	if !validResourceID(videoID) {
		return invalidArgument("update_video", "a valid video ID is required")
	}
	if err := validateUpdateVideo(input); err != nil {
		return err
	}
	type nestedEmbedding struct {
		EnableEmbed bool `json:"enable_embed"`
	}
	type nestedSource struct {
		FileURL string `json:"file_url"`
	}
	body := struct {
		Title                     *string          `json:"title,omitempty"`
		Description               *string          `json:"description,omitempty"`
		Category                  *string          `json:"category,omitempty"`
		Visibility                *string          `json:"visibility,omitempty"`
		IsForKids                 *bool            `json:"is_for_kids,omitempty"`
		IsExplicit                *bool            `json:"is_explicit,omitempty"`
		Password                  *string          `json:"password,omitempty"`
		PublishedAt               *time.Time       `json:"published_at,omitempty"`
		Language                  *string          `json:"language,omitempty"`
		Country                   *string          `json:"country,omitempty"`
		EngagementMessage         *string          `json:"engagement_message,omitempty"`
		Hashtags                  *[]string        `json:"hashtags,omitempty"`
		Tags                      *[]string        `json:"tags,omitempty"`
		IsAIAltered               *bool            `json:"is_ai_altered,omitempty"`
		EnableAIChapterGeneration *bool            `json:"enable_ai_chapter_generation,omitempty"`
		Embedding                 *nestedEmbedding `json:"embedding,omitempty"`
		Source                    *nestedSource    `json:"source,omitempty"`
	}{Title: input.Title, Description: input.Description, Category: input.Category, Visibility: input.Visibility, IsForKids: input.IsForKids, IsExplicit: input.IsExplicit, Password: input.Password, Language: input.Language, Country: input.Country, EngagementMessage: input.EngagementMessage, Hashtags: input.Hashtags, Tags: input.Tags, IsAIAltered: input.IsAIAltered, EnableAIChapterGeneration: input.EnableAIChapterGeneration}
	body.PublishedAt = input.PublishedAt
	if input.EnableEmbed != nil {
		body.Embedding = &nestedEmbedding{EnableEmbed: *input.EnableEmbed}
	}
	if input.SourceURL != nil {
		body.Source = &nestedSource{FileURL: *input.SourceURL}
	}
	return c.requestJSON(ctx, http.MethodPatch, "/videos/"+escapedID(videoID), nil, body, nil, options...)
}

func (c *Client) DeleteVideo(ctx context.Context, videoID string, options ...socialhub.CallOption) error {
	if err := c.requireScopes("delete_video", ScopeVideoManage); err != nil {
		return err
	}
	if !validResourceID(videoID) {
		return invalidArgument("delete_video", "a valid video ID is required")
	}
	return c.requestJSON(ctx, http.MethodDelete, "/videos/"+escapedID(videoID), nil, nil, nil, options...)
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	video, err := c.GetVideo(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	return c.mapVideo(*video)
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	videos, err := c.ListVideos(ctx, VideoListRequest{ProfileID: input.UserID, Cursor: input.Cursor, MaxResults: input.MaxResults, CreatedAfter: input.StartTime, CreatedBefore: input.EndTime}, options...)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(videos.Items))
	for _, video := range videos.Items {
		post, err := c.mapVideo(video)
		if err != nil {
			return socialhub.Page[socialhub.Post]{}, err
		}
		items = append(items, *post)
	}
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: videos.NextCursor, PrevCursor: videos.PrevCursor, HasMore: videos.HasMore}, nil
}

func (c *Client) ListComments(context.Context, socialhub.ListCommentsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	return socialhub.Page[socialhub.Comment]{}, unsupported("list_comments", "Dailymotion API v2 does not expose video comments")
}

const timeFormat = "2006-01-02T15:04:05Z07:00"

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

var _ VideoWorkflow = (*Client)(nil)
