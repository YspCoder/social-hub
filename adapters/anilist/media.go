package anilist

import (
	"context"

	"social-hub/pkg/socialhub"
)

const searchMediaQuery = `
query SearchMedia($page: Int!, $perPage: Int!, $search: String!, $type: MediaType!, $sort: [MediaSort], $isAdult: Boolean) {
  Page(page: $page, perPage: $perPage) {
    pageInfo { currentPage perPage hasNextPage }
    media(search: $search, type: $type, sort: $sort, isAdult: $isAdult) {` + mediaFields + `}
  }
}`

const getMediaQuery = `
query GetMedia($id: Int!) {
  Media(id: $id) {` + mediaFields + `}
}`

const listMediaQuery = `
query ListMedia($page: Int!, $perPage: Int!, $type: MediaType!, $sort: [MediaSort], $isAdult: Boolean) {
  Page(page: $page, perPage: $perPage) {
    pageInfo { currentPage perPage hasNextPage }
    media(type: $type, sort: $sort, isAdult: $isAdult) {` + mediaFields + `}
  }
}`

const seasonalMediaQuery = `
query SeasonalMedia($page: Int!, $perPage: Int!, $year: Int!, $season: MediaSeason!, $sort: [MediaSort], $isAdult: Boolean) {
  Page(page: $page, perPage: $perPage) {
    pageInfo { currentPage perPage hasNextPage }
    media(type: ANIME, seasonYear: $year, season: $season, sort: $sort, isAdult: $isAdult) {` + mediaFields + `}
  }
}`

func (c *Client) SearchMedia(ctx context.Context, input SearchMediaRequest, options ...socialhub.CallOption) (socialhub.Page[Media], error) {
	if !validSearch(input.Query) || !validMediaType(input.Type) || !validMediaSort(input.Sort) {
		return socialhub.Page[Media]{}, invalidArgument("search_media", "query, media type, or sort is invalid")
	}
	page, variables, err := pageVariables(input.Cursor, input.Limit)
	if err != nil {
		return socialhub.Page[Media]{}, err
	}
	sort := input.Sort
	if sort == "" {
		sort = MediaSortSearchMatch
	}
	variables["search"], variables["type"], variables["sort"] = input.Query, input.Type, []MediaSort{sort}
	if input.IsAdult != nil {
		variables["isAdult"] = *input.IsAdult
	}
	var response struct {
		Page *struct {
			PageInfo pageInfo `json:"pageInfo"`
			Media    []Media  `json:"media"`
		} `json:"Page"`
	}
	if err := c.requestGraphQL(ctx, "search_media", searchMediaQuery, variables, &response, options...); err != nil {
		return socialhub.Page[Media]{}, err
	}
	if response.Page == nil {
		return socialhub.Page[Media]{}, platformError("search_media", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return toPage(response.Page.Media, response.Page.PageInfo, page)
}

func (c *Client) GetMedia(ctx context.Context, mediaID int64, options ...socialhub.CallOption) (*Media, error) {
	if !validID(mediaID) {
		return nil, invalidArgument("get_media", "media ID must be a positive GraphQL Int")
	}
	var response struct {
		Media *Media `json:"Media"`
	}
	if err := c.requestGraphQL(ctx, "get_media", getMediaQuery, map[string]any{"id": mediaID}, &response, options...); err != nil {
		return nil, err
	}
	if response.Media == nil {
		return nil, platformError("get_media", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	return response.Media, nil
}

func (c *Client) ListTrendingMedia(ctx context.Context, input ListMediaRequest, options ...socialhub.CallOption) (socialhub.Page[Media], error) {
	if !validMediaType(input.Type) || (input.Sort != "" && input.Sort != MediaSortTrendingDesc) {
		return socialhub.Page[Media]{}, invalidArgument("trending_media", "media type or sort is invalid")
	}
	page, variables, err := pageVariables(input.Cursor, input.Limit)
	if err != nil {
		return socialhub.Page[Media]{}, err
	}
	variables["type"], variables["sort"] = input.Type, []MediaSort{MediaSortTrendingDesc}
	if input.IsAdult != nil {
		variables["isAdult"] = *input.IsAdult
	}
	var response struct {
		Page *struct {
			PageInfo pageInfo `json:"pageInfo"`
			Media    []Media  `json:"media"`
		} `json:"Page"`
	}
	if err := c.requestGraphQL(ctx, "trending_media", listMediaQuery, variables, &response, options...); err != nil {
		return socialhub.Page[Media]{}, err
	}
	if response.Page == nil {
		return socialhub.Page[Media]{}, platformError("trending_media", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return toPage(response.Page.Media, response.Page.PageInfo, page)
}

func (c *Client) ListSeasonalMedia(ctx context.Context, input SeasonalMediaRequest, options ...socialhub.CallOption) (socialhub.Page[Media], error) {
	if input.Year < 1940 || input.Year > 3000 || !validMediaSeason(input.Season) || !validMediaSort(input.Sort) || input.Sort == MediaSortSearchMatch {
		return socialhub.Page[Media]{}, invalidArgument("seasonal_media", "year, season, or sort is invalid")
	}
	page, variables, err := pageVariables(input.Cursor, input.Limit)
	if err != nil {
		return socialhub.Page[Media]{}, err
	}
	sort := input.Sort
	if sort == "" {
		sort = MediaSortPopularityDesc
	}
	variables["year"], variables["season"], variables["sort"] = input.Year, input.Season, []MediaSort{sort}
	if input.IsAdult != nil {
		variables["isAdult"] = *input.IsAdult
	}
	var response struct {
		Page *struct {
			PageInfo pageInfo `json:"pageInfo"`
			Media    []Media  `json:"media"`
		} `json:"Page"`
	}
	if err := c.requestGraphQL(ctx, "seasonal_media", seasonalMediaQuery, variables, &response, options...); err != nil {
		return socialhub.Page[Media]{}, err
	}
	if response.Page == nil {
		return socialhub.Page[Media]{}, platformError("seasonal_media", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return toPage(response.Page.Media, response.Page.PageInfo, page)
}
