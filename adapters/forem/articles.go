package forem

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) CreateArticle(ctx context.Context, input CreateArticleRequest, options ...socialhub.CallOption) (*Article, error) {
	attributes, err := createArticleAttributes(input)
	if err != nil {
		return nil, err
	}
	var response wireArticle
	if err := client.requestJSON(ctx, http.MethodPost, "/api/articles", nil, articleEnvelope{Article: attributes}, &response, options...); err != nil {
		return nil, err
	}
	if response.ID <= 0 {
		return nil, platformError("create_article", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	article := client.mapArticle(response)
	return &article, nil
}

func (client *Client) GetArticle(ctx context.Context, articleID string, options ...socialhub.CallOption) (*Article, error) {
	response, err := client.getArticle(ctx, articleID, options...)
	if err != nil {
		return nil, err
	}
	article := client.mapArticle(response)
	return &article, nil
}

func (client *Client) UpdateArticle(ctx context.Context, articleID string, input UpdateArticleRequest, options ...socialhub.CallOption) (*Article, error) {
	if !validID(articleID) {
		return nil, invalidArgument("update_article", "article ID must be a positive integer")
	}
	attributes, err := updateArticleAttributes(input)
	if err != nil {
		return nil, err
	}
	var response wireArticle
	if err := client.requestJSON(ctx, http.MethodPut, resourcePath("articles", articleID), nil, articleEnvelope{Article: attributes}, &response, options...); err != nil {
		return nil, err
	}
	if response.ID != mustID(articleID) {
		return nil, platformError("update_article", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	article := client.mapArticle(response)
	return &article, nil
}

func (client *Client) UnpublishArticle(ctx context.Context, articleID, note string, options ...socialhub.CallOption) error {
	if !validID(articleID) {
		return invalidArgument("unpublish_article", "article ID must be a positive integer")
	}
	if len(note) > 1024 || strings.ContainsRune(note, '\x00') {
		return invalidArgument("unpublish_article", "note must be at most 1024 bytes and contain no NUL")
	}
	query := url.Values{}
	if strings.TrimSpace(note) != "" {
		query.Set("note", note)
	}
	return client.requestJSON(ctx, http.MethodPut, resourcePath("articles", articleID)+"/unpublish", query, nil, nil, options...)
}

func (client *Client) ListMyArticles(ctx context.Context, state ArticleState, cursor string, maximum int, options ...socialhub.CallOption) (socialhub.Page[Article], error) {
	path := ""
	switch state {
	case ArticleStateAll:
		path = "/api/articles/me/all"
	case ArticleStatePublished:
		path = "/api/articles/me/published"
	case ArticleStateUnpublished:
		path = "/api/articles/me/unpublished"
	default:
		return socialhub.Page[Article]{}, invalidArgument("list_my_articles", "state must be all, published, or unpublished")
	}
	query, pageNumber, pageSize, err := pageQuery(cursor, maximum)
	if err != nil {
		return socialhub.Page[Article]{}, err
	}
	var response []wireArticle
	if err := client.requestJSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return socialhub.Page[Article]{}, err
	}
	result := socialhub.Page[Article]{Items: make([]Article, 0, len(response))}
	for _, value := range response {
		if value.ID <= 0 {
			return socialhub.Page[Article]{}, platformError("list_my_articles", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		result.Items = append(result.Items, client.mapArticle(value))
	}
	result.NextCursor, result.PrevCursor, result.HasMore = pageCursors(len(response), pageNumber, pageSize)
	return result, nil
}

func createArticleAttributes(input CreateArticleRequest) (articleAttributes, error) {
	if !validText(input.Title, 1024) || !validText(input.BodyMarkdown, 8<<20) {
		return articleAttributes{}, invalidArgument("create_article", "title and body_markdown are required")
	}
	tags, err := encodeTags(input.Tags, false)
	if err != nil {
		return articleAttributes{}, err
	}
	if err := validateOptionalMetadata(input.Series, input.MainImageURL, input.CanonicalURL, input.Description); err != nil {
		return articleAttributes{}, err
	}
	organizationID, err := optionalID(input.OrganizationID, "create_article")
	if err != nil {
		return articleAttributes{}, err
	}
	title, body, published := input.Title, input.BodyMarkdown, input.Published
	return articleAttributes{
		Title: &title, BodyMarkdown: &body, Published: &published,
		Series: optionalString(input.Series), MainImage: optionalString(input.MainImageURL),
		CanonicalURL: optionalString(input.CanonicalURL), Description: optionalString(input.Description),
		Tags: tags, OrganizationID: organizationID,
	}, nil
}

func updateArticleAttributes(input UpdateArticleRequest) (articleAttributes, error) {
	if input.Title == nil && input.BodyMarkdown == nil && input.Published == nil && input.Series == nil &&
		input.MainImageURL == nil && input.CanonicalURL == nil && input.Description == nil && input.Tags == nil && input.OrganizationID == nil {
		return articleAttributes{}, invalidArgument("update_article", "at least one article field is required")
	}
	if input.Title != nil && !validText(*input.Title, 1024) {
		return articleAttributes{}, invalidArgument("update_article", "title must be non-empty and at most 1024 bytes")
	}
	if input.BodyMarkdown != nil && !validText(*input.BodyMarkdown, 8<<20) {
		return articleAttributes{}, invalidArgument("update_article", "body_markdown must be non-empty and at most 8 MiB")
	}
	if err := validateOptionalMetadataPointers(input.Series, input.MainImageURL, input.CanonicalURL, input.Description); err != nil {
		return articleAttributes{}, err
	}
	var tags *string
	var err error
	if input.Tags != nil {
		tags, err = encodeTags(*input.Tags, true)
		if err != nil {
			return articleAttributes{}, err
		}
	}
	var organizationID *int64
	if input.OrganizationID != nil {
		organizationID, err = optionalID(*input.OrganizationID, "update_article")
		if err != nil || organizationID == nil {
			return articleAttributes{}, invalidArgument("update_article", "organization_id must be a positive integer")
		}
	}
	return articleAttributes{
		Title: input.Title, BodyMarkdown: input.BodyMarkdown, Published: input.Published,
		Series: input.Series, MainImage: input.MainImageURL, CanonicalURL: input.CanonicalURL,
		Description: input.Description, Tags: tags, OrganizationID: organizationID,
	}, nil
}

func encodeTags(values []string, allowEmpty bool) (*string, error) {
	values = cleanTags(values)
	if len(values) == 0 {
		if allowEmpty {
			empty := ""
			return &empty, nil
		}
		return nil, nil
	}
	if len(values) > 4 {
		return nil, invalidArgument("article_tags", "Forem articles accept at most four tags")
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if len(value) > 64 || strings.ContainsAny(value, ",\x00\r\n") {
			return nil, invalidArgument("article_tags", "tags must be comma-free, bounded strings")
		}
		if _, exists := seen[value]; exists {
			return nil, invalidArgument("article_tags", "tags must be unique")
		}
		seen[value] = struct{}{}
	}
	encoded := strings.Join(values, ",")
	return &encoded, nil
}

func cleanTags(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func validateOptionalMetadata(series, mainImageURL, canonicalURL, description string) error {
	return validateOptionalMetadataPointers(&series, &mainImageURL, &canonicalURL, &description)
}

func validateOptionalMetadataPointers(series, mainImageURL, canonicalURL, description *string) error {
	if series != nil && (len(*series) > 1024 || strings.ContainsRune(*series, '\x00')) {
		return invalidArgument("article_metadata", "series must be at most 1024 bytes and contain no NUL")
	}
	if description != nil && (len(*description) > 2048 || strings.ContainsRune(*description, '\x00')) {
		return invalidArgument("article_metadata", "description must be at most 2048 bytes and contain no NUL")
	}
	for _, value := range []*string{mainImageURL, canonicalURL} {
		if value != nil && *value != "" && !validHTTPURL(*value) {
			return invalidArgument("article_metadata", "image and canonical URLs must be absolute HTTP(S) URLs")
		}
	}
	return nil
}

func validHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}

func optionalID(value, operation string) (*int64, error) {
	if value == "" {
		return nil, nil
	}
	if !validID(value) {
		return nil, invalidArgument(operation, "organization_id must be a positive integer")
	}
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return &parsed, nil
}
