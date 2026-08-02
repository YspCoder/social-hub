package anilist

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

const userFields = `
id name about avatar { large medium } bannerImage siteUrl isFollowing isFollower
isBlocked unreadNotificationCount donatorTier donatorBadge createdAt updatedAt`

const mediaFields = `
id idMal title { romaji english native userPreferred } type format status description
startDate { year month day } endDate { year month day } season seasonYear seasonInt
episodes duration chapters volumes countryOfOrigin isLicensed source
coverImage { extraLarge large medium color } bannerImage genres synonyms averageScore meanScore
popularity favourites trending siteUrl updatedAt
nextAiringEpisode { id airingAt timeUntilAiring episode }`

const mediaListFields = `
id userId mediaId status score progress progressVolumes repeat priority private notes
hiddenFromStatusLists customLists advancedScores startedAt { year month day }
completedAt { year month day } updatedAt createdAt media {` + mediaFields + `}`

const activityFields = `
__typename
... on TextActivity {
  ` + textActivityFields + `
}
... on ListActivity {
  ` + listActivityFields + `
}`

const textActivityFields = `
id userId type replyCount text siteUrl isLocked isSubscribed likeCount isLiked isPinned createdAt
user {` + userFields + `}`

const listActivityFields = `
id userId type replyCount status progress siteUrl isLocked isSubscribed likeCount isLiked isPinned createdAt
user {` + userFields + `} media {` + mediaFields + `}`

const activityReplyFields = `
id userId activityId text likeCount isLiked createdAt user {` + userFields + `}`

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphQLError  `json:"errors"`
}

type pageInfo struct {
	CurrentPage int  `json:"currentPage"`
	PerPage     int  `json:"perPage"`
	HasNextPage bool `json:"hasNextPage"`
}

func (c *Client) requestGraphQL(ctx context.Context, operation, query string, variables map[string]any, output any, options ...socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return err
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "field selection is fixed by the typed GraphQL operation")
	}
	var response graphQLResponse
	err = c.api.JSON(ctx, http.MethodPost, "/", nil, graphQLRequest{Query: query, Variables: variables}, &response, options...)
	if platformErr, ok := err.(*socialhub.Error); ok {
		platformErr.Op = operation
		platformErr.Cause = sanitizeTransportError(platformErr.Cause)
	}
	if err != nil {
		return err
	}
	if len(response.Errors) > 0 {
		return graphQLPlatformError(operation, http.StatusOK, nil, response.Errors[0])
	}
	if output == nil {
		return nil
	}
	if len(response.Data) == 0 || string(response.Data) == "null" {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if err := json.Unmarshal(response.Data, output); err != nil {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return nil
}

func pageVariables(cursor string, limit int) (int, map[string]any, error) {
	page, valid := validPage(cursor, limit)
	if !valid {
		return 0, nil, invalidArgument("pagination", "cursor or limit is invalid")
	}
	if limit == 0 {
		limit = maxPageSize
	}
	return page, map[string]any{"page": page, "perPage": limit}, nil
}

func toPage[T any](items []T, info pageInfo, requestedPage int) (socialhub.Page[T], error) {
	if info.CurrentPage != requestedPage || info.CurrentPage < 1 || info.CurrentPage > maxPageNumber ||
		info.PerPage < 0 || info.PerPage > maxPageSize {
		return socialhub.Page[T]{}, platformError("pagination", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	result := socialhub.Page[T]{Items: items, HasMore: info.HasNextPage}
	if info.HasNextPage {
		next := strconv.Itoa(info.CurrentPage + 1)
		result.NextCursor = &next
	}
	if info.CurrentPage > 1 {
		previous := strconv.Itoa(info.CurrentPage - 1)
		result.PrevCursor = &previous
	}
	return result, nil
}

func sanitizeTransportError(err error) error {
	for err != nil {
		var urlError *url.Error
		if !errors.As(err, &urlError) || urlError.Err == nil {
			return err
		}
		err = urlError.Err
	}
	return nil
}
