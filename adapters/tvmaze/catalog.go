package tvmaze

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (c *Client) SearchShows(ctx context.Context, query string, options ...socialhub.CallOption) ([]ShowSearchResult, error) {
	if !validQuery(query) {
		return nil, invalidArgument("search_shows", "query must be a nonempty bounded value without surrounding whitespace")
	}
	values := url.Values{"q": {query}}
	var results []ShowSearchResult
	_, err := requestJSON(ctx, c.api, "search_shows", "/search/shows", values, &results, options...)
	return results, err
}

func (c *Client) GetShow(ctx context.Context, showID int64, options ...socialhub.CallOption) (*Show, error) {
	if showID <= 0 {
		return nil, invalidArgument("get_show", "show_id must be positive")
	}
	var show Show
	_, err := requestJSON(ctx, c.api, "get_show", "/shows/"+strconv.FormatInt(showID, 10), nil, &show, options...)
	return &show, err
}

func (c *Client) LookupShow(ctx context.Context, lookup LookupShowRequest, options ...socialhub.CallOption) (*Show, error) {
	query, err := lookupQuery(lookup)
	if err != nil {
		return nil, err
	}
	metadata, requestErr := requestJSON(ctx, c.api, "lookup_show", "/lookup/shows", query, nil, options...)
	if metadata.StatusCode != http.StatusMovedPermanently {
		if requestErr != nil {
			return nil, requestErr
		}
		return nil, invalidPlatformResponse("lookup_show", "lookup response did not contain the documented canonical redirect")
	}
	var redirectError *socialhub.Error
	if !errors.As(requestErr, &redirectError) || redirectError.Code != socialhub.CodePlatformError ||
		redirectError.Class != socialhub.ClassPermanent || redirectError.HTTPStatus != http.StatusMovedPermanently ||
		redirectError.PlatformCode != "" || redirectError.PlatformMessage != "" || redirectError.Cause != nil {
		if requestErr != nil {
			return nil, requestErr
		}
		return nil, invalidPlatformResponse("lookup_show", "lookup redirect did not match the documented response contract")
	}
	showID, err := c.showIDFromLocation(metadata.Header.Get("Location"))
	if err != nil {
		return nil, err
	}
	return c.GetShow(ctx, showID, options...)
}

func (c *Client) showIDFromLocation(location string) (int64, error) {
	parsed, err := url.Parse(location)
	if err != nil || !parsed.IsAbs() || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.RawPath != "" ||
		!strings.EqualFold(parsed.Scheme, c.baseURL.Scheme) || !strings.EqualFold(parsed.Host, c.baseURL.Host) {
		return 0, invalidPlatformResponse("lookup_show", "lookup returned an invalid canonical location")
	}
	prefix := strings.TrimRight(c.baseURL.Path, "/") + "/shows/"
	if !strings.HasPrefix(parsed.Path, prefix) {
		return 0, invalidPlatformResponse("lookup_show", "lookup returned a location outside the configured API path")
	}
	identifier := strings.TrimPrefix(parsed.Path, prefix)
	showID, err := strconv.ParseInt(identifier, 10, 64)
	if err != nil || showID <= 0 || strconv.FormatInt(showID, 10) != identifier {
		return 0, invalidPlatformResponse("lookup_show", "lookup returned a noncanonical show identifier")
	}
	return showID, nil
}

func (c *Client) ListEpisodes(ctx context.Context, showID int64, includeSpecials bool, options ...socialhub.CallOption) ([]Episode, error) {
	if showID <= 0 {
		return nil, invalidArgument("list_episodes", "show_id must be positive")
	}
	query := url.Values{}
	if includeSpecials {
		query.Set("specials", "1")
	}
	var episodes []Episode
	_, err := requestJSON(ctx, c.api, "list_episodes", "/shows/"+strconv.FormatInt(showID, 10)+"/episodes", query, &episodes, options...)
	return episodes, err
}

func (c *Client) GetEpisode(ctx context.Context, episodeID int64, options ...socialhub.CallOption) (*Episode, error) {
	if episodeID <= 0 {
		return nil, invalidArgument("get_episode", "episode_id must be positive")
	}
	var episode Episode
	_, err := requestJSON(ctx, c.api, "get_episode", "/episodes/"+strconv.FormatInt(episodeID, 10), nil, &episode, options...)
	return &episode, err
}

func (c *Client) GetEpisodeByNumber(ctx context.Context, showID int64, season, number int, options ...socialhub.CallOption) (*Episode, error) {
	if showID <= 0 || season <= 0 || number <= 0 {
		return nil, invalidArgument("get_episode_by_number", "show_id, season, and number must be positive")
	}
	query := url.Values{"season": {strconv.Itoa(season)}, "number": {strconv.Itoa(number)}}
	var episode Episode
	_, err := requestJSON(ctx, c.api, "get_episode_by_number", "/shows/"+strconv.FormatInt(showID, 10)+"/episodebynumber", query, &episode, options...)
	return &episode, err
}

func (c *Client) ListEpisodesByDate(ctx context.Context, showID int64, date time.Time, options ...socialhub.CallOption) ([]Episode, error) {
	if showID <= 0 || !validDate(date) {
		return nil, invalidArgument("list_episodes_by_date", "show_id must be positive and date must be set")
	}
	query := url.Values{"date": {date.Format(time.DateOnly)}}
	var episodes []Episode
	_, err := requestJSON(ctx, c.api, "list_episodes_by_date", "/shows/"+strconv.FormatInt(showID, 10)+"/episodesbydate", query, &episodes, options...)
	return episodes, err
}

func (c *Client) ListSeasons(ctx context.Context, showID int64, options ...socialhub.CallOption) ([]Season, error) {
	if showID <= 0 {
		return nil, invalidArgument("list_seasons", "show_id must be positive")
	}
	var seasons []Season
	_, err := requestJSON(ctx, c.api, "list_seasons", "/shows/"+strconv.FormatInt(showID, 10)+"/seasons", nil, &seasons, options...)
	return seasons, err
}

func (c *Client) ListSeasonEpisodes(ctx context.Context, seasonID int64, options ...socialhub.CallOption) ([]Episode, error) {
	if seasonID <= 0 {
		return nil, invalidArgument("list_season_episodes", "season_id must be positive")
	}
	var episodes []Episode
	_, err := requestJSON(ctx, c.api, "list_season_episodes", "/seasons/"+strconv.FormatInt(seasonID, 10)+"/episodes", nil, &episodes, options...)
	return episodes, err
}

func (c *Client) ListCast(ctx context.Context, showID int64, options ...socialhub.CallOption) ([]CastMember, error) {
	if showID <= 0 {
		return nil, invalidArgument("list_cast", "show_id must be positive")
	}
	var cast []CastMember
	_, err := requestJSON(ctx, c.api, "list_cast", "/shows/"+strconv.FormatInt(showID, 10)+"/cast", nil, &cast, options...)
	return cast, err
}

func (c *Client) ListCrew(ctx context.Context, showID int64, options ...socialhub.CallOption) ([]CrewMember, error) {
	if showID <= 0 {
		return nil, invalidArgument("list_crew", "show_id must be positive")
	}
	var crew []CrewMember
	_, err := requestJSON(ctx, c.api, "list_crew", "/shows/"+strconv.FormatInt(showID, 10)+"/crew", nil, &crew, options...)
	return crew, err
}
